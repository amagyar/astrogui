# Tasks

## 1. Nil-stat guards

- [ ] 1.1 In `internal/server/server.go` `handleSaveBody`, `handleSaveFrontmatter` (both response paths), and `handleSaveRaw`: guard the post-save `os.Stat` error and report `modTime` as null when unavailable; add a `server_test.go` regression that removes the file between save and stat (or simulates it via permissions) and verifies no panic and a 200 with null modTime

## 2. Honest entry lookup errors

- [ ] 2.1 Make every handler's `findPost` path return 404 only for a nil post, and 500 naming the read error when `Manager.Find` failed; add a `server_test.go` case with an unreadable `index.md` verifying the 500 names the read failure, and a missing-name case still returning 404

## 3. Loose-post asset confinement

- [ ] 3.1 Refuse `handleAsset` with 404 for loose posts (mirroring `handleUploadAsset`), update the handler's doc comment, and add a `server_test.go` case that an in-collection file is not served through a loose post's asset path
- [ ] 3.2 Verify the editor preview for loose posts shows the external/broken reference indication rather than fetching: adjust `app.js` hydration to skip local-image fetch for read-only posts and cover it in `tests/ui-contract.test.mjs` or `markdown.test.mjs` as fits

## 4. Inert asset documents

- [ ] 4.1 Set `Content-Security-Policy: default-src 'none'` and `X-Content-Type-Options: nosniff` on all asset responses in `handleAsset`; add a `server_test.go` assertion on both headers for an `.svg` asset

## 5. Integration

- [ ] 5.1 Run `go test ./...` and `node --test "tests/**/*.test.mjs"`; both pass
