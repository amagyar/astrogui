// Preview renderer and sanitizer tests, run with Node against the exact file
// the binary embeds (tasks 8.1, 8.2 rendering, 8.4, 8.11).
import { createRequire } from "node:module";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import assert from "node:assert/strict";

const require = createRequire(import.meta.url);
const src = readFileSync(new URL("../internal/server/web/markdown.js", import.meta.url), "utf8");
// Evaluate the UMD bundle without a DOM to get the factory, then call it with
// a DOMParser shim only where needed; sanitize() needs DOMParser, so this
// test file runs under Node's lack of DOMParser by exercising renderMarkdown
// and the pure helpers, plus sanitize via a linkedom-free check below.
const factory = new Function("module", "exports", src + "\nreturn module.exports;")(
  { exports: {} }, {});
const MD = factory;

test("code fences keep their metadata in the reading copy", () => {
  const html = MD.renderMarkdown("```rust {filename=main.rs}\nfn main() {}\n```");
  assert.match(html, /<pre data-fence="rust \{filename=main\.rs\}">/);
  assert.match(html, /fn main\(\) \{\}/);
});

test("tables, headings, footnotes, and breaks render", () => {
  const html = MD.renderMarkdown(
    "# Title\n\n| a | b |\n| -- | --: |\n| 1 | 2 |\n\nA note[^1].\n\n[^1]: the note\n");
  assert.match(html, /<h1>Title<\/h1>/);
  assert.match(html, /<table>/);
  assert.match(html, /<th align="right">b<\/th>/);
  assert.match(html, /href="#fn-1"/);
  assert.match(html, /id="fn-1"/);
});

test("images render with data-src for token-checked hydration", () => {
  const html = MD.renderMarkdown("![cover](cover.jpeg)");
  assert.match(html, /<img alt="cover" data-src="cover\.jpeg">/);
});

test("code spans render literally: no emphasis, image, or link inside backticks", () => {
  const html = MD.renderMarkdown("Use `**bold**`, `![a](b.png)`, and `[x](https://e.test)` in code.");
  assert.match(html, /<code>\*\*bold\*\*<\/code>/);
  assert.match(html, /<code>!\[a\]\(b\.png\)<\/code>/);
  assert.match(html, /<code>\[x\]\(https:\/\/e\.test\)<\/code>/);
  assert.ok(!/<img\b/.test(html), "image syntax inside a code span became an image");
  assert.ok(!/href="https:\/\/e\.test"/.test(html), "link syntax inside a code span became a link");
});

test("code-span sentinels cannot be forged with raw NUL bytes", () => {
  const html = MD.renderMarkdown("x\u0000CB0\u0000y");
  assert.ok(!/\u0000/.test(html), "NUL survived into the output");
  assert.ok(!/<code>/.test(html), "a forged sentinel produced a code span");
});

test("trailing double-space line breaks survive as <br>", () => {
  const html = MD.renderMarkdown("line one  \nline two");
  assert.match(html, /<br>/);
});

test("safeURL allows relative and http, blocks javascript and data", () => {
  assert.equal(MD.safeURL("cover.jpeg"), "cover.jpeg");
  assert.equal(MD.safeURL("https://astro.build/x.png"), "https://astro.build/x.png");
  assert.equal(MD.safeURL("javascript:alert(1)"), "");
  assert.equal(MD.safeURL("data:text/html;base64,xxx"), "");
});

// Task 8.11: a hostile post's content must not execute in the preview. The
// renderer escapes all raw HTML at the source — post markup can never become
// live elements — and this verifies the property over the hostile fixture:
// no script element, no event-handler attribute, no javascript: URL survives
// as live markup in the rendered output.
test("hostile post content cannot execute in the preview", () => {
  const hostile = "---\nslug: hostile\n---\n\n" +
    "<script>window.__pwned = 1;</script>\n\n" +
    '<img src=x onerror="window.__pwned2 = 1">\n\n' +
    "[link](javascript:window.__pwned3=1)\n\n" +
    '<a href="javascript:window.__pwned4=1">anchor</a>\n';
  // Strip the frontmatter the way the editor feeds the body to the preview.
  const body = hostile.replace(/^---\n[\s\S]*?\n---\n/, "");
  const html = MD.render(body);

  assert.ok(!/<script[\s>]/i.test(html), "live script element in output");
  // Strip escaped tags (their inner text is inert) before looking for a
  // live handler attribute on a real element.
  const liveOnly = html.replace(/&lt;[\s\S]*?&gt;/g, "");
  assert.ok(!/\sonerror\s*=/i.test(liveOnly), "live event handler in output");
  assert.ok(!/href="javascript:/i.test(html), "live javascript: URL in output");
  // The hostile markup is still visible as text: nothing is silently hidden.
  assert.match(html, /&lt;script&gt;/);
});

// Task 8.4's preview-side logic: image references render as data-src
// candidates that the app hydrates (or marks broken when they 404).
test("missing image references remain visible candidates", () => {
  const html = MD.render("![gone](gone.png)");
  assert.match(html, /data-src="gone\.png"/);
  assert.match(html, /alt="gone"/);
});
