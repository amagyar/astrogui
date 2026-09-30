# Proposal

## Why

astrogui's OpenSSF Best Practices badge entry (project 15108) stands at 33% — 8 of 24 Baseline Level 1 controls Met, 3 Unmet, 13 unanswered. The entry was created 2026-09-30 but the repository, documentation, and GitHub settings do not yet evidence the required practices. Reaching Baseline Level 1 makes the project's security posture legible to users and auditors, and closes gaps that matter independently of the badge (no LICENSE file anywhere in the repo, no security contact, unverified branch protections).

## What Changes

- Add an MIT `LICENSE` file at the repository root (license is already declared as MIT in `npm/package.json` and `.goreleaser.yml`), satisfying all four Legal controls (OSPS-LE-02.01/02.02/03.01/03.02).
- Ensure released artifacts carry the license: verify goreleaser archives include `LICENSE` (default file set does; confirm and pin if needed) so the license ships with every release asset (OSPS-LE-03.02).
- Add `SECURITY.md` with a security contact and disclosure policy (OSPS-VM-02.01).
- Record badge justifications that rest on facts already true today: user guide lives in `README.md` (OSPS-DO-01.01), Go `go.mod` + npm `package.json` enumerate direct dependencies (OSPS-QA-02.01), single-repository project so OSPS-QA-04.01 is N/A, no generated executables or unreviewable binaries tracked in git (OSPS-QA-05.01/05.02, with a git-history audit to confirm).
- Verify and document repository/CI protections, flipping the remaining unanswered controls to Met with justifications: lowest-privilege collaborator defaults (OSPS-AC-02.01), branch protection blocking direct commits to and deletion of `main` (OSPS-AC-03.01/03.02), sanitization posture of the tag-triggered release workflow (OSPS-BR-01.01), credential isolation in CI (OSPS-BR-01.03), and secrets-hygiene configuration such as push protection (OSPS-BR-07.01).
- Update the badge entry on bestpractices.dev with the new statuses and justifications, and embed the baseline badge in `README.md`.
- No product code changes; this change touches repository files, GitHub settings, and the badge entry only.

## Capabilities

### New Capabilities
- `openssf-baseline`: The project's repository, documentation, release assets, and OpenSSF badge entry SHALL satisfy OpenSSF Best Practices Baseline Level 1 and keep evidencing it as the project evolves.

### Modified Capabilities

## Impact

- Repository files: new `LICENSE`, new `SECURITY.md`, badge snippet in `README.md`.
- Release pipeline: `.goreleaser.yml` archive file list verified/adjusted to include `LICENSE` (build behavior otherwise unchanged).
- GitHub repository settings: branch protection on `main`, push protection/secret scanning — requires maintainer admin access, applied outside the codebase.
- Badge entry at bestpractices.dev project 15108: statuses and justification texts for 16 controls; no effect on `release-distribution` requirements.
