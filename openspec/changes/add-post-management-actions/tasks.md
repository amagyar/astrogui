# Tasks

## 1. Lifecycle operations

- [ ] 1.1 Add `lifecycle.Manager.Rename(name, state, newName)`: guard-check both paths, refuse loose posts and taken names, atomic rename via the manager's `rename` seam, then update the pinned `slug` through `fmedit.Update` (non-rollback on edit failure); verify `lifecycle_test.go` covers rename, taken-name refusal, loose refusal, and slug update, with `go test ./internal/lifecycle` passing
- [ ] 1.2 Add `lifecycle.Manager.Discard(name, state)`: guard-check, refuse loose, move to the trash directory with `UniqueName` suffixing on collision; verify tests for move, collision suffix, and loose refusal
- [ ] 1.3 Add `trashDir` to `internal/config` (`Project`, `FillDefaults`, resolver mirroring `IdeasPath`), default `drafts/trash`; verify `config_test.go` covers default and override

## 2. Server and wiring

- [ ] 2.1 Wire the trash directory as a fourth `safe.New` root in `serve.go`; verify `go build ./...` and that a fresh run creates `drafts/trash`
- [ ] 2.2 Add `POST /api/collections/{name}/entries/{post}/rename` and `POST …/discard` routes to `internal/server/server.go` with the same token/collection guards and the conflict/refusal error mapping used by `move`; verify `server_test.go` covers success, taken-name 409, and loose 403 for both routes

## 3. Client

- [ ] 3.1 Add a `+` new-post control on the ideas and wip column headers (not published) opening a dialog with a title field that calls the existing create endpoint with that state; verify the card appears without reload and keyboard focus lands in the title field
- [ ] 3.2 Add card-level Rename and Discard controls (keyboard-operable, `sr-only` labels like the move control, hidden/disabled for read-only cards); Rename prompts for the new name; Discard confirms with copy naming the trash location; verify both refresh the board after success
- [ ] 3.3 Published-rename path: confirmation names the changed public URL before calling rename; verify the confirmation text includes the new name
- [ ] 3.4 Update `tests/ui-contract.test.mjs` for the new controls and add `editor-state`-style pure-logic tests where logic was extracted; verify `node --test "tests/**/*.test.mjs"` passes

## 4. Documentation and integration

- [ ] 4.1 Update `README.md`'s on-disk convention section with the trash directory, the rename/discard actions, and the new-post flow; verify the documented layout matches `config.go` defaults
- [ ] 4.2 Run `go test ./...`, `node --test "tests/**/*.test.mjs"`, and `scripts/e2e.sh`; all pass
