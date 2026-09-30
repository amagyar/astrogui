# Design

## Context

See `proposal.md` for motivation. The repository currently has no `.github` workflow or GoReleaser configuration. The npm launcher and six optional platform package manifests are present, but use placeholder versions and the old unscoped/`@astrogui` package names. The launcher already resolves an OS/architecture-specific binary, and `main.version` is designed for linker stamping; `version_test.go` verifies that stamped version output.

The `dockupdate` release workflow provides a working repository precedent: `v*` tags, GoReleaser builds and release assets, a direct cross-repository tap update, and a downstream npm job using OIDC. The tap currently uses `Casks/dockupdate.rb`. Commit `715d301` removed dockupdate's Formula and recorded its Formula-to-Cask migration. astrogui is a new tap entry, so it does not need a migration entry.

## Goals / Non-Goals

**Goals:**

- Keep one Git tag as the version source for the Go binary, npm package family, GitHub release, and Homebrew Cask.
- Preserve the existing npm optional-dependency model and `astrogui` executable, with no postinstall download.
- Use the prebuilt release archives for both npm platform packages and Homebrew, avoiding duplicate builds.
- Match the existing release workflow's direct tap update, OIDC npm publishing, pinned actions, least-privilege permissions, and provenance practices.

**Non-Goals:**

- Publishing a Homebrew Formula or submitting astrogui to Homebrew core.
- Downloading the binary from GitHub during npm installation or compiling it on the user's machine.
- Adding a general pull-request CI workflow; this change gates tagged releases with tests.
- Changing the Go module path or the product's CLI name.

## Decisions

### Tag-driven release with GoReleaser artifacts

Trigger the release workflow on pushed `v*` tags. Run `go test ./...` before any publication, then use GoReleaser to build six targets (darwin, linux, and windows on amd64 and arm64), stamp `main.version` from the tag, create tar.gz archives and a SHA-256 checksum manifest, and publish a GitHub Release. The existing version test protects the linker-stamping contract. Archive and package versions omit the leading `v`.

The npm job runs after the release job and downloads the exact release archives/checksum manifest. This keeps the release assets as the single binary build used by both distribution channels and preserves the separation between release permissions and npm OIDC permissions.

### Keep the seven-package optional-dependency design

Rename the launcher to `@amagyar/astrogui` and platform packages to `@amagyar/astrogui-<platform>-<arch>`. CI sets all seven manifest versions and the launcher's optional-dependency versions from the tag, assembles one binary into each platform package, validates all package tarballs before publishing, and publishes the six platform packages before the launcher. The launcher continues resolving only the matching package and exposes the `astrogui` executable. Do not add a postinstall script.

This is preferred over dockupdate's postinstall-download design because the npm packages already have an explicit OS/CPU-filtered optional-dependency structure, and the user prefers installation without a network download script. The npm package repository metadata and README install command should point to the current repository and scoped package.

### Publish a Homebrew Cask, not a Formula

Use GoReleaser's `homebrew_casks` support to create `Casks/astrogui.rb` in `amagyar/homebrew-tap`. The Cask selects the four macOS/Linux archives and SHA-256 checksums, and installs the binary as the `astrogui` command. This follows the tap's current layout and its Formula-to-Cask transition in commit `715d301`; no `tap_migrations.json` entry is needed for a package that has never had a Formula.

Use a dedicated `HOMEBREW_TAP_GITHUB_TOKEN` with write access only to the tap. Check for the required secret before release work begins, and expose it only to the job that updates the tap.

### Separate publication credentials and permissions

Keep the release job and npm job separate. The release job receives the GitHub release permissions and tap-write secret. The npm job receives read-only repository access and `id-token: write`, uses npm trusted publishing through GitHub OIDC, and has no long-lived npm token. Configure the OIDC trusted publisher for each of the seven package names. Pin Actions to commit SHAs, retain runner hardening, and sign/attest the release artifacts as in the dockupdate workflow.

### Preserve the existing macOS Cask handling

The current dockupdate Cask uses a macOS postflight `xattr` command to remove the quarantine attribute from its unsigned CLI binary. Mirror that established behavior only if astrogui's Cask binary has the same Gatekeeper issue. This is a deliberate compatibility trade-off, not a security verification step; Apple code signing and notarization would be the safer alternative but are outside this change's current scope.

## Risks / Trade-offs

- **A platform package can publish before a later npm publish fails** → Build and validate every tarball before the first publish, publish platform packages before the launcher, report partial failures accurately, and make reruns skip only packages already published at the identical version/content; do not overwrite an immutable npm version.
- **The tap token grants cross-repository write access** → Use a dedicated tap-scoped credential, validate that it is present before release, and expose it only to the Cask publication job.
- **npm OIDC is not configured for every package** → Configure trusted publishing for the launcher and all six platform packages before the first release; stop and report accurately if npm rejects OIDC, including any platform packages already published.
- **Cask quarantine removal weakens macOS Gatekeeper protection** → Follow the existing dockupdate pattern only when necessary, document the behavior, and retain checksums plus artifact signatures/attestations; Apple signing/notarization remains a future alternative.
- **A tag, package version, or archive name drifts** → Derive versions from the tag, check the expected six-target matrix and archive names, and test the release configuration before its first real tag.

## Migration Plan

1. Configure npm trusted publishing for `@amagyar/astrogui` and all six `@amagyar/astrogui-<platform>-<arch>` packages, and add `HOMEBREW_TAP_GITHUB_TOKEN` as a repository secret.
2. Update README and package metadata to use the scoped install name while preserving the `astrogui` command. No user-facing binary command migration is needed.
3. Merge the workflow and release configuration, then exercise the build/package steps without publishing before pushing the first stable `v<version>` tag.
4. For an invalid published release, do not move or reuse its tag or attempt to overwrite npm versions; publish a corrected version. If only some npm packages were published, resume only after confirming the already-published package contents match the release.
