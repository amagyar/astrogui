# openssf-baseline Specification

## Purpose

Keep astrogui in compliance with OpenSSF Best Practices Baseline Level 1: the repository, documentation, release assets, GitHub settings, and the project's badge entry must continuously evidence the 24 Level 1 controls.

## Requirements

### Requirement: Repository carries an OSI-approved license

The repository SHALL contain a `LICENSE` file at its root containing the MIT license text, matching the license already declared in `npm/package.json` and `.goreleaser.yml`. The license SHALL satisfy the OSI Open Source Definition so that both the source code and the released software assets meet OSPS-LE-02.01 and OSPS-LE-02.02, and its presence satisfies OSPS-LE-03.01.

#### Scenario: License file is present and consistent

- **WHEN** the repository root is inspected
- **THEN** a `LICENSE` file exists containing the MIT license text
- **AND** the license declared in `npm/package.json` and `.goreleaser.yml` is `MIT`, matching the file

### Requirement: Released artifacts carry the license

Every release archive published by the release pipeline SHALL include the `LICENSE` file alongside the binaries, so the license for the released software assets ships with the assets (OSPS-LE-03.02). This SHALL hold for every artifact family (GitHub release archives, npm packages, Homebrew Cask) without changing the `release-distribution` versioning or publication requirements.

#### Scenario: Release archive includes the license

- **WHEN** a release archive from a published GitHub release is extracted
- **THEN** a `LICENSE` file is present at the top level of the archive

#### Scenario: License missing from archive fails release preparation

- **WHEN** the release pipeline stages archives that do not contain the license file
- **THEN** the verification step fails before any publication channel is updated

### Requirement: Security contact is published

The repository SHALL contain a `SECURITY.md` file at its root that identifies how to privately report security vulnerabilities for astrogui and states the project's disclosure expectations, satisfying OSPS-VM-02.01.

#### Scenario: Reporter finds the security contact

- **WHEN** someone wants to report a security vulnerability in astrogui
- **THEN** `SECURITY.md` names a working private contact channel (GitHub private vulnerability reporting or a maintainer email)
- **AND** sets an expectation for acknowledgement and coordinated disclosure

### Requirement: User documentation covers basic functionality

`README.md` SHALL serve as the project's user guide: it SHALL explain how to install astrogui, how to run it inside an Astro project, and how to use the board and editor, satisfying OSPS-DO-01.01 by documentation rather than a separate docs folder. The badge entry SHALL record this with a justification referencing `README.md`.

#### Scenario: New user installs and runs from the README alone

- **WHEN** a user follows only the instructions in `README.md`
- **THEN** they can install astrogui, launch it inside an Astro project, and use the board, editor, and publish flow
- **AND** any dangerous or destructive action documented in the README (such as publishing, which moves content directories) carries a visible explanation

### Requirement: Version control contains only reviewable sources

The version control system SHALL NOT track generated executable artifacts or unreviewable binary artifacts (OSPS-QA-05.01, OSPS-QA-05.02), and the repository history SHALL be free of such artifacts. Direct language dependencies SHALL be enumerated in tracked manifests: `go.mod` for the Go implementation and `npm/package.json` for the launcher (OSPS-QA-02.01). The project operates a single repository, so OSPS-QA-04.01 is N/A.

#### Scenario: No generated binaries are tracked

- **WHEN** the tracked file list and its full history are audited
- **THEN** no compiled executable or unreviewable binary artifact is tracked
- **AND** the local development binary remains git-ignored

#### Scenario: Direct dependencies are enumerable

- **WHEN** the repository is inspected for dependency manifests
- **THEN** `go.mod` lists the Go module's direct dependencies and `npm/package.json` lists the launcher's runtime dependencies

### Requirement: Repository access protections are enforced

The GitHub repository SHALL protect its primary branch `main` so that direct pushes are rejected and branch deletion requires explicit confirmation (OSPS-AC-03.01, OSPS-AC-03.02); SHALL keep collaborator permissions at the lowest available privileges by default (OSPS-AC-02.01); and SHALL require multi-factor authentication for collaborators (OSPS-AC-01.01).

#### Scenario: Direct commit to main is rejected

- **WHEN** a collaborator attempts to push a commit directly to `main` without a pull request
- **THEN** the push is rejected by branch protection

#### Scenario: Branch deletion is blocked

- **WHEN** deletion of `main` is attempted
- **THEN** the version control system requires explicit confirmation and the protected branch cannot be deleted in the normal course of work

### Requirement: Release pipeline handles untrusted input and credentials safely

The release workflow SHALL keep privileged credentials isolated from any untrusted code or metadata (OSPS-BR-01.03) and SHALL sanitize and validate untrusted metadata before use (OSPS-BR-01.01). The workflow SHALL continue to trigger only on maintainer-pushed version tags, quote all interpolated values, verify downloaded release archives against published checksums before use, and run jobs with least-privilege permissions. The repository SHALL enable secret scanning and push protection so unencrypted secrets cannot be committed (OSPS-BR-07.01).

#### Scenario: Fork pull requests never see privileged credentials

- **WHEN** CI runs for an untrusted code snapshot (such as a fork pull request)
- **THEN** the job either does not run or runs without access to privileged secrets and write permissions

#### Scenario: Release workflow input stays validated

- **WHEN** the release workflow derives the version from the pushed tag
- **THEN** all shell interpolations of the tag value are quoted and archives are checksum-verified before any derived value is forwarded to publication

#### Scenario: Secret accidentally staged in a commit is blocked

- **WHEN** a commit containing a recognized credential pattern is pushed
- **THEN** push protection rejects it before it enters the version control system

### Requirement: Badge entry reflects Baseline Level 1 compliance

The OpenSSF Best Practices badge entry for project 15108 SHALL record every Baseline Level 1 control as Met or N/A, each unanswered control accompanied by a justification that states the factual basis documented in this capability. `README.md` SHALL embed the baseline badge so the project page shows current status, and the entry SHALL be kept current when repository practices change.

#### Scenario: All Level 1 controls are answered

- **WHEN** the badge page for project 15108 is viewed
- **THEN** all 24 Baseline Level 1 controls show Met or N/A
- **AND** each carries a justification consistent with the actual repository state

#### Scenario: README shows the badge

- **WHEN** `README.md` is viewed on GitHub or npm
- **THEN** it embeds the baseline badge image linking to the project's badge page
