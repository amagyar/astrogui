# Observed compliance facts (tasks 5.1, 5.2, 4.1, 4.2, 6.x)

Facts recorded during apply, each checkable in the repository or its settings.
These feed the badge-entry justifications drafted in task 7.1.

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
