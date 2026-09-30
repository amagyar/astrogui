#!/usr/bin/env node
'use strict';

// astrogui npm shim: resolves the prebuilt platform binary and execs it with
// the arguments unchanged. npm has no native-binary concept, so the package
// ships this small JavaScript entry plus per-platform optional dependencies
// (@amagyar/astrogui-<platform>-<arch>) that each carry the binary for one
// target.

const { spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const exeSuffix = (platform) => (platform === 'win32' ? '.exe' : '');

function candidatePaths(platform, arch, from, env) {
  const out = [];
  if (env.ASTROGUI_BIN) out.push(env.ASTROGUI_BIN);
  // Installed per-platform package.
  out.push(`@amagyar/astrogui-${platform}-${arch}/bin/astrogui${exeSuffix(platform)}`);
  // A binary built next to this package (development and CI fallback).
  out.push(path.join(from, 'bin', `astrogui${exeSuffix(platform)}`));
  return out;
}

function resolveBinary(platform, arch, from, env = process.env) {
  for (const candidate of candidatePaths(platform, arch, from, env)) {
    if (path.isAbsolute(candidate)) {
      if (fs.existsSync(candidate)) return candidate;
      continue;
    }
    try {
      const resolved = require.resolve(candidate);
      if (fs.existsSync(resolved)) return resolved;
    } catch {
      // not installed; try the next candidate
    }
  }
  return null;
}

function run() {
  const bin = resolveBinary(process.platform, process.arch, __dirname);
  if (!bin) {
    console.error(`astrogui: no prebuilt binary for ${process.platform}-${process.arch}.`);
    console.error(`Install with the matching @amagyar/astrogui-${process.platform}-${process.arch} package,`);
    console.error('or point ASTROGUI_BIN at a local binary.');
    process.exit(1);
  }

  const result = spawnSync(bin, process.argv.slice(2), { stdio: 'inherit' });
  if (result.error) {
    console.error(`astrogui: failed to run ${bin}: ${result.error.message}`);
    process.exit(1);
  }
  process.exit(result.status === null ? 1 : result.status);
}

if (require.main === module) run();

// Exposed for tests/shim.test.mjs; the shim execs only when run directly.
module.exports = { candidatePaths, resolveBinary };
