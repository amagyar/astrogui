# Observed compliance facts (tasks 5.1, 5.2, 4.1, 4.2, 6.x)

Facts recorded during apply, each checkable in the repository or its settings.
These feed the badge-entry justifications drafted in task 7.1.

## Badge-entry justification drafts (task 7.1)

Controls already Met on the entry keep their existing justifications
(AC-01.01, BR-03.01, BR-03.02, DO-02.01, GV-02.01, GV-03.01, QA-01.01,
QA-01.02). The sixteen below are the new/changed entries.

| Control | Status | Justification text |
|---|---|---|
| OSPS-AC-02.01 | Met | This is a personal repository with a single collaborator (the owner); GitHub personal repositories have no default-write access — collaborators are only ever added manually with an explicit permission choice, and none exist besides the owner. |
| OSPS-AC-03.01 | Met | Branch protection on `main` requires a pull request before merging and is enforced for administrators; a direct-push probe commit was rejected by the repository rules. |
| OSPS-AC-03.02 | Met | Branch protection on `main` blocks deletion (allow_deletions=false) and force pushes; verified via the branch-protection API read-back. |
| OSPS-BR-01.01 | Met | The release workflow triggers only on maintainer-pushed `v*` tags; the only tag-derived value (`GITHUB_REF_NAME`) is expanded exclusively inside double quotes in every run block (shellcheck-clean), and downloaded release archives are SHA-256-verified against the published checksums before any derived value is used. |
| OSPS-BR-01.03 | Met | No workflow job executing untrusted code exists: the tag-only trigger means only maintainer-tagged sources are ever checked out; every external action is pinned by full commit SHA, permissions are least-privilege per job (workflow default `contents: read`), npm publishing uses OIDC trusted publishing with no long-lived token, and step-security/harden-runner runs on every job. |
| OSPS-BR-07.01 | Met | `.gitignore` excludes local build artifacts; secret scanning and push protection are enabled on the repository (verified via the API); a full-history audit of all 182 blobs found zero credentials ever committed. |
| OSPS-DO-01.01 | Met | `README.md` is the user guide: it documents installation (npm/Homebrew), running inside an Astro project, board/editor/publish usage, configuration locations, and carries a visible warning that publishing moves the post folder into the content collection. |
| OSPS-LE-02.01 | Met | The project is released under the MIT license (OSI-approved and FSF-free), declared in `npm/package.json` and `.goreleaser.yml`. |
| OSPS-LE-02.02 | Met | Released binaries and packages are MIT licensed, matching the source; the Homebrew Cask declares `license: MIT` and the npm packages ship the MIT LICENSE file. |
| OSPS-LE-03.01 | Met | The MIT license text is maintained in the repository's root `LICENSE` file. |
| OSPS-LE-03.02 | Met | Every release archive includes `LICENSE` (pinned explicitly in `.goreleaser.yml` `archives.files`), every npm tarball contains `package/LICENSE` (copied at stage time and validated by the staging script), and a release-workflow gate re-checks both families before publication. Verified end-to-end against a snapshot build. |
| OSPS-QA-02.01 | Met | Go dependencies are enumerated in `go.mod` (`go mod tidy` produces no diff); the npm launcher manifest declares its dependencies — zero runtime dependencies plus the six prebuilt platform packages as optionalDependencies. |
| OSPS-QA-04.01 | N/A | astrogui is a single repository (github.com/amagyar/astrogui); there is no multi-repo list to document. |
| OSPS-QA-05.01 | Met | A full-history audit (every blob on every ref, magic-byte scan for ELF/Mach-O/PE) found no generated executable artifact ever committed; the local development binary is git-ignored and untracked. |
| OSPS-QA-05.02 | Met | The same full-history audit found no unreviewable binary artifacts; the only non-source files ever added are text test fixtures. |
| OSPS-VM-02.01 | Met | `SECURITY.md` publishes the security contact channel (GitHub private vulnerability reporting, verified available), reporting guidance, and acknowledgement/coordinated-disclosure expectations. |

## Release workflow (OSPS-BR-01.01, BR-01.03) — .github/workflows/release.yml

- Trigger scope: `on: push: tags: ['v*']` only. No `pull_request`,
  no `workflow_dispatch`. Only maintainers with tag-push access can start it;
  fork pull requests get no CI from this workflow at all.
- Every external action pinned by full commit SHA: step-security/harden-runner,
  actions/checkout, actions/setup-go, sigstore/cosign-installer,
  goreleaser/goreleaser-action, actions/attest-build-provenance,
  actions/setup-node.
- Workflow-level `permissions: contents: read` is the default for every job;
  the goreleaser job elevates exactly what it needs (contents: write,
  id-token: write, attestations: write), the npm job needs contents: read +
  id-token: write (OIDC trusted publishing, no long-lived npm token).
- No job that could execute untrusted code has access to privileged secrets:
  the workflow never checks out untrusted snapshots (tag-only trigger).
- Untrusted-metadata handling: `GITHUB_REF_NAME` (the only tag-derived value)
  is expanded only inside double quotes (shellcheck clean across all six run
  blocks, bash dialect, style level); downloaded release archives are
  SHA-256-verified against the published `checksums.txt` before any derived
  value is used.
- step-security/harden-runner with egress auditing on every job.

## Repository / GitHub settings (OSPS-AC, BR-07.01) — via gh api, 2026-09-30

- Branch protection on `main` active (API read-back + live probe):
  require pull request before merging (0 approvals required), force pushes
  blocked, deletion blocked, enforced for admins too. A direct push of a
  probe commit to `main` was rejected ("Changes must be made through a pull
  request") and nothing landed.
- secret_scanning: enabled
- secret_scanning_push_protection: enabled
- Push-protection probe honesty note: two synthetic invalid `ghp_` tokens
  (all-caps and random mixed-case) pushed to a scratch branch were NOT
  blocked and produced no secret-scanning alerts (checked immediately and
  after delay). GitHub validity-filters non-functional credentials, so only
  a real leaked token would be blocked. Remote and local probe branches were
  deleted immediately; no synthetic token remains in the repository.
- Collaborators: exactly one (amagyar, owner, admin). Personal repositories
  have no org-style base permission; collaborators can only be added manually
  with an explicit permission choice, and there are none besides the owner.
- 2FA: GitHub has required two-factor authentication for all users who
  actively contribute since March 2023 (platform mandate; the per-account
  API field is not readable with the current token scope).

## Version control hygiene (OSPS-QA-02/04/05) — audited 2026-09-30

- Full history (182 unique blobs, all refs): zero ELF / Mach-O / PE magic
  bytes; the only non-source paths ever added are text fixtures. No generated
  executable or unreviewable binary artifact was ever tracked.
- Local dev binary `./astrogui` is git-ignored and untracked.
- `go mod tidy` produces no diff (direct Go dependencies enumerated in
  go.mod). The npm launcher has zero runtime `dependencies`; its
  `optionalDependencies` are exactly the six prebuilt platform packages.
- Single repository; no multi-repo list needed (OSPS-QA-04.01 N/A).

## License (OSPS-LE-02/03) — tasks 1.1-1.4

- MIT `LICENSE` at repo root (2026, Allain Magyar); matches
  `npm/package.json` ("license": "MIT") and `.goreleaser.yml`
  (homebrew_casks license: MIT).
- goreleaser `archives.files` pins LICENSE; snapshot build verified LICENSE
  present in all six platform archives.
- npm stage copies the root LICENSE into all seven packages and fails unless
  every tarball contains `package/LICENSE` (verified on a dry-run stage of
  0.0.1-snapshot).
- Release workflow gate re-checks archives and tarballs for the license
  before npm publish (positive + negative control verified locally).
