// astrogui markdown: a reading-copy renderer for the preview pane.
//
// The preview is content-accurate and style-oblivious by design: it conveys
// the post's structure, not the published site's styling, code highlighting,
// or image optimization.
//
// Post content is untrusted here. sanitize() strips script elements,
// event-handler attributes, and dangerous URL schemes before anything enters
// the tool's page; the server's Content-Security-Policy is the second layer.
(function (root, factory) {
  if (typeof module === "object" && module.exports) module.exports = factory();
  else root.ASTROGUI_MARKDOWN = factory();
})(typeof self !== "undefined" ? self : this, function () {
  "use strict";

  var ALLOWED_TAGS = {
    a: ["href", "title"], abbr: ["title"], b: [], blockquote: [], br: [],
    code: [], del: [], dd: [], div: [], dl: [], dt: [], em: [], figcaption: [],
    figure: [], h1: [], h2: [], h3: [], h4: [], h5: [], h6: [], hr: [], i: [],
    img: ["src", "alt", "title", "data-src"], ins: [], kbd: [], li: [], mark: [],
    ol: ["start"], p: [], pre: [], q: [], s: [], span: [], strong: [], sub: [],
    sup: [], table: [], tbody: [], td: [], th: ["align"], thead: [], tr: [],
    tfoot: [], u: [], ul: [], input: ["type", "checked", "disabled"],
    details: ["open"], summary: [], footnote: [],
  };

  function safeURL(url) {
    var u = String(url || "").trim();
    if (/^(https?:|mailto:|data:image\/(png|gif|jpeg|webp|avif);base64,|#|\/)/i.test(u)) return u;
    if (/^[a-z0-9._\-/]+$/i.test(u)) return u; // site-relative or post-relative
    return "";
  }

  function sanitize(html) {
    if (typeof DOMParser === "undefined") {
      // No DOM available (Node tests). The renderer has already escaped all
      // raw HTML at the source — every tag below came from the renderer
      // itself — so the input is safe by construction; return it as-is.
      return html;
    }
    var doc = new DOMParser().parseFromString(html, "text/html");
    var walk = function (node) {
      var children = Array.prototype.slice.call(node.childNodes);
      children.forEach(function (child) {
        if (child.nodeType === 3 /* text */ || child.nodeType === 8 /* comment (dropped below) */) {
          if (child.nodeType === 8) node.removeChild(child);
          return;
        }
        if (child.nodeType !== 1) return;
        var tag = child.tagName.toLowerCase();
        var allowed = ALLOWED_TAGS[tag];
        if (!allowed || tag === "script" || tag === "style" || tag === "iframe" ||
            tag === "object" || tag === "embed" || tag === "form" || tag === "link") {
          // Replace unknown/unsafe elements with their text content, so no
          // payload survives and nothing meaningful is hidden.
          var text = doc.createTextNode(child.textContent);
          node.replaceChild(text, child);
          return;
        }
        var attrs = Array.prototype.slice.call(child.attributes);
        attrs.forEach(function (attr) {
          var name = attr.name.toLowerCase();
          var isHandler = name.startsWith("on");
          var allowedHere = allowed.indexOf(attr.name) !== -1;
          if (isHandler || !allowedHere) {
            child.removeAttribute(attr.name);
            return;
          }
          if ((name === "href" || name === "src") && !safeURL(attr.value)) {
            child.removeAttribute(attr.name);
          }
        });
        walk(child);
      });
    };
    walk(doc.body);
    return doc.body.innerHTML;
  }

  function escapeHTML(s) {
    return String(s).replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }

  function inline(s) {
    // Code spans come out first and go back last, so no later rule —
    // image, link, or emphasis — can ever see inside them: `**x**` stays
    // literal code, and image or link syntax inside backticks stays
    // visible text. Sentinels carry a NUL, which typed text cannot
    // produce (renderMarkdown strips it from the source up front).
    var codeSpans = [];
    var out = String(s).replace(/`([^`]+)`/g, function (m, code) {
      codeSpans.push("<code>" + escapeHTML(code) + "</code>");
      return "\u0000CB" + (codeSpans.length - 1) + "\u0000";
    });
    out = escapeHTML(out);
    // images: ![alt](src) — rendered with data-src so the app can hydrate
    // them through the token-checked API (img tags cannot set headers).
    out = out.replace(/!\[([^\]]*)\]\(([^)\s]+)(?:\s+&quot;([^&]*)&quot;)?\)/g,
      function (m, alt, src) {
        return '<img alt="' + alt + '" data-src="' + escapeHTML(src) + '">';
      });
    // links
    out = out.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, function (m, text, href) {
      var url = safeURL(href.replace(/&amp;/g, "&"));
      if (!url) return text;
      return '<a href="' + escapeHTML(url) + '">' + text + "</a>";
    });
    out = out.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
    out = out.replace(/(^|\W)\*([^*\s]+)\*/g, "$1<em>$2</em>");
    out = out.replace(/(^|\W)_([^_\s]+)_/g, "$1<em>$2</em>");
    out = out.replace(/~~([^~]+)~~/g, "<del>$1</del>");
    // footnote references: [^id] -> superscript link
    out = out.replace(/\[\^([^\]]+)\]/g, function (m, id) {
      return '<sup><a href="#fn-' + escapeHTML(id) + '">[' + escapeHTML(id) + "]</a></sup>";
    });
    // Code spans return escaped, after every other rule has run.
    out = out.replace(/\u0000CB(\d+)\u0000/g, function (m, i) {
      return codeSpans[i] || "";
    });
    return out;
  }

  function renderMarkdown(src) {
    // NUL is the code-span sentinel character; stripping it from the source
    // keeps a crafted file from forging a sentinel of its own.
    var lines = String(src || "").replace(/\r\n/g, "\n").replace(/\u0000/g, "").split("\n");
    var html = [];
    var i = 0;

    function closeLists(stack) { while (stack.length) html.push("</" + stack.pop() + ">"); }

    var listStack = [];
    while (i < lines.length) {
      var line = lines[i];

      // fenced code
      var fence = line.match(/^```(.*)$/);
      if (fence) {
        closeLists(listStack);
        var meta = fence[1];
        var buf = [];
        i++;
        while (i < lines.length && !/^```/.test(lines[i])) { buf.push(lines[i]); i++; }
        i++; // closing fence
        html.push('<pre data-fence="' + escapeHTML(meta) + '"><code>' + escapeHTML(buf.join("\n")) + "</code></pre>");
        continue;
      }

      // headings
      var h = line.match(/^(#{1,6})\s+(.*)$/);
      if (h) {
        closeLists(listStack);
        var level = h[1].length;
        html.push("<h" + level + ">" + inline(h[2]) + "</h" + level + ">");
        i++;
        continue;
      }

      // horizontal rule
      if (/^(\s*)(-{3,}|\*{3,}|_{3,})\s*$/.test(line)) {
        closeLists(listStack);
        html.push("<hr>");
        i++;
        continue;
      }

      // blockquote
      if (/^>\s?/.test(line)) {
        closeLists(listStack);
        var quote = [];
        while (i < lines.length && /^>\s?/.test(lines[i])) {
          quote.push(lines[i].replace(/^>\s?/, ""));
          i++;
        }
        html.push("<blockquote>" + renderMarkdown(quote.join("\n")) + "</blockquote>");
        continue;
      }

      // table: header row + separator
      if (line.indexOf("|") !== -1 && i + 1 < lines.length && /^\s*\|?[\s:|-]+\|[\s:|-]*$/.test(lines[i + 1])) {
        closeLists(listStack);
        var header = splitRow(line);
        var align = splitRow(lines[i + 1]).map(function (c) {
          if (/^:-+:$/.test(c.trim())) return "center";
          if (/^-+:$/.test(c.trim())) return "right";
          return "";
        });
        i += 2;
        var rows = [];
        while (i < lines.length && lines[i].indexOf("|") !== -1 && lines[i].trim() !== "") {
          rows.push(splitRow(lines[i]));
          i++;
        }
        var t = "<table><thead><tr>";
        header.forEach(function (c, ci) {
          t += "<th" + (align[ci] ? ' align="' + align[ci] + '"' : "") + ">" + inline(c) + "</th>";
        });
        t += "</tr></thead><tbody>";
        rows.forEach(function (r) {
          t += "<tr>";
          for (var ci = 0; ci < header.length; ci++) {
            t += "<td" + (align[ci] ? ' align="' + align[ci] + '"' : "") + ">" + inline(r[ci] || "") + "</td>";
          }
          t += "</tr>";
        });
        t += "</tbody></table>";
        html.push(t);
        continue;
      }

      // lists
      var ul = line.match(/^[-*+]\s+(.*)$/);
      var ol = line.match(/^(\d+)[.)]\s+(.*)$/);
      if (ul || ol) {
        var want = ul ? "ul" : "ol";
        if (listStack[listStack.length - 1] !== want) {
          closeLists(listStack);
          listStack.push(want);
          html.push("<" + want + ">");
        }
        html.push("<li>" + inline((ul || ol)[(ul ? 1 : 2)]) + "</li>");
        i++;
        continue;
      }
      if (listStack.length && line.trim() === "") { closeLists(listStack); }

      // paragraph (blank line ends nothing else to end)
      if (line.trim() === "") { i++; continue; }

      // footnote definitions: [^id]: text
      var fn = line.match(/^\[\^([^\]]+)\]:\s+(.*)$/);
      if (fn) {
        closeLists(listStack);
        html.push('<p class="footnote" id="fn-' + escapeHTML(fn[1]) + '">' + inline(fn[2]) + "</p>");
        i++;
        continue;
      }

      closeLists(listStack);
      var para = [line];
      i++;
      while (i < lines.length && lines[i].trim() !== "" && !/^(#{1,6}\s|>|```|[-*+]\s|\d+[.)]\s)/.test(lines[i])) {
        para.push(lines[i]);
        i++;
      }
      // trailing double-space = hard line break, preserved in the reading copy
      html.push("<p>" + inline(para.join("\n")).replace(/\n/g, " ").replace(/ {2} /g, "<br>") + "</p>");
    }
    closeLists(listStack);
    return html.join("\n");
  }

  function splitRow(row) {
    return row.replace(/^\s*\|/, "").replace(/\|\s*$/, "").split("|").map(function (c) { return c.trim(); });
  }

  return {
    renderMarkdown: renderMarkdown,
    sanitize: sanitize,
    // render: markdown -> sanitized HTML, the preview entry point.
    render: function (src) { return sanitize(renderMarkdown(src)); },
    safeURL: safeURL,
    escapeHTML: escapeHTML,
  };
});
