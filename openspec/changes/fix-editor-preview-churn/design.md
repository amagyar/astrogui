# Design

## Context

`app.js` calls `updatePreview()` directly from the source textarea's `input`
event; `updatePreview` replaces `#preview`'s `innerHTML` and re-fetches every
`img[data-src]` through the assets API, minting a fresh `blob:` URL per image
per keystroke with no `revokeObjectURL` anywhere. See proposal.md → Why.

## Goals / Non-Goals

**Goals**

- Bound preview work per edit, cache image hydration per editing session,
  preserve preview scroll, literal code spans, single in-flight commit.

**Non-goals**

- No virtualized/CodeMirror-style editor; the textarea stays.
- No diffing renderer: the preview pane is still rebuilt per debounced
  update, which is cheap once images are cached.
- No change to the sanitization model; the renderer remains escape-first.

## Decisions

- **Debounce at 200ms trailing edge.** Typing feels live, bursts collapse to
  one render. Alternative (render every keystroke but skip unchanged
  segments) adds a diffing layer for no user-visible gain.
- **Image hydration cache keyed by post name + reference in module state.**
  On each render, cached object URLs are reattached to the new `img`
  elements without a fetch; URLs absent from the latest render or from a
  closed editor are revoked. Alternative (convert once to `data:` and embed)
  bloats every render string and hits the CSP the same way.
- **Scroll preservation by scrollTop snapshot.** Save/restore around the
  innerHTML swap; good enough for a reading pane.
- **Code spans tokenized first, held as placeholders.** `inline()` extracts
  `` `...` `` spans into opaque tokens, runs image/link/emphasis passes on
  the remainder, then substitutes the escaped code content back. This fixes
  the class of bug (any future inline rule also can't see inside code)
  rather than the one observed instance.
- **Commit confirm disables on click, re-enables on settle.** Both success
  and failure paths end at the report dialog, which is the re-enable point.

## Risks / Trade-offs

- [Placeholder substitution could be spoofed by post text containing the
  token pattern] → tokens use a NUL-containing sentinel
  (`\u0000CB<i>\u0000`), which markdown source cannot contain (the renderer
  normalizes input and NUL is not producible by typing; escapeHTML leaves it
  intact).
- [200ms lag on huge posts] → acceptable for a reading copy; the debounce is
  trailing-edge so the first keystroke of a burst still feels immediate.

## Migration Plan

Client-only; no data migration. Roll back by revert.
