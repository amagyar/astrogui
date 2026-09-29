# Tasks

## 1. Secure managed-directory writes

- [x] 1.1 Add rooted, symlink-safe write operations for managed paths and use exclusive creation for uploads; verify with unit tests for dangling final symlinks, escaping parent symlinks, existing-name collisions, and valid uploads.
- [x] 1.2 Route post creation and atomic post replacement through the rooted write boundary; verify with lifecycle/post tests that concurrent path changes cannot write outside the configured roots and that failed writes leave original files intact.
- [x] 1.3 Add an API regression test that uploads to a name occupied by a dangling symlink pointing outside the project; verify the request is refused or safely chooses a new in-post name and the outside target remains absent.

## 2. Startup, project selection, and metadata correctness

- [x] 2.1 Honor a valid configured collection, prompt only when the preference is missing or stale, and persist a newly selected collection; verify valid-choice, stale-choice, and save-error behavior with project/config tests.
- [x] 2.2 Make browser launch best-effort and test that a failed command start prints the manual URL without panicking or preventing the server from starting.
- [x] 2.3 Encode created frontmatter string values as YAML scalars; verify create-and-read round trips for hashes, colons, quotes, and multiline values without adding fields.
- [x] 2.4 Update the README's collection-selection and startup notes; verify the documented behavior matches the config and opener tests.

## 3. Editor data-loss prevention and preview behavior

- [x] 3.1 Track unsaved body, structured-field, and raw-frontmatter edits and route Board, Escape, and other close paths through a shared confirmation; verify clean close and dirty keep/discard/save behavior for every editor surface.
- [x] 3.2 Return current file bytes and modification time on save conflicts, and implement a UI recovery dialog that displays both versions and requires an explicit reload or reviewed replacement/merge; verify a second external change causes another conflict rather than being overwritten.
- [x] 3.3 Update editor tests and README guidance for unsaved changes and conflict recovery; verify the user-facing instructions describe the explicit choices and no silent-discard path remains.
- [x] 3.4 Keep remote preview images from issuing network requests and render a clear placeholder; verify external HTTP/HTTPS/protocol-relative references do not receive `src`, while local image hydration and broken-local-image indication still work.

## 4. Safe and transparent Git actions

- [x] 4.1 Add a machine-safe Git status preview for staged, unstaged, and untracked paths; verify parsing with spaces, unusual characters, renames, and an empty working tree.
- [x] 4.2 Require confirmation of the stage-all scope before commit or push, while preserving the existing `git add -A` behavior; verify cancel performs no Git operation and confirmation includes all expected changes.
- [x] 4.3 Preserve command results for commit and push failures through the API and render their full command/output in the UI; verify hook/commit and push failures are not reported as success and show diagnostic output.
- [x] 4.4 Update README Git-action guidance to explain the stage-all preview and confirmation; verify the text reflects the implemented behavior.

## 5. Accessible and responsive board

- [x] 5.1 Add semantic keyboard-operable card opening and a keyboard-accessible lifecycle destination control that uses the existing validated move path; verify keyboard-only open and move flows, including publish-gate failures.
- [x] 5.2 Add narrow-viewport layouts for board columns and editor panes; verify at desktop and mobile-sized viewports that controls remain reachable and content has no unintended horizontal page overflow.
- [x] 5.3 Update relevant board/editor UI tests and user guidance; verify drag-and-drop remains available and keyboard operation is discoverable.

## 6. Integration verification

- [x] 6.1 Run `go test ./...`, `go vet ./...`, and `node tests/markdown.test.mjs`; verify all pass.
- [x] 6.2 Run the end-to-end Astro workflow where dependencies are available; verify create/edit/publish still builds correctly and no project files outside managed directories are created or changed.
