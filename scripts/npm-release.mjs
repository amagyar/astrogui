#!/usr/bin/env node
// Release packaging and publishing for the astrogui npm package family.
//
//   node scripts/npm-release.mjs stage   --version <v> --dist <release-archives-dir> [--out <tgz-dir>]
//   node scripts/npm-release.mjs publish --version <v> --tarballs <tgz-dir>
//
// stage derives the version for all seven package manifests from the release
// tag (v<version>), extracts the matching prebuilt binary from the GoReleaser
// archives into each platform package (no postinstall script), packs every
// package with `npm pack`, and validates the tarballs: contents, executable
// bit, platform metadata, and the repository URL npm provenance verification
// requires. It records what it packed in <out>/manifest.json.
//
// publish sends the six platform packages first, then the @amagyar/astrogui
// launcher, so the launcher never references unpublished versions. A rerun
// skips only packages already published at the identical content (same
// tarball shasum); a version published with different content is an error,
// never an overwrite. Any npm rejection fails the run and reports exactly
// which packages made it out.
//
// Publishing authenticates through npm trusted publishing (GitHub OIDC) — no
// long-lived npm token is read or accepted here.

import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const npmDir = path.join(repoRoot, "npm");

const LAUNCHER = "@amagyar/astrogui";

// npm provenance verification (mandatory under OIDC trusted publishing) rejects
// a publish whose manifest repository.url does not match the workflow's repo.
const REPOSITORY_URL = "https://github.com/amagyar/astrogui";
const normalizeRepositoryUrl = (url) => (url || "").replace(/^git\+/, "").replace(/\.git$/, "");

// npm platform/arch names and the GoReleaser goos/goarch they pair with.
const TARGETS = [
  { platform: "darwin", arch: "arm64", goos: "darwin", goarch: "arm64" },
  { platform: "darwin", arch: "x64", goos: "darwin", goarch: "amd64" },
  { platform: "linux", arch: "arm64", goos: "linux", goarch: "arm64" },
  { platform: "linux", arch: "x64", goos: "linux", goarch: "amd64" },
  { platform: "win32", arch: "arm64", goos: "windows", goarch: "arm64" },
  { platform: "win32", arch: "x64", goos: "windows", goarch: "amd64" },
];

const platformPkg = (t) => `${LAUNCHER}-${t.platform}-${t.arch}`;
const platformDir = (t) => path.join(npmDir, "packages", `${t.platform}-${t.arch}`);
const binaryName = (t) => (t.platform === "win32" ? "astrogui.exe" : "astrogui");
const archiveName = (t, version) => `astrogui_${version}_${t.goos}_${t.goarch}.tar.gz`;
const sha1 = (file) => createHash("sha1").update(fs.readFileSync(file)).digest("hex");
const sha256 = (file) => createHash("sha256").update(fs.readFileSync(file)).digest("hex");

function die(message) {
  console.error(`npm-release: ${message}`);
  process.exit(1);
}

function run(cmd, args, opts = {}) {
  return execFileSync(cmd, args, { encoding: "utf8", stdio: ["ignore", "pipe", "inherit"], ...opts });
}

const readJSON = (p) => JSON.parse(fs.readFileSync(p, "utf8"));
const writeJSON = (p, value) => fs.writeFileSync(p, `${JSON.stringify(value, null, 2)}\n`);

function parseArgs(argv) {
  const args = { _: [] };
  for (let i = 0; i < argv.length; i++) {
    if (!argv[i].startsWith("--")) {
      args._.push(argv[i]);
      continue;
    }
    const key = argv[i].slice(2);
    const value = argv[i + 1];
    if (value === undefined || value.startsWith("--")) die(`missing value for --${key}`);
    args[key] = value;
    i++;
  }
  return args;
}

function requireVersion(raw) {
  const version = (raw || "").replace(/^v/, "");
  if (!/^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/.test(version)) {
    die(`version must be the plain semver from the release tag (e.g. v1.2.3), got: ${raw}`);
  }
  return version;
}

function verifyArchives(distDir, version) {
  for (const t of TARGETS) {
    const archive = path.join(distDir, archiveName(t, version));
    if (!fs.existsSync(archive)) die(`missing release archive ${archive}`);
  }
  const checksums = path.join(distDir, "checksums.txt");
  if (!fs.existsSync(checksums)) return; // optional locally; release downloads always include it
  for (const line of fs.readFileSync(checksums, "utf8").split("\n")) {
    const match = line.match(/^([0-9a-f]{64})\s+\*?(.+)$/);
    if (!match) continue;
    const [, expected, file] = match;
    const archive = path.join(distDir, file);
    if (!fs.existsSync(archive)) continue;
    const got = sha256(archive);
    if (got !== expected) die(`checksum mismatch for ${file}: expected ${expected}, got ${got}`);
  }
  console.log(`- release archives match ${path.basename(checksums)}`);
}

function stampVersions(version) {
  for (const t of TARGETS) {
    const manifestPath = path.join(platformDir(t), "package.json");
    const manifest = readJSON(manifestPath);
    manifest.version = version;
    writeJSON(manifestPath, manifest);
  }
  const launcherPath = path.join(npmDir, "package.json");
  const launcher = readJSON(launcherPath);
  if (launcher.scripts) die("launcher manifest must not declare lifecycle scripts (no postinstall download)");
  launcher.version = version;
  for (const name of Object.keys(launcher.optionalDependencies || {})) {
    launcher.optionalDependencies[name] = version;
  }
  const expected = TARGETS.map(platformPkg).sort();
  if (Object.keys(launcher.optionalDependencies).sort().join() !== expected.join()) {
    die(`launcher optionalDependencies must be exactly ${expected.join(", ")}`);
  }
  writeJSON(launcherPath, launcher);
  console.log(`- stamped all seven package manifests at ${version}`);
}

function placeLicense() {
  const license = path.join(repoRoot, "LICENSE");
  if (!fs.existsSync(license)) die("missing LICENSE at repository root");
  // Copied from the single root LICENSE at stage time so every npm tarball
  // ships the license (OSPS-LE-03.02) without a second tracked copy.
  for (const dir of [...TARGETS.map(platformDir), npmDir]) {
    fs.copyFileSync(license, path.join(dir, "LICENSE"));
  }
  console.log("- copied LICENSE into all seven package directories");
}

function placeBinaries(distDir, version) {
  for (const t of TARGETS) {
    const binDir = path.join(platformDir(t), "bin");
    fs.rmSync(binDir, { recursive: true, force: true });
    fs.mkdirSync(binDir, { recursive: true });
    const name = binaryName(t);
    run("tar", ["-xzf", path.join(distDir, archiveName(t, version)), "-C", binDir, name]);
    fs.chmodSync(path.join(binDir, name), 0o755); // keep the executable bit through npm pack
    console.log(`- ${platformPkg(t)}: staged ${name} from ${archiveName(t, version)}`);
  }
}

function packAll(outDir) {
  fs.rmSync(outDir, { recursive: true, force: true });
  fs.mkdirSync(outDir, { recursive: true });
  const packed = [];
  for (const t of [...TARGETS, null]) {
    const dir = t ? platformDir(t) : npmDir;
    const stdout = run("npm", ["pack", "--pack-destination", outDir], { cwd: dir });
    const tgz = path.join(outDir, stdout.trim().split("\n").pop());
    if (!fs.existsSync(tgz)) die(`npm pack produced no tarball for ${dir}`);
    packed.push({ target: t, dir, tgz });
  }
  return packed;
}

function verifyTarballs(packed, version) {
  const packages = [];
  for (const { target: t, dir, tgz } of packed) {
    const name = t ? platformPkg(t) : LAUNCHER;
    const entries = run("tar", ["-tzf", tgz]).trim().split("\n").sort();
    const expected = ["package/package.json", "package/LICENSE", t ? `package/bin/${binaryName(t)}` : "package/index.js"];
    for (const entry of expected) {
      if (!entries.includes(entry)) die(`${name} tarball is missing ${entry} (has: ${entries.join(", ")})`);
    }

    if (t) {
      // The binary must arrive executable: npm preserves the packed file mode.
      const listing = run("tar", ["-tvzf", tgz, `package/bin/${binaryName(t)}`]);
      if (!/^-rwx/.test(listing.trim())) die(`${name} tarball binary is not executable: ${listing.trim()}`);
    }

    const meta = JSON.parse(run("tar", ["-xzOf", tgz, "package/package.json"]));
    if (meta.name !== name) die(`${tgz} contains package ${meta.name}, expected ${name}`);
    if (meta.version !== version) die(`${tgz} contains version ${meta.version}, expected ${version}`);
    if (meta.scripts) die(`${name} tarball declares lifecycle scripts; there must be no postinstall`);
    if (normalizeRepositoryUrl(meta.repository?.url) !== REPOSITORY_URL) {
      die(
        `${name} tarball repository.url is ${JSON.stringify(meta.repository?.url ?? "")}, ` +
          `expected to normalize to ${REPOSITORY_URL}; ` +
          "npm rejects the publish at provenance verification otherwise",
      );
    }
    if (t) {
      if (JSON.stringify(meta.os) !== JSON.stringify([t.platform]) || JSON.stringify(meta.cpu) !== JSON.stringify([t.arch])) {
        die(`${name} tarball declares os/cpu ${JSON.stringify(meta.os)}/${JSON.stringify(meta.cpu)}`);
      }
    } else {
      for (const dep of Object.values(meta.optionalDependencies || {})) {
        if (dep !== version) die(`launcher optional dependency ${dep} does not match release version ${version}`);
      }
    }
    packages.push({ name, dir, tgz: path.basename(tgz), sha1: sha1(tgz), sha256: sha256(tgz) });
    console.log(`- verified ${name}-${version}: ${path.basename(tgz)}`);
  }
  return packages;
}

function npmViewShasum(name, version) {
  try {
    const out = run("npm", ["view", `${name}@${version}`, "dist.shasum", "--json"]);
    const parsed = JSON.parse(out || "null");
    // npm >= 12 returns an array for this query even when the version spec
    // matches exactly; npm 11 returns a scalar. Normalize both.
    const value = Array.isArray(parsed) ? parsed[0] : parsed;
    return typeof value === "string" && value ? value : null;
  } catch {
    return null; // not published (E404) or registry unreachable; publish decides below
  }
}

function publish(version, tarballsDir) {
  const manifestPath = path.join(tarballsDir, "manifest.json");
  if (!fs.existsSync(manifestPath)) die(`no ${manifestPath}; run stage first`);
  const { version: staged, packages } = readJSON(manifestPath);
  if (staged !== version) die(`manifest was staged for ${staged}, not ${version}`);
  const byName = new Map(packages.map((p) => [p.name, p]));
  for (const t of TARGETS) {
    if (!byName.has(platformPkg(t))) die(`manifest is missing platform package ${platformPkg(t)}`);
  }
  if (!byName.has(LAUNCHER)) die(`manifest is missing the launcher ${LAUNCHER}`);

  // Platform packages first; the launcher goes out only once everything it
  // references exists on the registry.
  const order = [...TARGETS.map(platformPkg), LAUNCHER];
  const done = [];
  const pending = order.slice();
  for (const name of order) {
    const pkg = byName.get(name);
    const tgz = path.join(tarballsDir, pkg.tgz);
    if (!fs.existsSync(tgz)) die(`missing packed tarball ${tgz}; run stage again`);

    const published = npmViewShasum(name, version);
    if (published) {
      if (published === pkg.sha1) {
        console.log(`- ${name}@${version}: already published with identical content, skipping`);
        done.push(name);
        pending.splice(pending.indexOf(name), 1);
        continue;
      }
      die(
        `${name}@${version} is already published with different content (registry ${JSON.stringify(published)}, ours ${JSON.stringify(pkg.sha1)}); ` +
          "npm versions are immutable — publish a corrected release version instead",
      );
    }

    console.log(`- publishing ${name}@${version} …`);
    // A prerelease version must not become `latest`: npm requires an explicit
    // dist-tag for it, so prereleases go out under the `prerelease` tag.
    const args = ["publish", "--access", "public"];
    if (/-[0-9A-Za-z.-]+$/.test(version)) args.push("--tag", "prerelease");
    try {
      execFileSync("npm", args, { cwd: pkg.dir, stdio: "inherit" });
    } catch {
      die(
        `npm rejected publication of ${name}@${version}.` +
          (done.length ? ` Published this run: ${done.join(", ")}.` : "") +
          (pending.filter((n) => n !== name).length ? ` Not yet published: ${pending.filter((n) => n !== name).join(", ")}.` : "") +
          " Fix the cause and rerun; already-published packages at identical content are skipped.",
      );
    }
    done.push(name);
    pending.splice(pending.indexOf(name), 1);
  }
  console.log(`npm release complete: ${done.join(", ")} at ${version}`);
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const command = args._[0];
  const version = requireVersion(args.version);

  if (command === "stage") {
    // Absolute paths: npm pack resolves --pack-destination relative to each
    // package's own directory, not to the caller's cwd.
    const distDir = path.resolve(args.dist || die("stage requires --dist <release-archives-dir>"));
    const outDir = path.resolve(args.out || path.join(distDir, "npm"));
    verifyArchives(distDir, version);
    stampVersions(version);
    placeLicense();
    placeBinaries(distDir, version);
    const packed = packAll(outDir);
    const packages = verifyTarballs(packed, version);
    writeJSON(path.join(outDir, "manifest.json"), { version, packages });
    console.log(`staged ${packages.length} packages at ${version} in ${outDir}`);
  } else if (command === "publish") {
    const tarballsDir = path.resolve(args.tarballs || die("publish requires --tarballs <tgz-dir from stage>"));
    publish(version, tarballsDir);
  } else {
    die(`unknown command ${JSON.stringify(command)}; use stage or publish`);
  }
}

main();
