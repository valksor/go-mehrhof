#!/usr/bin/env bun
// Stamp a release version into the frontend and desktop manifests so they match
// the Go binary's version (which is derived from the git tag at build time).
// This keeps the npm package, the Tauri bundle, and the Rust crate from drifting
// to a stale hardcoded value.
//
// Usage: bun scripts/sync-version.mjs <version>   (e.g. 0.11.0 or v0.11.0)
import { readFileSync, writeFileSync } from 'node:fs'

const version = (process.argv[2] ?? '').replace(/^v/, '')
// Anchored so trailing junk can't slip through; optional pre-release/build
// suffix is allowed (e.g. 1.0.0-rc.1).
if (!/^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/.test(version)) {
  console.error(`sync-version: invalid version ${JSON.stringify(process.argv[2])}`)
  process.exit(1)
}

// replaceFirst rewrites only the first match of regex, leaving the rest of the
// file (formatting, key order) untouched.
function replaceFirst(path, regex, replacement) {
  const src = readFileSync(path, 'utf8')
  if (!regex.test(src)) {
    console.error(`sync-version: pattern not found in ${path}`)
    process.exit(1)
  }
  writeFileSync(path, src.replace(regex, replacement))
  console.log(`  ${path} -> ${version}`)
}

console.log(`Syncing version ${version}:`)
// The package's own "version" is the first version key in each JSON file;
// dependency versions are keyed by package name, not "version".
replaceFirst('web/package.json', /("version"\s*:\s*)"[^"]*"/, `$1"${version}"`)
replaceFirst('web/src-tauri/tauri.conf.json', /("version"\s*:\s*)"[^"]*"/, `$1"${version}"`)
// In Cargo.toml, anchor on [package] and stop at the next section header so the
// match can never cross into [dependencies] and clobber a dep's version.
replaceFirst('web/src-tauri/Cargo.toml', /(\[package\][^[]*?\nversion\s*=\s*)"[^"]*"/, `$1"${version}"`)
// In Cargo.lock, update the kvelmo-desktop package entry (version follows name)
// so the lockfile stays consistent with Cargo.toml without running cargo.
replaceFirst('web/src-tauri/Cargo.lock', /(name = "kvelmo-desktop"\nversion = )"[^"]*"/, `$1"${version}"`)
