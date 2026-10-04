# Proposal

## Why

The editor's preview is deliberately style-oblivious — the README names the
blog's own dev server as "the fidelity tool" — yet reaching it means leaving
the editor, knowing the dev server's port, and constructing the post's URL
by hand. The loop the tool sets out to support (write here, see the real
thing) ends one step short, and that last step is the one authors repeat
most.

## What Changes

- Add an "Open in dev server" action to the editor that opens the post's
  public URL — built from the pinned slug and a configured dev-server base
  URL — in a new browser tab.
- The base URL defaults to Astro's conventional `http://localhost:4321` and
  is configurable per project in the tool's own config (`devUrl`), exactly
  where the draft-directory overrides already live.
- If the dev server is unreachable, the action says so and offers to open
  the URL anyway or to copy it; it never pretends the page loaded.
- astrogui does **not** start or manage `astro dev`; running the dev server
  stays the user's own command, matching the operating boundary.

## Capabilities

### New Capabilities

- `dev-server-preview`: opening the post being edited at its real URL on
  the project's Astro dev server, with the base URL configurable.

### Modified Capabilities

## Impact

- `internal/server` (one read-only route exposing the resolved dev URL or a
  probe result), `internal/config` (`devUrl` per-project field +
  resolution), `internal/server/web` (editor action, reachability message),
  Node and Go test suites, README's fidelity-ceiling section. No writes to
  the project beyond what exists; no new processes are spawned.
