# Tasks

## 1. Literal code spans in the markdown renderer

- [ ] 1.1 In `internal/server/web/markdown.js`, extract code spans into sentinel placeholders before image/link/emphasis passes and substitute them back (escaped) after; verify with new cases in `tests/markdown.test.mjs`: `` `**x**` `` renders as literal code, `![a](b)` inside backticks does not become an image, and existing inline-formatting tests still pass via `node --test tests/markdown.test.mjs`

## 2. Bounded preview updates

- [ ] 2.1 Debounce the `input` → `updatePreview` wiring at 200ms trailing edge in `app.js`; verify by typing a burst and observing a single render, and that the ui-contract test for the input listener is updated accordingly
- [ ] 2.2 Save and restore `#preview` scroll position around the innerHTML swap; verify the pane no longer jumps to top while typing mid-document

## 3. Image hydration cache

- [ ] 3.1 Add a per-editing-session cache in `app.js` mapping image reference → object URL; reattach cached URLs on re-render without fetching; verify via a stubbed `fetch` (Node test or manual devtools) that typing 50 characters causes one request per image
- [ ] 3.2 Revoke object URLs for images no longer referenced after a render, and revoke everything on editor close (`closeEditor`); verify no `blob:` URLs accumulate across a type-close-reopen session
- [ ] 3.3 Keep the broken-reference indication working when the reference misses the API on first load; verify the existing "missing image is visible" behavior manually

## 4. Single in-flight commit

- [ ] 4.1 Disable `#commit-confirm` on activation and re-enable it when the commit request settles (success or failure report); verify double-clicking "Stage all and commit" against a dirty tree produces exactly one `git add -A` (visible in the report output) and `go test ./internal/vcs` still passes

## 5. Integration

- [ ] 5.1 Run `node --test "tests/**/*.test.mjs"` and `go test ./...`; both pass
