import { mkdtempSync, mkdirSync, writeFileSync, copyFileSync, chmodSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

// Tests for the npm shim (npm/index.js): platform-package mapping, the
// local-binary fallback, and the missing-binary error path.

const require = createRequire(import.meta.url);
const npmDir = new URL("../npm/", import.meta.url);
const { candidatePaths, resolveBinary } = require(fileURLToPath(new URL("index.js", npmDir)));

// The six supported platform/arch combinations and their package manifests.
const targets = [
  ["darwin", "arm64"],
  ["darwin", "x64"],
  ["linux", "arm64"],
  ["linux", "x64"],
  ["win32", "arm64"],
  ["win32", "x64"],
];

// A copy of the shim in a temp dir: require.resolve there cannot find any
// installed platform package, and its bin/ fallback is empty, so results are
// deterministic on every machine.
function shimCopy() {
  const dir = mkdtempSync(path.join(tmpdir(), "astrogui-shim-"));
  copyFileSync(new URL("index.js", npmDir), path.join(dir, "index.js"));
  return dir;
}

function runShim(dir, args = [], env = {}) {
  try {
    const stdout = execFileSync(process.execPath, [path.join(dir, "index.js"), ...args], {
      env: { ...process.env, ...env, ASTROGUI_BIN: env.ASTROGUI_BIN },
      encoding: "utf8",
    });
    return { status: 0, stdout, stderr: "" };
  } catch (err) {
    return { status: err.status, stdout: err.stdout, stderr: err.stderr };
  }
}

test("every supported platform maps to its scoped platform package", () => {
  for (const [platform, arch] of targets) {
    const pkg = `@amagyar/astrogui-${platform}-${arch}`;
    const exe = platform === "win32" ? ".exe" : "";
    const candidates = candidatePaths(platform, arch, "/shim-home", {});
    assert.ok(
      candidates.includes(`${pkg}/bin/astrogui${exe}`),
      `${platform}-${arch} must resolve the ${pkg} package binary`,
    );
  }
});

test("only the six supported combinations have package manifests", () => {
  const manifestNames = targets.map(([platform, arch]) => {
    const manifest = JSON.parse(readFileSync(new URL(`packages/${platform}-${arch}/package.json`, npmDir), "utf8"));
    assert.deepEqual(manifest.os, [platform], `${platform}-${arch} declares os`);
    assert.deepEqual(manifest.cpu, [arch], `${platform}-${arch} declares cpu`);
    assert.equal(manifest.name, `@amagyar/astrogui-${platform}-${arch}`);
    return manifest.name;
  });
  assert.equal(new Set(manifestNames).size, 6, "manifest names are unique");
});

test("launcher manifest keeps the astrogui command and pins the six platform packages", () => {
  const launcher = JSON.parse(readFileSync(new URL("package.json", npmDir), "utf8"));
  assert.equal(launcher.name, "@amagyar/astrogui");
  assert.deepEqual(launcher.bin, { astrogui: "index.js" }, "executable stays astrogui");
  assert.equal(launcher.scripts, undefined, "no install/postinstall scripts");
  const pinned = Object.keys(launcher.optionalDependencies);
  assert.deepEqual([...pinned].sort(), targets.map(([p, a]) => `@amagyar/astrogui-${p}-${a}`).sort());
  for (const [platform, arch] of targets) {
    const platformPkg = JSON.parse(readFileSync(new URL(`packages/${platform}-${arch}/package.json`, npmDir), "utf8"));
    assert.equal(
      launcher.optionalDependencies[platformPkg.name],
      platformPkg.version,
      "launcher pins each platform package at its own version",
    );
  }
});

test("ASTROGUI_BIN is the first candidate when set", () => {
  assert.deepEqual(candidatePaths("darwin", "arm64", "/shim-home", { ASTROGUI_BIN: "/custom/astrogui" })[0], "/custom/astrogui");
});

test("resolveBinary falls back to a binary built next to the shim", () => {
  const dir = shimCopy();
  mkdirSync(path.join(dir, "bin"));
  const bin = path.join(dir, "bin", "astrogui");
  writeFileSync(bin, "#!/bin/sh\n");
  chmodSync(bin, 0o755);
  assert.equal(resolveBinary(process.platform, process.arch, dir, {}), bin);
});

test("resolveBinary honors the exe suffix on win32", () => {
  const dir = shimCopy();
  mkdirSync(path.join(dir, "bin"));
  const exe = path.join(dir, "bin", "astrogui.exe");
  writeFileSync(exe, "");
  assert.equal(resolveBinary("win32", "x64", dir, {}), exe);
});

test("resolveBinary prefers an existing ASTROGUI_BIN over the local fallback", () => {
  const dir = shimCopy();
  mkdirSync(path.join(dir, "bin"));
  writeFileSync(path.join(dir, "bin", "astrogui"), "#!/bin/sh\n");
  const custom = path.join(dir, "custom-astrogui");
  writeFileSync(custom, "#!/bin/sh\n");
  assert.equal(resolveBinary(process.platform, process.arch, dir, { ASTROGUI_BIN: custom }), custom);
});

test("missing binary reports the platform clearly and exits nonzero", () => {
  const dir = shimCopy();
  const { status, stderr } = runShim(dir);
  assert.notEqual(status, 0, "exits nonzero");
  assert.match(stderr, /no prebuilt binary for /);
  assert.match(stderr, new RegExp(`@amagyar/astrogui-${process.platform}-${process.arch}`));
  assert.match(stderr, /ASTROGUI_BIN/);
});

test("a nonexistent ASTROGUI_BIN is skipped, not fatal by itself", () => {
  const dir = shimCopy();
  const { status, stderr } = runShim(dir, [], { ASTROGUI_BIN: path.join(dir, "does-not-exist") });
  assert.notEqual(status, 0);
  assert.match(stderr, /no prebuilt binary for /);
});

test("the shim execs the resolved binary with the arguments unchanged", { skip: process.platform === "win32" }, () => {
  const dir = shimCopy();
  const echo = path.join(dir, "fake-astrogui");
  writeFileSync(echo, "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\"; done\n");
  chmodSync(echo, 0o755);
  const { status, stdout } = runShim(dir, ["version"], { ASTROGUI_BIN: echo });
  assert.equal(status, 0);
  assert.equal(stdout.trim(), "version");
});
