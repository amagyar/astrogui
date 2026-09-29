# Proposal

## Why

Writing a blog in an Astro project means managing unpublished work — ideas,
half-written drafts, stalled drafts — with no good tool. The content lives in
markdown files whose state the framework has no concept of, so the only options
today are a terminal, a file manager, or a hosted git-based CMS.

The hosted CMSs (Spinal, Keystatic, Decap, TinaCMS) all solve this by
*entering the blog*: an integration installed into the project, a GitHub App
connected to the repo, or a hosted service. They are built for teams editing a
shared repository, where the branch is the review unit. A single author with
forty local, uncommitted, half-formed ideas needs the opposite — a local cockpit
that shows the whole pipeline at a glance and can never damage the writing.

## What Changes

- Add `astrogui`, a locally-run management tool for an Astro project. It is
  installed globally and run inside a blog directory; it is **not** a dependency
  of the blog and makes no change to the blog's build.
- Represent the post lifecycle as a directory state machine
  (`ideas` -> `in progress` -> published). Board columns are directories, and
  every transition is a single `fs.rename` of a post folder.
- Represent a post as a folder (`index.md` plus its images) so that relative
  image references survive a publish move untouched.
- Add a kanban board over those directories, carrying the derived metadata a
  filesystem cannot show: age, stalled drafts, growth, idea-to-post funnel.
- Add a split-pane markdown editor. The left pane is the file verbatim (no
  parse/serialize round trip); the right pane is a rendered reading copy.
- Gate the publish move behind a pre-flight check that verifies the post can
  survive being seen by the blog's build.
- Ship a Go binary distributed through npm (per-platform optional packages);
  Homebrew follows as a separate change against the same release artifacts.
- Establish the operating boundary: **your content, and the commands you already
  run.** astrogui writes inside the project only in the user's own content
  directories — apart from its own settings and derived cache, kept outside the
  project — and otherwise only shells out to commands the user already invokes
  themselves.

## Capabilities

### New Capabilities

- `project-integration`: Locating the Astro project and its content
  collections, and enforcing the write boundary that keeps the tool out of
  everything the user did not ask it to touch.
- `post-lifecycle`: The directory state machine that tracks a post from idea to
  published, including the publish gate that must pass before a post becomes
  visible to the blog's build.
- `board`: The kanban view over post directories, and the derived metadata that
  gives a directory-backed board its value over a file listing.
- `post-editing`: Authoring and editing a post without risking corruption of the
  author's content, including frontmatter handling and image management.
- `local-server`: The Go binary that hosts the tool, its HTTP API surface, the
  security controls protecting a file-writing server on localhost, and
  distribution of the binary.

### Modified Capabilities

None. This is the first change in the project; there are no existing specs.

## Impact

- **New project.** `astrogui` is a greenfield Go module with an embedded web UI.
  The repository currently contains only the OpenSpec root.
- **No changes to any Astro project.** The tool is a standalone binary. It does
  not require an integration, does not modify `astro.config.*`, does not modify
  `content.config.ts`, and does not appear in the blog's dependencies. A blog
  that never runs astrogui is unaffected.
- **Requires an Astro blog with a content collection** using the `glob()` loader
  over a directory. Blogs without content collections are out of scope.
- **User-visible filesystem convention.** Posts managed by astrogui are folders,
  not loose `.md` files. Posts the tool did not create are never restructured:
  loose files are listed, edited as raw text only, and never migrated to the
  folder layout.
- **Distribution surface.** npm global install across darwin/linux/win on amd64
  and arm64. The Homebrew formula is deferred to a later change.
