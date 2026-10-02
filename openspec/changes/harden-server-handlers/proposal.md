# Proposal

## Why

A review found several server-handler weaknesses. Four write handlers
(`handleSaveBody`, `handleSaveFrontmatter` ×2, `handleSaveRaw`) call
`os.Stat` after a write, discard the error, and dereference the possibly-nil
result — a post deleted or made unreadable between write and stat panics the
handler. Entry read failures (I/O errors from `findPost`) are reported as
404 "no post", masking real errors as absence. And the asset GET endpoint
confines to the post's directory only for folder posts: for loose files
`p.Dir` is the whole collection directory, so the endpoint serves any file
in the collection — flatly contradicting the endpoint's documented control
("confined by real path … to the post's own directory") and inconsistent
with asset upload, which refuses loose posts outright. Finally, SVG assets
are served as `image/svg+xml` with no sandboxing header, so direct
navigation to an asset URL runs an active document in the interface's own
origin.

## What Changes

- Guard every post-write `os.Stat`: a failed stat after a successful save
  reports success with a null modification time instead of panicking.
- Distinguish entry lookup errors: a post that does not exist is 404, an
  existing post that cannot be read is 500 with the read error named.
- Refuse asset GET for loose posts, matching the upload endpoint's refusal;
  loose files expose no asset namespace.
- Serve assets with a `Content-Security-Policy: default-src 'none'` (and
  `X-Content-Type-Options: nosniff`) response header, so even a directly
  navigated SVG cannot execute in the interface's origin.
- Correct the asset handler's doc comment to match behavior.

## Capabilities

### New Capabilities

### Modified Capabilities

- `local-server`: "Requests are confined to the project" gains scenarios
  covering loose files and non-executable asset responses; gains an
  error-honesty requirement for entry lookups.

## Impact

- `internal/server/server.go` (four handlers, asset handler, error
  mapping), `internal/server/server_test.go` and `editor_test.go`
  (regression tests), `internal/server/web/app.js` (loose posts already
  block local-image hydration client-side; verify the placeholder path is
  used). No API shape changes; the loose-asset path is documented as
  refused.
