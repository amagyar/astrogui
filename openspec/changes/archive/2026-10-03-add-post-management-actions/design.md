# Design

## Context

Lifecycle transitions are already atomic directory renames gated by
`safe.Guard` (`lifecycle.Manager.Move`); rename and discard are the same
mechanism with different destinations. The create endpoint
(`handleCreate`) already accepts `title`+`body`; only the UI lacks it.
Guard roots are currently `ideas`, `wip`, `content` (`serve.go`) — the
trash directory must join them. See proposal.md → Why.

## Goals / Non-Goals

**Goals**

- Rename: in-state atomic folder rename, slug pin follows the name.
- Discard: atomic move into `drafts/trash`, board never lists trash.
- New post: minimal titled-creation UI on draft columns.

**Non-goals**

- No trash management UI (empty/list/restore) — trash is a plain directory;
  restore is a manual move, matching "state is the directory".
- No permanent-delete button in the tool; the user empties trash outside.
- No renaming of loose files, ever.

## Decisions

- **Two new POST routes** (`…/entries/{post}/rename`, `…/entries/{post}/discard`)
  rather than overloading `move`: discard's destination is outside the
  three-state machine and rename has no state change; separate routes keep
  `Move`'s contract (state machine + publish gate) intact. Reuse
  `Manager`'s guard checks and `rename` seam for `EXDEV` handling.
- **Slug follows name on rename** via the existing `fmedit.Update` path,
  applied after a successful folder rename; if the frontmatter edit fails,
  the rename is NOT rolled back (folder rename already succeeded atomically)
  but the post keeps working — the slug pin only guarantees URL stability,
  and the user just changed the name deliberately. Published-post rename
  requires the confirmation dialog to name the new URL path segment.
- **Trash location `<project>/drafts/trash`**, configurable alongside
  `ideasDir`/`wipDir` in the existing per-project config (`trashDir`,
  default filled by `FillDefaults`). It is guarded like the other roots and
  excluded from `posts.List` by construction — `List` reads only the three
  state directories.
- **Discard collision**: reuse `UniqueName(trash, name)` suffixing.
- **UI**: card footer gains small "Rename" / "Discard" ghost buttons (labels
  in the existing `sr-only` pattern); confirmation reuses the `report`
  dialog pattern with a confirm row instead of a single close button.
  New-post is a `+` control in each draft column header opening a dialog
  with a title field; creation targets that column's state.

## Risks / Trade-offs

- [Slug edit after rename is two writes, not atomic with the move] →
  acceptable: the folder identity changed intentionally; worst case slug and
  folder disagree until next save, which the stable-URL design tolerates
  (slug wins for the URL).
- [Users expect discard ≈ delete] → confirmation copy says "moved to
  drafts/trash; delete the folder there to remove it permanently".

## Migration Plan

On first run after upgrade, `safe.New` creates `drafts/trash` if absent —
the tool already creates its draft directories this way. No data migration.
