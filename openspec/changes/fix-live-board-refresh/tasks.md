# Tasks

## 1. Fix the watch feed before it becomes load-bearing

- [ ] 1.1 Fix `watch.rootFor` containment to reject `"../sibling"` paths (`strings.HasPrefix(rel, ".."+separator)`), matching `safe.containsPath`; add a regression test in `internal/watch/watch_test.go` that a sibling path resolves to no root, and verify `go test ./internal/watch` passes
- [ ] 1.2 Update `README.md` ("The watcher keeps the board live") if its description of the mechanism changes; verify the text matches the new behavior after group 3 lands

## 2. Server: SSE change feed

- [ ] 2.1 Add `GET /api/collections/{name}/events` to `internal/server/server.go`: accept the session token from the `token` query parameter (GET-only, EventSource cannot set headers), keep the existing Host-header and collection guards; verify a request without a token returns 401 and a wrong Host returns 403 in `server_test.go`
- [ ] 2.2 Wire one fan-in goroutine draining `watcher.Events()` (started in `serve.go`, plumbed through `server.App`) that broadcasts a `data: changed` tick to each connected SSE client with non-blocking sends; verify with a test that a file created in a watched directory reaches a connected SSE client
- [ ] 2.3 Add a 25s `: ping` heartbeat and client cleanup on write error; verify `go test ./internal/server` covers a client disconnect not blocking other clients
- [ ] 2.4 Extend the "watching unavailable" stderr path so the feed-open failure is visible the same way; verify manually that `astrogui` still serves and polls when watch setup fails

## 3. Client: event-driven, context-preserving refresh

- [ ] 3.1 In `internal/server/web/app.js`, replace the fixed 2s `setInterval` with an `EventSource` subscription that calls `refreshBoard()` on each tick, with a 30s fallback poll when EventSource errors repeatedly or is unavailable; verify the fallback engages by loading the board with the feed blocked
- [ ] 3.2 Fingerprint the board response (per card: name, state, modTime, size, stalled, readOnly) and skip `renderBoard` when unchanged; verify via a Node test or manual check that focus on a card survives an unchanged refresh
- [ ] 3.3 Defer a pending render while focus is inside `#board`, while a card's move select is open, or during an active drag; retry on next tick or a 1s timer; verify keyboard tab-through and an open "Move to…" dropdown survive an external `touch` of a post file
- [ ] 3.4 Make the funnel panel refetch on every open (drop the session-cached `state.funnel` reuse); verify counts update after a move without reloading the page
- [ ] 3.5 Update the source-shape assertions in `tests/ui-contract.test.mjs` that reference the removed 2s interval, and add contract checks for the EventSource wiring and deferral guard; verify `node --test "tests/**/*.test.mjs"` passes

## 4. Integration

- [ ] 4.1 Run the full suites: `go test ./...` and `node --test "tests/**/*.test.mjs"` both pass
- [ ] 4.2 Manual smoke against `testdata/e2e-blog`: board open, `touch drafts/ideas/x/index.md` from another terminal, verify the card updates without reload and without stealing focus from a tabbed card
