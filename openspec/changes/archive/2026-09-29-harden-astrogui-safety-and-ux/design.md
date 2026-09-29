# Design

## Context

See `proposal.md` for motivation. The server currently relies on `safe.Guard.Check` followed by path-based writes, and its editor/API already exchange modification times and conflict contents. Project configuration has existing load, set, and save helpers, while the browser UI is embedded and served under a restrictive CSP. The implementation must retain the no-new-runtime-dependency model and the existing stage-all Git behavior.

## Goals / Non-Goals

**Goals:**
- Make writes resistant to symlink escapes and path races, including for newly created upload targets.
- Preserve user data and make recovery actions explicit across editor close and conflict flows.
- Make collection selection, Git result reporting, and UI behavior match the user-visible contracts in the delta specs.
- Keep the current APIs compatible unless additional response fields are needed for the UI.

**Non-Goals:**
- Change which files an explicitly confirmed `git add -A` action stages.
- Replace the Markdown renderer, execute Astro configuration, or permit remote preview content to run.
- Add a new runtime dependency or modify a blog project's manifests.

## Decisions

### Anchor writes to managed-directory roots

Use root-relative filesystem operations anchored to an opened managed-directory root for writes, rather than treating a successful string-path containment check as sufficient authorization. Resolve each post path relative to that root, reject unresolved/final symlinks, create uploaded assets exclusively (retrying a suffixed name on an existing entry), and perform atomic replacement operations through the same rooted boundary where supported. Keep `Guard.Check` for clear early errors, but do not rely on it alone to prevent a path-component swap between check and write.

**Alternative considered:** Extend `EvalSymlinks` fallback handling only. This closes the dangling-final-symlink case but leaves check/use races in ordinary path-based writes.

### Use existing project configuration as the collection preference store

When `Project.Collection` names a currently detected collection, use it without prompting. If it is absent or stale, keep the explicit selection prompt, update the per-project config through `File.Set`, and save it. If persistence fails, explain that the choice applies only to this run and surface the config error rather than silently claiming it was remembered.

**Alternative considered:** Persist only a content-directory override. That does not reliably identify a collection when several collections share configuration or when the detected layout changes.

### Serialize created frontmatter values as YAML scalars

Use a YAML-aware scalar encoder for string values written during post creation. Keep the pinned slug controlled by the post identity and retain deterministic field ordering. Add round-trip tests for hashes, colons, quotes, and line breaks.

**Alternative considered:** Add ad-hoc quoting rules to `yamlLine`. That would duplicate YAML escaping rules and remain fragile for control characters and multiline strings.

### Treat editor state as a snapshot with explicit conflict recovery

Track dirty state for body, structured fields, and raw frontmatter against their loaded/saved values. Route the Board button and dialog cancellation through one close guard; saving all changes should complete before close, and a failed save keeps the editor open. On a conflict, return the current bytes and modification time, show them alongside the retained editor snapshot, and require a deliberate choice to load disk or apply the retained/merged version against the version reviewed. Every retry remains conditional on that reviewed modification time.

**Alternative considered:** Automatically update the editor's modification time and retry the original save. This would silently replace the external edit and violate the no-lost-edits contract.

### Make stage-all scope visible before Git actions

Add a read-only Git status operation that reports tracked, staged, unstaged, and untracked paths in a machine-safe form. The UI presents that list and states that confirmation will stage the whole working tree; only confirmation proceeds to the existing commit or commit-and-push path. Preserve structured command, output, and success information on both successful and failed command executions, and have the client display the structured result even for non-2xx responses.

**Alternative considered:** Silently switch to committing only a selected post. That changes established behavior and could omit other posts the user expected in the single commit.

### Keep external preview images blocked and explain why

Retain the restrictive image CSP and do not assign remote references to `img.src`. Render a placeholder with the external URL or a concise notice instead, preventing tracking requests while making the preview's limitation visible. Continue hydrating local images only through the token-checked asset endpoint.

**Alternative considered:** Add `http:` and `https:` to `img-src`. This would make previews more faithful but would cause post content to trigger third-party network requests and could leak reader metadata.

### Add accessible board controls and responsive layout

Keep drag-and-drop as a pointer convenience, but give each card a semantic keyboard-activatable open control and a keyboard-usable state-transition control that invokes the same move endpoint. Add responsive layout rules that stack or intentionally scroll board columns and stack editor panes on narrow viewports.

**Alternative considered:** Replace drag-and-drop outright. Retaining it avoids regressing existing pointer users while adding a non-pointer path.

### Make browser opening a best-effort operation

Return immediately after `cmd.Start` fails, print the manual URL, and release the process only after a successful start. The HTTP listener remains the source of truth and must start regardless of whether a desktop opener exists.

## Risks / Trade-offs

- [Rooted filesystem APIs differ slightly across platforms] → Use standard-library rooted operations supported by the declared Go version and exercise symlink and atomic-write tests on supported OS builds.
- [A Git status preview can become stale if another process changes the worktree before confirmation] → Refresh immediately before presenting the confirmation, identify it as a snapshot, and avoid staging or changing the index until the user confirms.
- [Explicitly choosing the editor version after a conflict can replace the reviewed disk version] → Display both versions and require a separate, clearly labeled confirmation; reject the save if the file changes again.
- [Persisting a collection choice may fail due to config-directory permissions] → Continue with the explicit choice for the current run only and clearly report that it was not persisted.
- [Responsive and keyboard controls expand the UI surface] → Keep drag-and-drop intact and test keyboard operation and narrow viewports independently.

## Migration Plan

No project-data migration is required. Existing `collection` config values become effective; an absent or stale value triggers the existing explicit selection flow and is saved after selection. Rollback consists of reverting the application and embedded UI changes; existing posts, Git history, and project manifests remain untouched.
