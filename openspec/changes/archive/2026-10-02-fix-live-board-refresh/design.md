# Design

## Context

`internal/watch` already produces a debounced (`60ms`), advisory change feed;
`serve.go` starts it and immediately discards the events. The client polls
`GET /api/collections/{name}/board` every 2s and rebuilds the board with
`innerHTML = ""` per response. See proposal.md → Why for the user-facing
damage this causes.

## Goals / Non-Goals

**Goals**

- Push change notification over SSE (`GET /api/collections/{name}/events`);
  the payload is only "something changed", the client re-reads the board API.
- Client renders only when the listing actually changed, and defers rendering
  while the board is being interacted with.
- Remove the fixed 2s interval; keep a slow fallback poll.
- Fix the latent `watch.rootFor` containment gap (`rel != ".."` does not
  reject `"../sibling"`) before the feed drives client behavior.

**Non-goals**

- No WebSocket, no third-party SSE library, no event payloads carrying post
  data (advisory by design; the listing API remains the source of truth).
- No change to the editor's polling behavior while open (already excluded).
- Not removing `internal/watch` — it becomes load-bearing here.

## Decisions

- **SSE over WebSocket / long-poll.** One-directional, text protocol writable
  with `net/http` + `http.Flusher` alone; browser `EventSource` gives
  reconnect for free. It rides through the existing token check: EventSource
  cannot set headers, so the token is accepted from the `token` query
  parameter for this GET-only endpoint (equivalent exposure to today's
  fragment-in-URL model; fragment is not sent to servers, query is — but the
  server is loopback-only with Host validation, so the token never leaves the
  machine, and the endpoint must not appear in logs. `net/http` does not log
  requests by default). Alternative considered: keep header-only and fall
  back to poll — rejected, it would silently disable the primary mechanism.
- **Server fans in the watcher once.** One subscription goroutine per server
  drains `watcher.Events()` and broadcasts to connected SSE clients
  (slice of `chan struct{}`); a client channel that is full is dropped from
  that tick — clients re-read on the next event anyway.
- **Fingerprint-gated render.** The board handler response already contains
  everything needed; the client hashes (name, state, mtime, size, stalled,
  readonly) per card and skips `renderBoard` when identical to the previous
  fingerprint. Changes render through the existing full rebuild.
- **Deferral rule.** Skip a pending render while
  `document.activeElement` is inside `#board`, while a `.move-control
  select:focus-visible`/open dropdown exists, or during drag (`dragstart`
  sets a flag until `dragend`); retry on the next event or a 1s timer.
- **Fallback poll.** 30s interval when EventSource errors repeatedly or is
  unsupported; the stderr "watching unavailable" message already exists
  server-side and is extended into the feed-open failure path.
- **Funnel freshness.** The funnel panel refetches on every open instead of
  caching `state.funnel` for the session (restores compliance with the
  existing "counts come from the directories" scenario).

## Risks / Trade-offs

- [EventSource without custom headers needs the token in the query string] →
  endpoint stays GET-only, side-effect-free, same Host + token gate; token is
  per-run and never logged by the server.
- [fsnotify watch limits on huge trees] → the watcher already degrades
  gracefully; client falls back to the 30s poll, no functional loss.
- [SSE connection held per tab; server has no shutdown signaling today] →
  responses flush with heartbeats (`: ping` every 25s) so dead connections
  surface as write errors and are dropped.

## Migration Plan

Purely additive endpoint + client change; roll back by reverting the client
to its poll (the fallback path remains in code permanently).
