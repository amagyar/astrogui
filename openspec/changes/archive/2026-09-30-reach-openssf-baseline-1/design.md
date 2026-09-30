# Design

## Context

The badge entry (project 15108, criteria v2026.08.28) shows 8 Met / 3 Unmet / 13 unanswered. Repository facts verified on 2026-09-30: no `LICENSE`, `SECURITY.md`, or `docs/` exists; `npm/package.json` and `.goreleaser.yml` already declare MIT; the release workflow (`.github/workflows/release.yml`) is tag-triggered, SHA-pinned, per-job least-privilege, OIDC-based, and checksum-verifies archives; the local dev binary is git-ignored and untracked. GitHub repository settings (branch protection, push protection) cannot be expressed in the repo and are applied out-of-band.

## Goals / Non-Goals

**Goals:**
- Every one of the 24 Level 1 controls recorded as Met or N/A with a justification that matches reality.
- Repo files and release assets that make those justifications durable (license ships with releases, security contact published).
- Exact GitHub settings documented so the out-of-band application is mechanical.

**Non-Goals:**
- Level 2/3 criteria; no spec changes to `release-distribution` mechanics.
- Configuration-as-code for GitHub settings (Terraform etc.).
- A permanent CI gate for binary-artifact detection (one-time history audit only).
- Signing or notarization changes; the existing cosign/provenance pipeline stays as is.

## Decisions

- **License is MIT, single root file.** Already declared in `npm/package.json` and `.goreleaser.yml`; introducing any other license would create a three-way inconsistency. Alternatives considered: Apache-2.0 (rejected: contradicts existing metadata), dual licensing (needless for this project). Copyright line: "Allain Magyar".

- **OSPS-DO-01.01 via justification, not a `docs/` folder.** The README already covers install, invocation, board/editor/publish usage. Creating `docs/` would duplicate that content purely to please the checker's folder heuristic. Justification text references the README sections. Alternative (docs folder) is trivial to switch to later if the project outgrows one page; the spec is written so either satisfies it.

- **OSPS-BR-01.01/01.03 via justification anchored in observable workflow properties.** The workflow triggers only on `v*` tags pushed by maintainers, quotes every interpolation of `GITHUB_REF_NAME`, checksum-verifies archives before use, pins actions by SHA, grants per-job least-privilege permissions, and exposes no secrets to any job that executes untrusted code (none exists: it builds only maintainer-tagged sources). A small hardening pass still verifies there is no unquoted interpolation before the justifications are recorded — justify facts, not intentions.

- **License-in-archives made deterministic.** goreleaser's default archive file set includes `LICENSE*`, but the spec requires failure when it is absent. Pin `LICENSE` explicitly in `.goreleaser.yml` `archives.files`, extend the npm staging path so each npm tarball contains the license file, and add a verification step to the release workflow that checks every staged artifact for the license before publication. The Cask needs no change: it installs the same verified archives.

- **GitHub settings applied by maintainer with admin, documented exactly.** Required settings: branch protection on `main` (require pull request, block force pushes, block deletion), collaborator default role `Read`, MFA enforcement (org/repo level; already satisfied by GitHub-wide 2FA for the justification on record), secret scanning + push protection enabled. Alternative rejected: keeping instructions only in a GitHub issue (drift); they live in `tasks.md` where the apply pass executes them.

- **One-time history audit, not a recurring gate.** `git log --all --name-only --diff-filter=A` plus a magic-byte scan for ELF/Mach-O/PE in history blobs confirms no generated executable or unreviewable binary ever landed (expected: clean, since the working tree binary is ignored). Recurring enforcement is a Non-Goal; if binaries ever appear, `.gitignore` plus push protection is the guard.

- **Badge entry updated last, in one pass.** Statuses and justification texts are drafted in `tasks.md` during apply, after repo files and settings are in place, so every justification states a current fact. The entry is updated via the web UI (or the page's automation-proposal URL); entry owner is the maintainer.

- **Security contact: GitHub private vulnerability reporting** as the primary channel (no public email to scrape, integrated with advisories), with a maintainer email as fallback only if the maintainer wants one published. Trivial to swap in `SECURITY.md` later.

## Risks / Trade-offs

- [Branch protection blocks solo maintainer's direct pushes to `main`] → Accepted: the project already merges via PRs for reviewed changes; protection is the point of the control. Can be temporarily relaxed by the admin if a hotfix demands it, then re-enabled.
- [Push protection may flag test fixtures in `testdata/` containing credential-looking strings] → Mitigation: audit flagged paths; allowlist specific patterns only after confirming they are synthetic.
- [Justifications are self-attested and could drift from reality] → Mitigation: every justification cites a checkable artifact (file path, workflow property, setting); the `openssf-baseline` spec requires the entry to track repository changes.
- [npm tarball layout change (adding LICENSE) could break the launcher's expectations] → Mitigation: launcher only reads `index.js` and the platform binary; license files are inert additions; the release workflow's smoke checks already run before publish.

## Open Questions

- None. The security-contact channel choice defaults to GitHub private vulnerability reporting and can be swapped without touching specs or tasks.
