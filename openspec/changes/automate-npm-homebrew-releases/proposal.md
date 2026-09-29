# Proposal

## Why

astrogui has no automated release workflow, while its npm package manifests are still placeholders and the unscoped `astrogui` npm name is already taken. A tag-driven release will publish version-aligned prebuilt binaries to npm and Homebrew, under an npm scope the project controls, without requiring users to compile Go or run an install-time download script.

## What Changes

- Add a version-tagged GitHub Actions release flow (`v*`) that tests the project, builds version-stamped binaries for the six supported OS/architecture combinations, and creates checksummed GitHub release archives.
- Publish six platform-specific optional npm packages and the `@amagyar/astrogui` launcher package, all at the tag version. Keep the `astrogui` executable name and avoid a postinstall download; publish the platform packages before the launcher.
- Generate and directly update a Homebrew Cask in `amagyar/homebrew-tap` from the prebuilt macOS and Linux release archives. Use a dedicated tap-write credential and expose the `astrogui` command through the Cask.
- Use npm trusted publishing via OIDC and apply the release workflow's least-privilege and artifact-integrity practices.
- Update installation documentation for `@amagyar/astrogui` and the corresponding scoped platform packages.

## Capabilities

### New Capabilities

- `release-distribution`: Build and publish versioned astrogui releases to npm and the Homebrew tap from Git tags.

### Modified Capabilities

- `local-server`: Specify the scoped npm package used for global installation while preserving the `astrogui` command and prebuilt, no-compilation install behavior.

## Impact

- Adds GitHub Actions release automation and Go release configuration.
- Updates npm package manifests, platform package names, binary packaging, and the launcher’s package resolution.
- Updates README installation instructions.
- Writes generated Cask updates to `amagyar/homebrew-tap` using a repository secret; npm publishing uses OIDC trusted publishing.
