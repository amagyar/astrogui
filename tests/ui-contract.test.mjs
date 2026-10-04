import { readFileSync } from "node:fs";
import { test } from "node:test";
import assert from "node:assert/strict";

const app = readFileSync(new URL("../internal/server/web/app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("../internal/server/web/index.html", import.meta.url), "utf8");
const css = readFileSync(new URL("../internal/server/web/style.css", import.meta.url), "utf8");

test("board exposes keyboard-operable open and move controls", () => {
  assert.match(app, /open\.type = "button"/);
  assert.match(app, /open\.setAttribute\("aria-label", "Open post:/);
  assert.match(app, /var destination = document\.createElement\("select"\)/);
  assert.match(app, /destination\.addEventListener\("change"/);
  assert.match(app, /movePost\(card\.name, destination\.value\)/);
});

test("Git review and editor recovery have explicit action controls", () => {
  for (const id of ["commit-review", "commit-cancel", "commit-confirm", "conflict-mine", "conflict-disk", "conflict-use-mine", "unsaved-keep", "unsaved-discard", "unsaved-save"]) {
    assert.match(html, new RegExp(`id="${id}"`), id);
  }
});

test("source input rerenders from the textarea rather than treating the event as text", () => {
  assert.match(app, /\$\("source"\)\.addEventListener\("input", function \(\) \{/);
  assert.match(app, /updatePreview\b/);
});

test("preview updates are debounced and preserve the reader's scroll", () => {
  assert.match(app, /clearTimeout\(previewTimer\)/);
  assert.match(app, /setTimeout\(updatePreview, 200\)/);
  assert.match(app, /pane\.scrollTop = scrollTop/);
});

test("image bytes are fetched once per editing session and released when done", () => {
  // The cache is consulted before any fetch, so a re-render reattaches
  // already-fetched bytes without a request.
  assert.match(app, /imageCache\[p\.name\] \|\| \(imageCache\[p\.name\] = \{\}\)/);
  assert.match(app, /if \(cache\[ref\]\) \{/);
  // References that fall out of the post release their object URL.
  assert.match(app, /URL\.revokeObjectURL\(cache\[ref\]\)/);
  // Closing the editor releases every fetched URL for the session.
  assert.match(app, /releaseImageCache\(\)/);
  // A first-load miss still shows the broken-reference indication.
  assert.match(app, /img\.classList\.add\("broken"\)/);
});

test("commit confirmation allows one command sequence at a time", () => {
  assert.match(app, /confirmBtn\.disabled = true/);
  assert.match(app, /confirmBtn\.disabled = false/);
});

test("board refresh is event-driven with a polling fallback", () => {
  assert.match(app, /new EventSource\(/);
  assert.match(app, /addEventListener\("changed", function \(\) \{ refreshBoard\(\)/);
  // The fixed 2s poll is gone; the fallback interval is the slow one.
  assert.match(app, /\}, 30000\)/);
  assert.doesNotMatch(app, /\}, 2000\)/);
});

test("unchanged listings never rebuild the board, and interaction defers a rebuild", () => {
  assert.match(app, /BoardState\.fingerprint\(data\)/);
  assert.match(app, /if \(fp !== lastBoardFingerprint\) queueBoardRender\(fp\)/);
  assert.match(app, /boardInteractionActive\(\)/);
  assert.match(app, /boardDragging = true/);
  assert.match(app, /boardDragging = false/);
  // A ended drag applies a deferred render at once.
  assert.match(app, /tryBoardRender\(\); \/\/ a deferred refresh applies/);
});

test("funnel figures are refetched on every open", () => {
  assert.doesNotMatch(app, /state\.funnel/);
});

test("cards expose rename and discard; draft columns offer new post", () => {
  assert.match(app, /openRename\(card\)/);
  assert.match(app, /openDiscard\(card\)/);
  assert.match(app, /if \(!card\.readOnly\)/);
  assert.match(app, /openNewPost\(st\)/);
  // The published column offers no creation control.
  assert.match(app, /st === "ideas" \|\| st === "wip"/);
  for (const id of ["new-post", "new-post-title", "rename", "rename-input", "confirm-discard", "discard-confirm"]) {
    assert.match(html, new RegExp(`id="${id}"`), id);
  }
});

test("published renames warn about the public URL before running", () => {
  assert.match(app, /rename-warning/);
  assert.match(app, /public URL path to \//);
  assert.match(app, /previewSlug\(\$\("rename-input"\)\.value\)/);
});

test("discard confirmation names the trash location and its reversibility", () => {
  assert.match(html, /drafts\/trash/);
  assert.match(html, /Restore it anytime by moving the folder back/);
});

test("the editor bridges to the project's dev server honestly", () => {
  assert.match(html, /id="editor-dev"/);
  assert.match(html, /Dev server not answering/);
  for (const id of ["dev-server", "dev-url-text", "dev-cancel", "dev-copy", "dev-open"]) {
    assert.match(html, new RegExp(`id="${id}"`), id);
  }
  // Folder posts use the pinned slug; loose posts fall back to the filename.
  assert.match(app, /\(p\.frontmatter && p\.frontmatter\.slug\) \|\| p\.name/);
  assert.match(app, /window\.open\(res\.url, "_blank", "noopener"\)/);
  // Unreachable is a choice, never a claimed success.
  assert.match(html, /Open anyway/);
  assert.match(app, /navigator\.clipboard\.writeText\(devURL\)/);
});

test("preview hydration asks editor-state which images are blocked instead of fetching them", () => {
  // The wiring: every data-src image is checked through blockedImageReason,
  // so loose (read-only) posts never request local images the server refuses.
  assert.match(app, /EditorState\.blockedImageReason\(ref, readOnly\)/);
  const editorState = readFileSync(new URL("../internal/server/web/editor-state.js", import.meta.url), "utf8");
  assert.match(editorState, /External image not loaded: /);
  assert.match(editorState, /loose posts hold no assets of their own/);
});

test("narrow viewports reflow both board and editor panes", () => {
  assert.match(css, /@media \(max-width: 700px\)/);
  assert.match(css, /#board \{ grid-template-columns: minmax\(0, 1fr\)/);
  assert.match(css, /#split \{ grid-template-columns: minmax\(0, 1fr\)/);
});
