# Tasks

## 1. Foundation and distribution

- [ ] 1.1 Initialize a Go module with a CLI entry point that accepts no arguments, and verify `go build ./...` and `go run . --help` both succeed
- [ ] 1.2 Add a version command stamped at build time, and verify the reported version matches the value passed to the linker
- [ ] 1.3 Define the tool's own configuration file format (draft directories, content directory override, staleness threshold), defaulting to `drafts/ideas`, `drafts/wip`, and the detected content directory, and verify a config round-trips and that absent keys fall back to defaults
- [ ] 1.4 Create the npm package with a small JavaScript shim that execs the platform binary, plus per-platform optional dependency stubs, and verify the shim resolves and runs the binary locally
- [ ] 1.5 Write the Homebrew formula against the release artifact layout, and verify `brew style` passes and the formula's URL and checksum placeholders are documented in the repository
- [ ] 1.6 Document the install paths for npm and Homebrew, and verify each documented command is the one the repository actually implements

## 2. Project resolution and the write boundary

- [ ] 2.1 Detect an Astro project by walking up from the working directory, and verify detection succeeds from a nested subdirectory and fails in a non-Astro directory with a message naming the directory searched
- [ ] 2.2 Resolve the content collection directory from the project layout, and verify the conventional layout is detected and a user override takes precedence
- [ ] 2.3 When multiple collections are present, prompt for which to manage, and verify no collection is selected without an explicit answer
- [ ] 2.4 Implement a single path-containment guard that every write passes through, and verify it accepts paths inside the managed directories and rejects paths outside them, including after symlink resolution
- [ ] 2.5 Add a test that runs a full create-edit-publish cycle against a fixture project and asserts no file outside the draft and content directories was created, modified, or deleted

## 3. Post model, listing, and change tracking

- [ ] 3.1 Read a post folder into a model with its name, frontmatter, body, and asset list, and verify a fixture post round-trips through the model unchanged
- [ ] 3.2 List loose markdown files in the content directory alongside folder posts, marking loose ones read-only, and verify both shapes appear without either being rewritten
- [ ] 3.3 Derive per-post metadata — first seen, last modified, body size, referenced image count — and verify the values match the filesystem for a fixture with known timestamps
- [ ] 3.4 Watch the managed directories and emit change events for create, modify, rename, and delete, and verify a post created outside the tool produces an event
- [ ] 3.5 Store first-seen timestamps in a derived cache under the tool's own directory, and verify deleting the cache leaves the board fully reconstructible with no post lost

## 4. Lifecycle moves and the publish gate

- [ ] 4.1 Implement a lifecycle transition as a single atomic move between state directories, and verify a move either fully succeeds or leaves both locations unchanged when interrupted
- [ ] 4.2 Verify a transition preserves post contents byte-for-byte and preserves image files, with a test comparing file hashes before and after
- [ ] 4.3 Create new posts as folders carrying a `slug` pinned to the post name, and verify the published URL is derived from the slug rather than the file path
- [ ] 4.4 Implement pre-flight checks covering missing referenced images, missing title, unparseable or future publication date, and empty body, and verify each failure blocks the move and names the specific problem
- [ ] 4.5 Verify a post referencing an asset outside its own directory fails the pre-flight check, and that the failure names the offending reference
- [ ] 4.6 Refuse a move whose destination already exists, and verify both posts are left unchanged
- [ ] 4.7 Add a test that a move is recorded as a rename by version control, so the post's history is continuous across states

## 5. Local server and its security controls

- [ ] 5.1 Serve the embedded interface and an API rooted at `/api/collections/:name/entries`, and verify a collection is addressable by name rather than only the default
- [ ] 5.2 Bind the listener to the loopback interface only, and verify a request from a non-loopback address is not served
- [ ] 5.3 Validate the `Host` header against the expected origin and reject mismatches, and verify a rebinding-style hostname is refused
- [ ] 5.4 Mint a session token at startup, deliver it in the URL fragment, and require it on every API call, and verify a request without it is refused with no filesystem effect
- [ ] 5.5 Serve a post's assets only from that post's directory, resolving containment by real path, and verify a traversal attempt and a symlink escape are both refused while a legitimate image is served
- [ ] 5.6 Report a port conflict and either select a free port or exit, and verify the tool never silently takes over a port it does not own
- [ ] 5.7 Add tests asserting the interface is reachable only from loopback and that no filesystem-affecting endpoint responds without the session token

## 6. Version control actions

- [ ] 6.1 Implement a commit action that stages the working tree and prompts for a message, and verify several posts published beforehand are included in a single commit
- [ ] 6.2 Implement a commit-and-push action that pushes to the tracked remote, and verify a push failure is reported with its output and not reported as success
- [ ] 6.3 Verify a lifecycle move triggers no version control operation, so staging stays a separate explicit act
- [ ] 6.4 Report plainly when a configured command is unavailable, and verify no action is reported as successful in that case

## 7. Board interface

- [ ] 7.1 Render one column per lifecycle directory with cards drawn from the filesystem, and verify the board is identical after a restart
- [ ] 7.2 Show age, last change, and size on each card, and verify a fixture with known timestamps and sizes renders those values
- [ ] 7.3 Mark cards stalled past the configured threshold and recompute when the threshold changes, and verify changing the threshold updates markings without modifying any post
- [ ] 7.4 Support dragging a card between columns, issuing one atomic move per drop, and verify the resulting directories match the drop target
- [ ] 7.5 Reflect filesystem changes made outside the tool without a reload, and verify a post created, renamed, moved, or deleted externally updates the board
- [ ] 7.6 Add a capture flow for a bare idea requiring no structured metadata, and verify a single line of text becomes a dated card

## 8. Editor interface

- [ ] 8.1 Build the split pane with a source textarea bound to the file's bytes and a rendered reading copy, and verify the preview updates as the source is typed
- [ ] 8.2 Verify a fixture post containing code-fence metadata, raw HTML, component tags, footnotes, tables, and trailing-space line breaks is byte-identical after open, edit, and save
- [ ] 8.3 Verify saving without a body change leaves the file's contents and modification time untouched
- [ ] 8.4 Resolve relative image references in the preview against the post's own directory, and verify a missing reference renders a visible broken-reference indication
- [ ] 8.5 Edit frontmatter through structured fields using a comment- and order-preserving YAML document, and verify unrecognised fields and comments survive an edit to a recognised field
- [ ] 8.6 Provide a raw frontmatter view for direct editing, and verify a raw edit is reflected in the post and in the board card
- [ ] 8.7 Verify no frontmatter rewrite occurs when no field changed
- [ ] 8.8 Save a pasted image into the post's own directory and insert a relative reference at the insertion point, and verify the reference resolves before and after a publish move
- [ ] 8.9 Name an unnamed post and its pasted image usefully rather than failing, and verify the resulting post publishes successfully

## 9. Integration

- [ ] 9.1 Build a fixture Astro project with a `glob()` content collection and run the complete flow end to end: capture an idea, write it with an image, publish it, and confirm the project builds with the post visible at the expected URL
- [ ] 9.2 Confirm a fixture project that has never run astrogui builds identically before and after, and that the project's dependency manifest is unchanged
- [ ] 9.3 Verify a project whose content directory holds loose markdown files shows them read-only and is never rewritten, and that only drafts created by astrogui use the folder layout
- [ ] 9.4 Verify the whole flow against a project with an unrecognised frontmatter field and a custom `generateId`, confirming the pinned slug still yields the expected URL
- [ ] 9.5 Document the on-disk convention, the boundary rule, the pre-flight checks and their limits, and the preview's fidelity ceiling, and verify each documented behavior matches the implemented behavior
