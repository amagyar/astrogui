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
  assert.match(app, /\$\("source"\)\.addEventListener\("input", function \(\) \{ updatePreview\(\); \}\)/);
  assert.match(app, /External image not loaded:/);
});

test("narrow viewports reflow both board and editor panes", () => {
  assert.match(css, /@media \(max-width: 700px\)/);
  assert.match(css, /#board \{ grid-template-columns: minmax\(0, 1fr\)/);
  assert.match(css, /#split \{ grid-template-columns: minmax\(0, 1fr\)/);
});
