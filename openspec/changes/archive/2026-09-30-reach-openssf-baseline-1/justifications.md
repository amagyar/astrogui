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

- secret_scanning: enabled
- secret_scanning_push_protection: enabled
- Branch protection on main: see tasks.md 6.1 result.
- Collaborators: see tasks.md 6.3 result.

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

## Final result (task 7.3, 2026-09-30 16:49 UTC)

Badge entry: 24/24 Baseline Level 1 controls answered — 23 Met + 1 N/A
(QA-04.01, single repository). Badge status: passing; in_progress marker
gone. All spec scenarios re-verified against live main post-merge: LICENSE
resolves (200) and matches npm/goreleaser MIT declarations; goreleaser pins
LICENSE into archives; the release workflow carries the license gate; the
npm staging script validates package/LICENSE; README embeds the badge and
carries the publish warning; branch protection read-back confirms require-PR,
no-force-push, no-deletion, admin-enforced. Process note: the first badge
save (88%, 21/24) preceded the merge, so the auto-checker correctly held
DO-01.01/LE-03.01/LE-03.02 Unmet against main; merging PR #8 and re-saving
the three controls with the drafted justifications completed the entry.
