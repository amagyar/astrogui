# Tasks

## 1. Scoped npm package family

- [x] 1.1 Rename the launcher package to `@amagyar/astrogui` and the six optional packages to `@amagyar/astrogui-<platform>-<arch>`; update dependency versions, repository metadata, and shim resolution while keeping the executable name `astrogui`. Add or update shim tests and verify all six platform mappings, local-binary fallback, and missing-binary errors with `node --test`.
- [x] 1.2 Add release packaging that derives all seven npm package versions from the `v<version>` tag and places the matching prebuilt binary in each platform package without a postinstall script. Verify the six platform tarballs contain the correct executable and platform metadata, and `npm pack --dry-run` succeeds for every package.
- [x] 1.3 Update installation and development documentation to use `npm install -g @amagyar/astrogui` and `npx @amagyar/astrogui`, explain supported platforms, and preserve the `astrogui` command. Verify the documented package names match the manifests and launcher.

## 2. Release archives and Homebrew Cask

- [x] 2.1 Add GoReleaser configuration for six OS/architecture builds, tag-derived `main.version`, tar.gz archives, SHA-256 checksums, and artifact signing/provenance. Verify `goreleaser check` and a snapshot build produce the expected six archives and checksum entries; confirm the existing version test reports the tag version.
- [x] 2.2 Configure GoReleaser to publish an `astrogui` Cask to `amagyar/homebrew-tap`, using the macOS and Linux release archives and exposing the `astrogui` binary. Verify the generated Cask targets all four supported Homebrew combinations and contains the matching release URLs and SHA-256 values.
- [x] 2.3 Validate the generated Cask behavior, including whether the existing dockupdate-style macOS quarantine hook is needed. Verify the resulting Cask passes Homebrew's Cask audit where available, and do not add an astrogui Formula migration entry unless a Formula has actually been published.

## 3. Tag-triggered GitHub Actions release

- [x] 3.1 Add a `v*` tag-triggered release workflow that runs `go test ./...` before publishing and stops on test/build failure. Pin actions by commit SHA and verify workflow syntax with `actionlint`.
- [x] 3.2 Add a preflight for `HOMEBREW_TAP_GITHUB_TOKEN`, then run GoReleaser with only the required GitHub release, attestation, and tap-write permissions. Verify the tap credential is scoped to `amagyar/homebrew-tap` and is not exposed to the npm job.
- [x] 3.3 Add a downstream npm job using GitHub OIDC trusted publishing, download the exact release archives, validate all seven tarballs before publishing, and publish platform packages before `@amagyar/astrogui`. Verify the job has `id-token: write`, no long-lived npm token, and fails rather than claiming success when npm rejects publication.
- [x] 3.4 Document one-time release setup for npm trusted publishers on all seven packages and the tap-write repository secret. Verify the documented setup names match the workflow and package manifests.

## 4. Release integration checks

- [x] 4.1 Exercise the complete release path in non-publishing/snapshot mode and verify tag-derived versions, all six binary targets, npm package contents and ordering, generated Cask metadata, checksums, and signatures/attestations agree before the first real release tag.
