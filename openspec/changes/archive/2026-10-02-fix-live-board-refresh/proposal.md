# Proposal

## Why

A review found that the board's 2-second poll rebuilds every column with
`innerHTML = ""` on each tick. That rebuild destroys keyboard focus (the
README advertises tabbing to a card and pressing Enter), closes an open
"Move to…" dropdown mid-interaction, resets column scroll, and breaks an
in-flight drag — so the board's keyboard accessibility is unusable in
practice. Meanwhile the finished, tested filesystem watcher
(`internal/watch`) is started on every run and its events are never
consumed: the change feed the poll approximates already exists and goes
nowhere.

## What Changes

- Expose the watcher's debounced change feed to the interface as a
  server-sent events stream on the local server, behind the same session
  token and Host checks as the rest of the API.
- Replace the board's fixed 2-second full re-render with event-driven
  refresh: the board re-reads the listing when the feed reports a change,
  with a slow fallback poll for environments where watching is
  unavailable.
- Never rebuild the board DOM when the data has not changed, and defer a
  rebuild while the user is interacting with it (focus inside the board,
  an open move dropdown, an active drag), applying it immediately after.
- Refresh the funnel data each time the funnel panel is opened, so
  figures never go stale within a session.

## Capabilities

### New Capabilities

### Modified Capabilities

- `board`: "Board tracks the filesystem" gains event-driven refresh
  semantics, and the board gains a requirement that refreshes preserve
  the user's interaction context.
- `local-server`: gains a requirement describing the advisory change
  feed the interface subscribes to.

## Impact

- `internal/server` (new SSE route, event fan-in from `watch`),
  `serve.go` (wire watcher events into the server), `internal/watch`
  (consumed for the first time; latent `rootFor` containment gap fixed
  before the feed drives behavior), `internal/server/web/app.js`
  (event-driven refresh, render preservation), `tests/` and Go test
  suites.
- No new dependencies (`text/event-stream` is writable with the standard
  library). No change to the wire format of existing endpoints; the
  client tolerates the feed being absent.
