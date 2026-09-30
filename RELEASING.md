# Releasing astrogui

Releases are tag-driven. Pushing a `v<version>` tag runs
[`.github/workflows/release.yml`](.github/workflows/release.yml), which tests,
builds six targets, publishes the GitHub release, updates the Homebrew Cask,
and publishes the npm package family — all at the tag's version.

## One-time setup (before the first release tag)

### 1. npm trusted publishing — all seven packages

The npm job authenticates with GitHub OIDC (no npm token). For each of the
seven package names below, configure a trusted publisher on
[npmjs.com](https://www.npmjs.com) → package **Settings → Trusted Publisher →
GitHub Actions** with:

| Field                | Value        |
| -------------------- | ------------ |
| Organization or user | `amagyar`    |
| Repository           | `astrogui`   |
| Workflow filename    | `release.yml`|
| Environment name     | *(empty — the npm job uses no GitHub environment)* |
| Allowed actions      | direct `npm publish` (not stage-only) |

Packages:

- `@amagyar/astrogui` (the launcher)
- `@amagyar/astrogui-darwin-arm64`
- `@amagyar/astrogui-darwin-x64`
- `@amagyar/astrogui-linux-arm64`
- `@amagyar/astrogui-linux-x64`
- `@amagyar/astrogui-win32-arm64`
- `@amagyar/astrogui-win32-x64`

These names must stay in sync with `npm/package.json`,
`npm/packages/*/package.json`, and `scripts/npm-release.mjs`.

Notes:

- None of the packages exist on npm yet. The settings page expects an
  existing package; to pre-register a trusted publisher for a name that has
  never been published, bootstrap it with
  `npx --yes setup-npm-trusted-publish <package-name>` (see
  [npm/cli#8544](https://github.com/npm/cli/issues/8544)), or publish the
  first version once with a short-lived granular token and configure the
  trusted publisher immediately after. Every later release uses OIDC only.
- npm requires the workflow's `id-token: write` permission, npm CLI ≥ 11.5.1
  and Node ≥ 22.14 — the workflow pins `npm@12.0.1` on Node 26.
- npm checks that `repository.url` in each manifest matches the repository;
  all seven manifests declare `git+https://github.com/amagyar/astrogui.git`,
  and staging fails if any packed tarball's manifest is missing it or
  mismatches (the `422` provenance error only surfaces at publish time).
- Workflow filename must be exactly `release.yml` (filename only, no path).
  Trusted publishing does not validate the configuration when you save it —
  mistakes surface only at publish time.

### 2. Homebrew tap write credential

GoReleaser commits the generated `Casks/astrogui.rb` directly to
[`amagyar/homebrew-tap`](https://github.com/amagyar/homebrew-tap):

1. Create a fine-grained personal access token with repository access to
   **only** `amagyar/homebrew-tap` and the single permission
   **Contents: Read and write**. No other repositories, no other scopes.
2. Add it as the repository secret `HOMEBREW_TAP_GITHUB_TOKEN` in
   `amagyar/astrogui` (Settings → Secrets and variables → Actions).

The release workflow checks for the secret before doing any release work
(the `preflight` job) and exposes it only to the GoReleaser job that writes
the tap — never to the npm job.

## What a release does

1. `test` — `go test ./...` must pass.
2. `preflight` — the tap secret must be present.
3. `goreleaser` — builds the six darwin/linux/windows × amd64/arm64 binaries
   with `main.version` stamped from the tag, archives them (`tar.gz`), writes
   SHA-256 `checksums.txt`, signs every artifact keyless with cosign
   (`.sigstore.json` bundles), attests build provenance, publishes the GitHub
   release, and commits the updated Cask to the tap.
4. `npm` — downloads the exact release archives, verifies the checksums,
   stages and validates all seven package tarballs
   (`scripts/npm-release.mjs stage`), then publishes the six platform
   packages before the launcher (`scripts/npm-release.mjs publish`), all via
   OIDC.

## Release rules

- The tag is the single version source: GitHub release, binaries, all seven
  npm packages, and the Cask all use it (without the leading `v`).
- Never move or reuse a published tag, and never try to overwrite a published
  npm version — npm versions are immutable. For a bad release, publish a
  corrected version instead.
- If a release fails partway through npm publishing, fix the cause and rerun
  the workflow: packages already published at the identical tarball content
  are skipped; a version published with *different* content is a hard error.

## Exercising a release without publishing

```sh
goreleaser check
goreleaser release --snapshot --clean --skip=sign   # dist/ archives + checksums
node scripts/npm-release.mjs stage --version <v> --dist dist --out dist/npm
node --test "tests/**/*.test.mjs"                   # shim + manifest checks
actionlint .github/workflows/release.yml
```
