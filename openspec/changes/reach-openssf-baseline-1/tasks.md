# Tasks

## 1. License (OSPS-LE-02.01/02.02/03.01/03.02)

- [x] 1.1 Add MIT `LICENSE` file at repository root with copyright line "Allain Magyar"; verify `head LICENSE` shows MIT text and matches the license declared in `npm/package.json` and `.goreleaser.yml`
- [x] 1.2 Pin `LICENSE` explicitly in `.goreleaser.yml` `archives.files` so it cannot regress with tool upgrades; verify with `goreleaser check` (or `goreleaser release --snapshot --clean` into a scratch dist) that archives contain `LICENSE`
- [x] 1.3 Extend the npm staging path (`scripts/npm-release.mjs`) so each staged npm tarball includes the license file; verify by inspecting the staged tarballs (`tar -tf`) from a dry-run stage
- [x] 1.4 Add a verification step to `.github/workflows/release.yml` that fails before publication if any staged artifact (archives, npm tarballs, checksums inputs) lacks `LICENSE`; verify by triggering the check against a snapshot build

## 2. Security contact (OSPS-VM-02.01)

- [x] 2.1 Add `SECURITY.md` at repository root naming GitHub private vulnerability reporting as the reporting channel and stating acknowledgement/coordinated-disclosure expectations; verify the file renders on GitHub and the private reporting link is enabled in repo settings

## 3. User documentation (OSPS-DO-01.01)

- [x] 3.1 Review `README.md` against the spec scenario (install, run inside an Astro project, board/editor/publish usage) and close gaps: document the collection-selection prompt and add a visible warning on the publish step since it moves content directories; verify a fresh read-through lets a new user install and run without other sources
- [x] 3.2 Embed the baseline badge in `README.md` (`[![OpenSSF Baseline](https://www.bestpractices.dev/projects/15108/baseline)](https://www.bestpractices.dev/projects/15108)`); verify the badge image renders on GitHub

## 4. Version control hygiene (OSPS-QA-02.01, QA-04.01, QA-05.01/05.02)

- [x] 4.1 Run a one-time history audit: `git log --all --diff-filter=A --name-only` plus a magic-byte scan of history blobs for ELF (`\x7fELF`), Mach-O (`\xcf\xfa\xed\xfe`, `\xfe\xed\xfa\xcf`), and PE (`MZ`) signatures; verify no generated executable or unreviewable binary was ever tracked and record the result for the badge justification
- [x] 4.2 Confirm `go.mod` direct-require list and `npm/package.json` dependencies are current (`go mod tidy` produces no diff; `npm/package.json` lists the launcher's runtime needs); verify manifests answer OSPS-QA-02.01 factually

## 5. Release workflow hardening pass (OSPS-BR-01.01, BR-01.03, BR-07.01 evidence)

- [x] 5.1 Audit `.github/workflows/release.yml` for unquoted shell interpolations of `GITHUB_REF_NAME` and other workflow values; quote anything unquoted; verify `shellcheck` (or manual review) reports no unquoted variable expansions in the workflow's `run` blocks
- [x] 5.2 Confirm no workflow job with access to privileged secrets executes untrusted code and that trigger scope is maintainer tag pushes only; record observed properties (SHA-pinned actions, per-job permissions, OIDC) as justification facts

## 6. GitHub repository settings (OSPS-AC-01.01, AC-02.01, AC-03.01/03.02, BR-07.01)

- [ ] 6.1 Enable branch protection on `main`: require pull request before merging, block force pushes, block branch deletion; verify by attempting a direct push to `main` (rejected) and reviewing the protection rules
- [ ] 6.2 Enable secret scanning and push protection; verify with a dry-run push of a synthetic credential-shaped string to a scratch branch (blocked) and remove the string afterwards
- [ ] 6.3 Verify collaborator default permission is the lowest available role and 2FA is enforced for the account/org; record the observed values for the AC-02.01 and AC-01.01 justifications

## 7. Badge entry update (project 15108, all 24 controls)

- [ ] 7.1 Draft the justification text for each control from the recorded facts of tasks 1-6 (one to two sentences each, citing checkable artifacts: file paths, workflow properties, settings); keep the drafts in the change notes for review
- [ ] 7.2 Update the badge entry at bestpractices.dev project 15108: set OSPS-LE-02.01/02.02/03.01/03.02, VM-02.01, DO-01.01, QA-02.01, QA-04.01 (N/A), QA-05.01/05.02, AC-02.01, AC-03.01/03.02, BR-01.01, BR-01.03, BR-07.01 to Met or N/A with the drafted justifications; verify the page shows all 24 controls Met or N/A
- [ ] 7.3 Final integration check: re-read the badge page against this change's `specs/openssf-baseline/spec.md` scenario-by-scenario and confirm every justification matches the actual repository state; note the percentage reached (target: 100% of Level 1 controls)
