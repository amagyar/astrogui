# Spec Delta

## Purpose

Defines how versioned astrogui releases are built and distributed to npm and the Homebrew tap, so published binaries and package metadata stay aligned with the source tag.

## ADDED Requirements

### Requirement: Version-tagged releases use a single source version

The release process SHALL start from a pushed `v<version>` Git tag and SHALL use the tag's version for the GitHub release, every binary, all npm packages, and the Homebrew Cask. Tests and release builds SHALL pass before publishing to any distribution channel.

#### Scenario: Release tag passes validation
- **WHEN** a `v<version>` tag is pushed and its tests and builds pass
- **THEN** the release process creates version-stamped binary artifacts and proceeds with npm and Homebrew publication using that version

#### Scenario: Release validation fails
- **WHEN** tests or release builds fail for a `v<version>` tag
- **THEN** the workflow fails before publishing to npm or updating the Homebrew tap

#### Scenario: Binary reports the release version
- **WHEN** a user runs the binary published for a release tag
- **THEN** its version output matches the tag version without the leading `v`

### Requirement: npm publishes the scoped launcher and platform packages

For each release, the release process SHALL publish `@amagyar/astrogui` and six matching optional platform packages named `@amagyar/astrogui-<platform>-<arch>`. All seven package versions SHALL equal the release version. Each platform package SHALL contain its prebuilt binary and declare only its supported operating system and architecture. The launcher SHALL resolve the matching optional package without running a postinstall download or compiling the binary locally.

#### Scenario: Publish a release package family
- **WHEN** npm publication begins for a release that has passed validation
- **THEN** all six platform packages are published before `@amagyar/astrogui`
- **AND** the launcher package references the six platform packages at the same release version

#### Scenario: Install on a supported platform
- **WHEN** a user installs `@amagyar/astrogui` on a supported operating system and architecture
- **THEN** npm installs the matching platform package and exposes the `astrogui` executable
- **AND** installation does not require a postinstall download or local compilation

#### Scenario: Publish to npm using trusted identity
- **WHEN** GitHub Actions publishes any package in the release family to npm
- **THEN** it authenticates through npm trusted publishing with GitHub OIDC rather than a long-lived npm token

### Requirement: Homebrew tap receives a Cask for supported platforms

For each release, the release process SHALL directly update the `amagyar/homebrew-tap` repository with an `astrogui` Cask that installs prebuilt macOS and Linux binaries from that release. The Cask SHALL select the correct supported architecture, verify the downloaded archive with its SHA-256 checksum, and expose the `astrogui` command.

#### Scenario: Publish a Homebrew Cask update
- **WHEN** the release process publishes a validated version
- **THEN** it commits the generated `Casks/astrogui.rb` update directly to `amagyar/homebrew-tap`
- **AND** the Cask points to that release's versioned archives

#### Scenario: Install from Homebrew on a supported platform
- **WHEN** a user installs astrogui from the tap on macOS or Linux using a supported architecture
- **THEN** Homebrew installs the matching prebuilt binary from the GitHub release and exposes the `astrogui` command
- **AND** verifies the archive against the published checksum

#### Scenario: Tap credential is unavailable or rejected
- **WHEN** the release process cannot authenticate to update `amagyar/homebrew-tap`
- **THEN** the workflow reports failure and does not report Homebrew publication as successful
