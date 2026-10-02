# Proposal

## Why

A review found two content-model gaps. First, folder-post detection only
recognizes `index.md`: a directory post built on `index.mdx` (fully valid
for an Astro content collection) never appears on the board at all, while
loose `.mdx` files are listed — the tool silently ignores one of the two
file forms it otherwise supports. Second, `Slugify` strips every non-ASCII
character, so a title written in Japanese, Cyrillic, Arabic, Korean, etc.
collapses to `untitled` (and then `untitled-2`, `untitled-3`…), which both
erases the author's wording from the URL and makes captured ideas
indistinguishable on the board.

## What Changes

- Recognize a folder post by either index file (`index.md` or `index.mdx`),
  for listing, finding, moving, and pre-flight checks — everywhere a folder
  post is read.
- Derive post names (and therefore pinned slugs) from letters and numbers
  in any script instead of ASCII only; a title with no usable letters still
  falls back to a unique placeholder.

## Capabilities

### New Capabilities

### Modified Capabilities

- `post-lifecycle`: gains requirements for index-file recognition and for
  script-independent name derivation (the pinned-slug guarantee is
  unchanged).

## Impact

- `internal/posts/posts.go` (folder detection, `Read`), `internal/lifecycle`
  (`FindIn` loose/folder probing, slugify), `internal/project`
  (`hasMarkdown` folder check), tests across those packages plus the
  `testdata/e2e-blog` fixture gains an `index.mdx` folder post. No API or
  config changes; Markdown frontmatter handling already treats the files
  identically.
