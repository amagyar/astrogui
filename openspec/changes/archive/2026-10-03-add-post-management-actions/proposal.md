# Proposal

## Why

Three everyday workflow gaps force the user out of the tool. There is no
way to create a *titled* post from the interface — the API already supports
title+body creation, but the UI exposes only one-line idea capture. A typo
in a folder name (which is also the pinned slug, and after publishing the
public URL) cannot be corrected without a terminal. And a bad or abandoned
draft can never be removed from the board: there is no delete operation at
all, not even from the API.

## What Changes

- Add a "New post" action on the draft columns that asks for a title and
  creates the post through the existing create endpoint.
- Add a rename action for folder posts: an atomic in-state directory rename
  that keeps the pinned slug equal to the name, refuses taken names, and
  warns when renaming a published post (its public URL changes).
- Add a discard action for folder posts: an atomic move of the post's
  folder into a trash directory (`drafts/trash`), which the board does not
  list; recovery is moving the folder back with any tool. Loose files stay
  read-only and are never renamed or discarded.

## Capabilities

### New Capabilities

### Modified Capabilities

- `post-lifecycle`: gains requirements for renaming a post and for
  discarding a post to the trash directory.
- `board`: gains a requirement for titled post creation from the board.

## Impact

- `internal/lifecycle` (Rename, Discard operations reusing the atomic
  rename machinery), `internal/server/server.go` (two routes), `serve.go`
  (trash directory added to the guard roots), `internal/server/web/app.js`
  + `index.html` (column-level New-post control, card-level rename/discard
  controls with confirmation), `internal/config` (optional trash-dir
  override), tests across `lifecycle`, `server`, and the Node suites.
- No changes to existing endpoints' contracts; the trash directory is the
  only new on-disk location, inside the project and documented in the
  README's layout section.
