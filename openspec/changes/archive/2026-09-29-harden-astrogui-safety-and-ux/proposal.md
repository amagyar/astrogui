# Proposal

## Why

A review found a symlink edge case that can bypass the managed-directory write boundary, along with startup, collection-selection, editor-recovery, and Git-feedback defects. The editor also makes it too easy to lose unsaved work and is difficult to use on small screens or without drag-and-drop.

## What Changes

- Make asset writes safe against dangling symlinks and races, and add regression tests for writes that must remain inside managed directories.
- Honor and persist a valid collection selection; retain an explicit prompt when no valid selection is configured. Make browser-launch failure a nonfatal fallback and serialize created frontmatter strings safely.
- Protect all editor changes from accidental dismissal. On save conflicts, expose both versions and provide a deliberate recovery path that cannot silently overwrite either version.
- Surface full Git command output on failures and show the working-tree changes before the existing stage-all commit action, including for commit-and-push.
- Make board cards and lifecycle actions keyboard-accessible, adapt the board/editor layout to narrow screens, and make the preview clearly identify external images that are blocked to avoid unintended network requests.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `local-server`: preserve the managed-directory write boundary for symlinked upload targets and keep browser-launch failures nonfatal.
- `project-integration`: persist and honor the selected collection across runs.
- `post-editing`: protect unsaved work, provide conflict recovery with both versions available, safely create frontmatter, and clearly handle blocked external images.
- `post-lifecycle`: show the scope of stage-all commits and report command output for failures.
- `board`: support keyboard operation and usable layouts on narrow viewports.

## Impact

Affected areas include `internal/safe`, `internal/server`, `internal/lifecycle`, `internal/config`, `serve.go`, the embedded web UI, and their Go/Node tests. No new runtime dependencies or project-manifest changes are expected. The existing explicit Git behavior (`git add -A`, then commit, and optionally push) remains; the UI will make its scope visible before execution.
