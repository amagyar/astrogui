import { createRequire } from "node:module";
import { test } from "node:test";
import assert from "node:assert/strict";

const require = createRequire(import.meta.url);
const EditorState = require("../internal/server/web/editor-state.js");

const clean = { source: "body", title: "title", date: "2026-01-01", raw: "title: title\n" };

test("editor is clean until any editable surface differs from its baseline", () => {
  assert.equal(EditorState.dirty(EditorState.changed(clean, clean)), false);
  for (const field of Object.keys(clean)) {
    const current = { ...clean, [field]: clean[field] + " changed" };
    assert.equal(EditorState.dirty(EditorState.changed(current, clean)), true, field);
  }
});

test("save plan writes body, fields, and raw metadata without losing either surface", () => {
  assert.deepEqual(EditorState.savePlan(false, { source: true, title: true, date: false, raw: false }), ["body", "fields"]);
  assert.deepEqual(EditorState.savePlan(false, { source: true, title: true, date: false, raw: true }), ["raw", "fields"]);
  assert.deepEqual(EditorState.savePlan(true, { source: true, title: false, date: false, raw: false }), ["body"]);
});

test("clean and read-only editors do not request unnecessary saves", () => {
  assert.deepEqual(EditorState.savePlan(false, { source: false, title: false, date: false, raw: false }), []);
  assert.deepEqual(EditorState.savePlan(true, { source: false, title: false, date: false, raw: false }), []);
});

test("external image references are identified without permitting implicit fetches", () => {
  assert.equal(EditorState.isExternalImage("https://example.test/a.png"), true);
  assert.equal(EditorState.isExternalImage("//example.test/a.png"), true);
  assert.equal(EditorState.isExternalImage("./local.png"), false);
});
