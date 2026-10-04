# Design

## Context

The editor knows the post's slug (pinned frontmatter, surfaced in
`handleEntry`'s `frontmatter` map) and the page cannot probe
`http://localhost:4321` itself: the interface is served from
`http://127.0.0.1:<port>`, so a reachability fetch from the browser is
cross-origin and unreliable without the dev server's CORS cooperation.
See proposal.md → Why.

## Goals / Non-Goals

**Goals**

- One editor action → new tab with the post's real URL; honest failure when
  the dev server is down.

**Non-goals**

- No spawning/managing `astro dev`; no WebSocket-driven live bridge; no
  route-pattern inference beyond base+slug (see Risks).

## Decisions

- **Server-side reachability probe.** `GET /api/collections/{name}/dev-url?slug=<s>`
  returns `{ url, reachable }`; reachability is a 1.5s `HEAD` (fallback
  `GET`) against the base URL from the Go process, where no CORS exists.
  The client then `window.open`s the URL itself — browser popup rules are
  satisfied because the click gesture is preserved through the probe
  (window.open is called in the same task chain after `await`).
- **URL shape is `<base>/<slug>/`.** Correct for collections routed as
  `[...id].astro` (the overwhelmingly common pattern) and for the tool's
  own pinned-slug guarantee. Blogs routing posts under a subpath set
  `devUrl` accordingly (e.g. `http://localhost:4321/posts`) — the config
  value already being a base URL makes this a zero-code escape hatch.
- **Config reuses the existing per-project file**: `devUrl` string field on
  `config.Project`, resolved in `serve.go` into `server.App`, surfaced by
  the new route. No CLI flags.
- **Failure UX is a dialog, not a silent open.** Unreachable → offer
  "Open anyway" / "Copy URL" / Cancel, because dev servers are often
  merely slow to boot, and an unreachable-then-loaded tab is common.

## Risks / Trade-offs

- [Blogs with url-rewriting routes (`[...slug]` transforms, `generateId`)
  produce URLs the base+slug shape doesn't match] → documented limitation;
  the `devUrl` base covers the subpath case, and full route-template support
  is future work noted in the README's fidelity section.
- [Probe adds latency to the click] → 1.5s cap, and "Open anyway" remains
  one click.

## Migration Plan

Additive route + config field; absent config yields the Astro default. No
migration.
