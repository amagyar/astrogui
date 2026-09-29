# Design

## Context

astrogui is greenfield. The repository contains only the OpenSpec root — no
source, no history, no existing conventions to preserve. Every architectural
choice here is a first choice rather than a migration.

Two facts about Astro shape the whole design, and both were verified against the
Astro documentation during discovery:

1. **Astro has no notion of an unpublished post.** The `glob()` loader loads
   every file matching its pattern, the collection schema validates shape, and
   the *blog's own* route code decides what gets published. A `status: draft`
   field in frontmatter has no effect unless the blog already filters on it.
   File location is therefore the only publication gate a tool can use without
   modifying the blog.

2. **`image()` resolves frontmatter paths relative to the entry's own file**, and
   an unresolvable path fails schema validation and breaks the build rather than
   degrading quietly. A post that moves to a different directory with a relative
   image reference will break unless its assets move with it.

The user has previously shipped a Go CLI distributed through both npm and
Homebrew, so that pipeline is a known quantity rather than a new risk.

## Goals / Non-Goals

**Goals:**

- The tool is a standalone binary. A blog that has never run astrogui builds and
  runs identically.
- Content the tool does not intend to change is never altered. Corruption is the
  one failure mode that would end trust in the product.
- Every risky operation is pushed to the single place it cannot cause harm: an
  atomic filesystem move.
- The boundary is expressible in one sentence, so it can be extended later
  without re-litigating cases.

**Non-Goals:**

- Rendering a faithful preview of the published site. The blog's own dev server
  is the fidelity tool; astrogui's preview is a reading copy.
- Reading the project's collection schema. `content.config.ts` imports
  `astro:content` and is TypeScript, so it cannot be imported or reliably
  parsed. Pre-flight checks are heuristic by necessity.
- Any write to framework configuration, dependency manifests, or environment
  files.
- Multi-author editing, merge conflict resolution, or concurrent access safety.
  This is a single-author tool; pretending otherwise would add cost without a
  user.
- Migrating a project's existing loose markdown files into the folder layout.

## Decisions

### Lifecycle state is the directory, not a frontmatter field

`drafts/ideas/`, `drafts/wip/`, and the collection's content directory. A
transition is `fs.rename`.

**Alternative considered — a `status` frontmatter field.** Rejected on two
grounds. First, it does not work: Astro ignores it, and making it work requires
editing the blog's route code, which violates the boundary. Second, it requires
the YAML round trip that the directory move avoids, and that round trip is the
only mechanism in this design capable of corrupting a post.

**Alternative considered — store state in a tool-owned sidecar file.** Rejected
as *state*, because it creates a second source of truth that can drift from the
filesystem after a hand edit or a merge. It is used only for *derived* data, where
drift is harmless because the value is recomputable. Derived data covers
first-seen timestamps and observed state transitions — both disposable, so
progression history that predates the tool is not claimed.

The consequence worth stating: because the board is a directory view, it is
correct by construction and survives restart without a load step.

### A post is a folder, and carries a pinned slug

```
  drafts/wip/async-rust/
    index.md
    cover.jpeg
    diagram.png
```

Chosen so a publish is one rename of one directory and every relative image
reference stays valid. Storing loose files would require rewriting image paths
during the move — a regex over the author's prose, which is precisely the
operation most likely to corrupt a post.

The follow-on risk is that Astro derives an entry's id from its file path, so a
nested `index.md` would produce a different id than the flat filename did. The
mitigation is `slug`: the content-layer documentation ("Defining custom IDs")
states that a `slug` frontmatter property overrides the generated entry id, and
`generateId()` receives the frontmatter before schema validation, so the pin
holds even where the schema does not declare the field. Every post astrogui
creates carries `slug: <post-name>`, so the published URL depends only on the
post's identity and is unchanged by any move. This is what makes "move does not
change the URL" true rather than merely intended.

Unrecognised keys are stripped by Zod rather than rejected, so a schema that
does not declare `slug` neither breaks the build nor surfaces the field in
entry data — the id pin still applies.

**Alternative considered — a single shared assets directory.** Fewer
directories, but it imposes a convention on the blog and makes the publish move
depend on assets that may be shared between posts.

### A move that cannot be atomic is refused

An `fs.rename` is atomic within one filesystem. Across filesystems — a draft
directory on an external volume, a content directory on another mount — the
operation degenerates into copy-then-delete, which has no atomic form. Rather
than implement a transactional copy, astrogui refuses the move and names both
locations. The guarantee that a move either fully succeeds or leaves both
locations unchanged is preserved by never attempting the non-atomic path.
Cross-volume layouts are rare for a single-author blog, and a user in that
position can complete the move by hand exactly as they would without the tool.

**Alternative considered — copy with rollback.** Rejected: it widens the
surface of the one operation the whole design depends on staying trivial, to
serve a layout the tool does not recommend.

### The editor has no document round trip

The left pane is a textarea over the file's bytes. There is no parse, no
document model, no serialize. The right pane renders a copy for reading, in the
browser.

This is the decision that makes the no-corruption guarantee real for authoring,
not just for moving. A WYSIWYG editor is a round trip, and a round trip
normalizes: Shiki code-fence metadata is the concrete casualty, but trailing
spaces, reference-style link definitions, and comment syntaxes are all at risk.

Structured frontmatter fields are edited through a form because the board needs
a title and a date to render cards, and because requiring YAML to capture a
one-line idea is a bad trade. This is the one place the round trip exists, so it
is bounded three ways: a YAML library with a document/CST API that preserves
comments and key order (in Go, `goccy/go-yaml` is the candidate;
`gopkg.in/yaml.v3` does not round-trip comments reliably enough to promise
this), a write only when a field actually changed, and a raw text view so any
failure is recoverable by hand. Ideas skip the form entirely and are dated from
the filesystem.

Saving is guarded against concurrent modification: a save against a file that
changed on disk since the editor opened it is refused rather than applied, so
the editor can never silently discard an edit made in another tool.

### Preview is client-side

The preview pane renders in the browser, which removes any need for a markdown
renderer inside the binary. The binary's entire job is file I/O, process
watching, and git.

This is what makes the Go choice pay off: the binary compiles to something small
and starts instantly, with no dependency resolution and no module errors inside
someone else's blog. A Node CLI would put the tool's dependency tree next to the
blog's, in the one directory that must stay clean.

The preview is content-accurate and style-oblivious by design. It is documented
as a reading copy so its limits are not later mistaken for bugs.

### Pre-flight checks gate the publish move

The move into the collection directory is the moment of no return — the post
enters the build, and a missing image fails the whole build rather than one
page. So the check runs *before* the move, and a failure leaves the post
untouched with a message naming the specific problem.

Checks are heuristic (see Non-Goals): every referenced image resolves inside the
post directory, a title is present, the date parses and is not in the future, the
body is non-empty. A post that references an image on a path outside its own
directory fails — that is the price of the folder layout, and it is the right
trade.

### The binary is Go, distributed through npm and Homebrew

The user has shipped this shape before, and the server is mostly file I/O and
process management, where Go's standard library is sufficient and its build
produces a single static file with no runtime dependency.

npm has no native binary concept, so the package follows the esbuild/Keystatic
pattern: a small JavaScript shim plus per-platform optional packages
(`@astrogui/darwin-arm64`, `@astrogui/linux-x64`, and so on). This preserves
`npx astrogui` alongside the global install. A Homebrew formula is added later
against the same release artifacts.

The HTTP surface is shaped as `/api/collections/:name/entries` rather than
`/api/posts`. The tool is called a manager for Astro, and a manager that
hardcodes `blog` is wrong the first time someone has a second collection. The
cost is naming the collection in more URLs; the benefit is not having to
redesign the wire format to grow.

### The boundary rule is consent-based

> **Your content, and the commands you already run.**

The test for any future capability is whether the user already performs that
operation themselves. `git push`, `astro build`, and anything in the project's
`scripts` pass. Editing `astro.config`, touching `.env`, or writing outside the
managed directories does not.

This is preferred over a risk-based enumeration because it needs no maintained
list and no judgment calls: the rule adjudicates its own edge cases. Content
management remains the only capability that writes files, and it writes only
inside the managed directories.

### Local server hardening

The server reads and writes the user's files over HTTP, which makes it a known
vulnerability class rather than a hypothetical one — Vite has shipped multiple
dev-server CVEs of this shape, most recently bypassing `server.fs.deny`.

Five controls, all cheap:

- Bind `127.0.0.1` only, so the port is unreachable from the network.
- Validate the `Host` header against the expected origin, which is what defeats
  DNS rebinding and the single most commonly skipped check.
- Mint a random token at startup, carry it in the URL fragment (not the query
  string, so it is not leaked in `Referer`), and require it on every API call.
  A cross-origin page cannot read the fragment.
- Resolve and confine asset paths by real path, after symlink resolution, against
  the post's own directory.
- Treat post content as untrusted inside the tool's own page: the preview
  neutralizes script elements and event-handler attributes, backed by a
  Content-Security-Policy that forbids inline script. Without this, a hostile
  draft pasted from the web is script executing at the tool's origin, able to
  read the session token from the fragment — the same asset the four network
  controls exist to protect.

The host check and the token are belt and braces: either alone would be
defensible, and together they cost on the order of a day.

## Risks / Trade-offs

- **A blog whose posts are loose files cannot be managed without migration.**
  → Published posts are read-only on the board; editing is offered in the raw
  text view. Only drafts created by astrogui use the folder layout. This is
  recorded as a permanent scope boundary rather than deferred work.

- **The folder layout breaks the common habit of sharing images between posts.**
  → Accepted. A post that references an asset outside its own directory fails the
  pre-flight check with a specific message rather than failing the build later.
  This is the correct failure ordering.

- **`slug` pins URLs, but a project with a custom `generateId` may behave
  differently, since a custom function may ignore the slug.** → The docs
  guarantee the override only for the default id generation. Validate against a
  representative fixture during implementation rather than assuming.

- **The heuristic pre-flight check cannot know what a given blog's schema
  actually requires.** → Accepted. A missing required field surfaces as a build
  error after publishing, exactly as it would without the tool. The checks cover
  the failures that are common and cheap to detect; the tool does not pretend to
  be a validator.

- **The npm distribution needs a CI matrix across platforms and architectures.**
  → Mitigated by the user having done this before. Start with the platforms that
  matter and add the rest; the shim reports clearly when no prebuilt binary
  matches.

- **The tool holds the whole post in memory as text.** → Non-issue at blog
  scale; posts are kilobytes. Noted only so it is a known property rather than a
  surprise.
- **Windows is in the distribution matrix but not in the daily loop.** → Rename
  semantics on Windows can fail where POSIX succeeds (open handles, antivirus
  locks), watcher behavior differs, and path containment must be tested against
  backslash and drive-letter forms. The CI matrix includes a Windows runner from
  the first release rather than as a retrofit.
