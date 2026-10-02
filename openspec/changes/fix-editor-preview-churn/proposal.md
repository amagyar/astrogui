# Proposal

## Why

A review found that the editor re-renders the entire preview on every
keystroke: every image in the post is re-fetched through the API per
keystroke, a new `blob:` object URL is created each time and none are ever
revoked (an unbounded memory leak during a writing session), and the
preview's scroll position resets while the author reads. The markdown
reading-copy renderer also interprets emphasis markers inside backtick code
spans, so code samples display incorrectly. Separately, the commit
confirmation button stays enabled while its request is in flight, so a
double-click can submit the same commit twice.

## What Changes

- Debounce preview updates while typing, and keep the preview's scroll
  position across re-renders.
- Hydrate each local image once per post per editing session: cache the
  fetched object URL by image reference, reuse it across re-renders, and
  revoke object URLs when the editor closes or the image disappears.
- Render inline code spans literally: emphasis markers and images inside
  backticks display as their raw characters, not as formatting.
- Disable the commit confirmation button while a commit request is in
  flight, so one gesture produces at most one command run.

## Capabilities

### New Capabilities

### Modified Capabilities

- `post-editing`: "Rendered preview reflects the body" gains scenarios for
  bounded preview work and literal code spans.
- `post-lifecycle`: "Version control actions are explicit and separate"
  gains a single-submission scenario.

## Impact

- `internal/server/web/app.js` (debounce, image hydration cache, scroll
  preservation, commit button state), `internal/server/web/markdown.js`
  (code-span tokenizer order), `tests/markdown.test.mjs`,
  `tests/ui-contract.test.mjs`, `tests/editor-state.test.mjs` if helpers
  move there. No API or on-disk format changes.
