# astrogui

A local cockpit for an [Astro](https://astro.build) blog: a kanban board over
your drafts, a split-pane markdown editor, and a publish step that moves a post
into your content collection by a single atomic directory rename.

astrogui is **not** a dependency of your blog. It is a standalone binary you
run from inside the project. A blog that has never run astrogui builds and
runs identically.

## Install

astrogui is a prebuilt Go binary distributed through npm. Installing never
compiles anything on your machine:

```sh
npm install -g astrogui
```

The npm package carries a small JavaScript shim plus per-platform optional
packages (`@astrogui/darwin-arm64`, `@astrogui/linux-x64`, …), so the same
install works on macOS, Linux, and Windows, on amd64 and arm64. You can also
run it once without installing:

```sh
npx astrogui
```

If your platform has no prebuilt binary, the install reports that clearly
instead of failing obscurely; you can point the shim at a binary you built
yourself with the `ASTROGUI_BIN` environment variable.

## Use

From anywhere inside your Astro project:

```sh
astrogui
```

That resolves the project, opens the board in your browser, and listens on
loopback only. Other invocations:

```sh
astrogui version    # print the version
astrogui --help     # usage
```

Requirements: an Astro project with a content collection defined over a
directory using the `glob()` loader.

## The on-disk convention

The board is three directories. Every post is a **folder** holding `index.md`
plus the post's own images:

```
drafts/ideas/my-post/index.md      captured, not yet written
drafts/wip/my-post/index.md        in progress
src/content/blog/my-post/index.md  published — inside your collection
```

- **Lifecycle state is the directory.** Moving a card is a single atomic
  `rename` of the post's folder. There is no status field in your frontmatter,
  and the state is readable with any tool, with or without astrogui.
- **Every post astrogui creates carries `slug: <post-name>`**, pinned at
  creation. Astro's content layer lets a `slug` frontmatter property override
  the generated entry id, so the published URL depends only on the post's
  identity — moving between draft directories can never change it. The pin
  holds even when your collection schema does not declare `slug` (unknown
  keys are stripped from entry data, not rejected); a custom `generateId`
  that ignores frontmatter is outside the tool's control.
- **Images live in the post's folder** and are referenced relatively
  (`![cover](cover.jpeg)`), so references survive the publish move untouched.
- **Posts you did not create are never restructured.** Loose `.md` files in
  the content directory are listed read-only (dragging and structured edits
  are disabled; the raw text view still works) and are never migrated to the
  folder layout.

Directory names default to `drafts/ideas` and `drafts/wip` and are
configurable, as is the content directory override and the staleness
threshold, in the tool's own config at
`~/.config/astrogui/config.json` (or your platform's equivalent). The tool's
derived cache (`cache.json`, first-seen timestamps and the funnel's recorded
transitions) lives beside it. Both are disposable: delete them and the board
rebuilds from the filesystem.

## The operating boundary

> **Your content, and the commands you already run.**

- astrogui writes **only** inside the managed directories (the two draft
  directories and the resolved content directory) — every write passes one
  containment check, resolved through symlinks. Apart from that it writes
  only its own config and derived cache, outside the project.
- It never touches `astro.config.*`, `content.config.ts`, `package.json`,
  lockfiles, or `.env`.
- It is not a dependency of your blog: installing or running it changes no
  manifest, and a project that has never run it builds identically.
- Version control stays yours: a move performs **no** git operation. The
  commit and commit-and-push buttons run exactly `git add -A`, `git commit`,
  `git push` in your repository — the commands you already run yourself —
  and a failure is reported with its full output, never as success.
- A move that cannot be atomic is refused: if the draft and content
  directories sit on different filesystems, astrogui names both locations
  and leaves the post untouched rather than performing a non-atomic copy.

## The publish gate

Moving a post into the content directory is the moment it enters your build,
so astrogui checks it first. A failed check blocks the move and names the
specific problem:

- every referenced image resolves **inside the post's own directory** —
  a post referencing a shared asset elsewhere fails with the reference named;
  that is the price of the folder layout, and it fails here rather than in
  your build;
- a frontmatter `title` is present;
- the `date` parses and is not in the future;
- the body is not empty.

**Limits:** these checks are heuristic. astrogui does not read your
collection schema (it is TypeScript that imports `astro:content`; executing
it is out of the question), so a required field your schema adds beyond
title/date surfaces as a build error after publishing — exactly as it would
without the tool. The tool does not pretend to be a validator.

## The preview's fidelity ceiling

The editor is a split pane: the left side is the file's bytes, the right side
is a **reading copy** rendered in the browser. The reading copy conveys
content, not appearance: expect your post's structure (headings, tables, code
fences and their metadata, footnotes, images), and do not expect the
published site's styling, Shiki highlighting, or optimized images — the
blog's own dev server is the fidelity tool. Nothing you type is normalized:
saving stores your bytes exactly, and an unchanged file is never rewritten.

## Development

```sh
go build ./...     # build everything
go test ./...      # run the test suite
go run . --help    # CLI help
```

The npm package lives in [`npm/`](npm/); its shim execs the platform binary,
falling back to a build at `npm/bin/astrogui` for local testing:

```sh
go build -o npm/bin/astrogui . && node npm/index.js version
```
