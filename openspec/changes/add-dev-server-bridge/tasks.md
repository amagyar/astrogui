# Tasks

## 1. Config

- [ ] 1.1 Add `devUrl` to `internal/config.Project` with default `http://localhost:4321` filled in `FillDefaults` (or an `EffectiveDevURL()` accessor mirroring `EffectiveStaleness`); verify `config_test.go` covers the default and an override with port and base path

## 2. Server

- [ ] 2.1 Resolve the dev URL in `serve.go` into `server.App` and add `GET /api/collections/{name}/dev-url?slug=<s>` to `internal/server/server.go` returning `{ url, reachable }` with a 1.5s-capped probe; verify `server_test.go` covers the default URL shape, a configured base with subpath, `reachable:false` when nothing listens, and that the existing token/Host guards apply to the route

## 3. Client

- [ ] 3.1 Add an "Open in dev server" button to the editor header, enabled for every post (folder posts use the pinned slug, loose posts the filename); on click, fetch the dev-url route and `window.open` the URL when reachable; verify manually against `testdata/e2e-blog` with `astro dev` running and stopped
- [ ] 3.2 On `reachable:false`, show a dialog naming the URL with "Open anyway", "Copy URL" (clipboard), and Cancel; verify no success is claimed and both open/copy paths work
- [ ] 3.3 Cover the button and dialog wiring in `tests/ui-contract.test.mjs`; verify `node --test "tests/**/*.test.mjs"` passes

## 4. Documentation and integration

- [ ] 4.1 Update the README's "preview's fidelity ceiling" section with the dev-server action and the `devUrl` config key; verify the documented default matches `config.go`
- [ ] 4.2 Run `go test ./...`, `node --test "tests/**/*.test.mjs"`, and `scripts/e2e.sh`; all pass
