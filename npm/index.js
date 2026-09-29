#!/usr/bin/env node
'use strict';

// astrogui npm shim: resolves the prebuilt platform binary and execs it with
// the arguments unchanged. npm has no native-binary concept, so the package
// ships this small JavaScript entry plus per-platform optional dependencies
// (@astrogui/<platform>-<arch>) that each carry the binary for one target.

const { spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const platform = process.platform; // darwin | linux | win32
const arch = process.arch;         // arm64 | x64 | ...
const exe = platform === 'win32' ? '.exe' : '';

function candidatePaths() {
  const out = [];
  if (process.env.ASTROGUI_BIN) out.push(process.env.ASTROGUI_BIN);
  // Installed per-platform package.
  out.push(`@astrogui/${platform}-${arch}/bin/astrogui${exe}`);
  // A binary built next to this package (development and CI fallback).
  out.push(path.join(__dirname, 'bin', `astrogui${exe}`));
  return out;
}

function resolveBinary() {
  for (const candidate of candidatePaths()) {
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

const bin = resolveBinary();
if (!bin) {
  console.error(`astrogui: no prebuilt binary for ${platform}-${arch}.`);
  console.error('Install with the matching @astrogui/<platform>-<arch> package,');
  console.error('or point ASTROGUI_BIN at a local binary.');
  process.exit(1);
}

const result = spawnSync(bin, process.argv.slice(2), { stdio: 'inherit' });
if (result.error) {
  console.error(`astrogui: failed to run ${bin}: ${result.error.message}`);
  process.exit(1);
}
process.exit(result.status === null ? 1 : result.status);
