// astrogui interface. The session token lives in the URL fragment (never the
// query string); it is lifted into the X-AstroGUI-Token header for every API
// call, which a cross-origin page cannot do.
"use strict";

(function () {
  var MD = window.ASTROGUI_MARKDOWN;
  var token = (location.hash.match(/token=([a-f0-9]+)/) || [])[1] || "";
  var state = { board: null, editorPost: null, editorModTime: null, dirty: false, funnel: null };
  var $ = function (id) { return document.getElementById(id); };

  function api(method, path, body) {
    return fetch(path, {
      method: method,
      headers: {
        "Content-Type": "application/json",
        "X-AstroGUI-Token": token,
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    }).then(function (res) {
      return res.json().catch(function () { return {}; }).then(function (data) {
        if (!res.ok) {
          var err = new Error(data.error || res.statusText);
          err.status = res.status;
          err.data = data;
          throw err;
        }
        return data;
      });
    });
  }

  function report(title, body) {
    $("report-title").textContent = title;
    $("report-body").textContent = typeof body === "string" ? body : JSON.stringify(body, null, 2);
    $("report").showModal();
  }

  // ---- time formatting -------------------------------------------------

  function ago(iso) {
    var ms = Date.now() - new Date(iso).getTime();
    if (ms < 0) ms = 0;
    var min = Math.round(ms / 60000);
    if (min < 1) return "now";
    if (min < 60) return min + "m";
    var h = Math.round(min / 60);
    if (h < 48) return h + "h";
    return Math.round(h / 24) + "d";
  }

  function size(bytes) {
    if (bytes < 1024) return bytes + "B";
    return Math.round(bytes / 102.4) / 10 + "k";
  }

  // ---- board ------------------------------------------------------------

  var STATE_LABELS = { ideas: "Ideas", wip: "In progress", published: "Published" };

  function renderBoard(data) {
    var board = $("board");
    board.innerHTML = "";
    data.states.forEach(function (st) {
      var col = document.createElement("section");
      col.className = "column";
      col.dataset.state = st;
      var head = document.createElement("h2");
      head.innerHTML = "<span>" + (STATE_LABELS[st] || st) + "</span><span>" + (data.columns[st] || []).length + "</span>";
      col.appendChild(head);

      (data.columns[st] || []).forEach(function (card) {
        var el = document.createElement("article");
        el.className = "card" + (card.readOnly ? " readonly" : "");
        el.draggable = !card.readOnly;
        el.dataset.post = card.name;
        var signals = [];
        if (card.stalled) signals.push('<span class="stalled">stalled ' + ago(card.meta.lastChanged) + "</span>");
        else signals.push("<span>changed " + ago(card.meta.lastChanged) + "</span>");
        signals.push("<span>age " + ago(card.meta.firstSeen) + "</span>");
        signals.push("<span>" + size(card.meta.size) + "</span>");
        if (card.meta.images) signals.push("<span>" + card.meta.images + " img</span>");
        if (card.loose) signals.push('<span class="loose">loose file</span>');
        el.innerHTML = "<h3>" + MD.escapeHTML(card.title) + "</h3>" +
          (card.snippet ? '<div class="snippet">' + MD.escapeHTML(card.snippet) + "</div>" : "") +
          '<div class="signals">' + signals.join("") + "</div>";
        el.addEventListener("click", function () { openEditor(card.name); });
        el.addEventListener("dragstart", function (ev) {
          ev.dataTransfer.setData("text/plain", card.name);
          el.style.opacity = 0.5;
        });
        el.addEventListener("dragend", function () { el.style.opacity = ""; });
        col.appendChild(el);
      });

      col.addEventListener("dragover", function (ev) { ev.preventDefault(); col.classList.add("dragover"); });
      col.addEventListener("dragleave", function () { col.classList.remove("dragover"); });
      col.addEventListener("drop", function (ev) {
        ev.preventDefault();
        col.classList.remove("dragover");
        var name = ev.dataTransfer.getData("text/plain");
        if (name && name !== "") movePost(name, st);
      });
      board.appendChild(col);
    });
  }

  function refreshBoard() {
    return api("GET", boardURL()).then(function (data) {
      state.board = data;
      renderBoard(data);
    });
  }

  function boardURL() { return "/api/collections/" + COLLECTION + "/board"; }
  function entryURL(post) { return "/api/collections/" + COLLECTION + "/entries/" + encodeURIComponent(post); }

  function movePost(name, to) {
    api("POST", entryURL(name) + "/move", { to: to }).then(function (res) {
      refreshBoard();
    }).catch(function (err) {
      if (err.status === 422) report("Cannot publish yet", (err.data.problems || []).map(function (p) { return "• " + p.message; }).join("\n"));
      else report("Move refused", err.message);
      refreshBoard(); // the board shows the truth, not the wish
    });
  }

  // ---- capture ----------------------------------------------------------

  function capture() {
    var input = $("capture-input");
    var line = input.value.trim();
    if (!line) return;
    api("POST", "/api/collections/" + COLLECTION + "/entries", { line: line }).then(function () {
      input.value = "";
      refreshBoard();
    }).catch(function (err) { report("Capture failed", err.message); });
  }

  // ---- funnel -----------------------------------------------------------

  function toggleFunnel() {
    var panel = $("funnel-panel");
    if (!state.funnel) {
      api("GET", "/api/collections/" + COLLECTION + "/funnel").then(function (f) {
        state.funnel = f;
        var rows = Object.keys(f.counts).map(function (st) {
          return "<tr><td>" + (STATE_LABELS[st] || st) + "</td><td>" + f.counts[st] +
            "</td><td>" + (f.advanced[st] || 0) + " advanced here</td></tr>";
        }).join("");
        panel.innerHTML = "<strong>Funnel</strong><table>" + rows + "</table>";
        panel.hidden = false;
      });
    } else {
      panel.hidden = !panel.hidden;
    }
  }

  // ---- version control actions ------------------------------------------

  function commit(push) {
    var message = window.prompt(push ? "Commit message (then push):" : "Commit message:", "");
    if (message === null) return; // cancelled: no action
    message = message.trim();
    if (!message) { report("No commit", "An empty message commits nothing; the action was cancelled."); return; }
    api("POST", "/api/collections/" + COLLECTION + "/commit", { message: message, push: push }).then(function (res) {
      report(res.ok ? (push ? "Committed and pushed" : "Committed") : "Action failed", res.command + "\n\n" + res.output);
    }).catch(function (err) { report("Action failed", err.message); });
  }

  // ---- editor -----------------------------------------------------------

  function openEditor(name) {
    api("GET", entryURL(name)).then(function (p) {
      state.editorPost = p;
      state.editorModTime = p.modTime || null;
      state.dirty = false;
      var isLoose = !!p.readOnly;
      $("editor-title").textContent = p.name + " — " + (STATE_LABELS[p.state] || p.state);
      $("editor-flags").textContent = isLoose
        ? "loose file — the whole file is shown, saved as raw text"
        : "";
      // The structured fields are for folder posts; a loose post is edited
      // only through the raw text view.
      $("fm-title").disabled = isLoose;
      $("fm-date").disabled = isLoose;
      $("fm-save").disabled = isLoose;
      $("fm-raw-toggle").hidden = isLoose;
      $("raw-wrap").hidden = true;
      $("source").value = isLoose ? fullFileText(p) : p.body;
      $("fm-title").value = (p.frontmatter && p.frontmatter.title) || "";
      $("fm-date").value = p.frontmatter && p.frontmatter.date ? String(p.frontmatter.date).slice(0, 10) : "";
      $("raw-fm").value = p.frontmatterRaw || "";
      updatePreview(p.body);
      $("editor-status").textContent = "";
      resetCheckBtn();
      hideCheckPop();
      $("editor").showModal();
    }).catch(function (err) { report("Open failed", err.message); });
  }

  // fullFileText reassembles a post's complete file bytes from the server's
  // parts: delimiter, frontmatter block, body — exactly what is on disk.
  function fullFileText(p) {
    if (!p.hasFrontmatter) return p.body;
    var fm = p.frontmatterRaw || "";
    if (fm !== "" && !fm.endsWith("\n")) fm += "\n";
    return FM_OPEN + fm + FM_CLOSE + p.body;
  }

  // updatePreview renders the reading copy of the post's body. The whole-file
  // text (with the frontmatter block) is only ever stored in the source pane.
  function updatePreview(text) {
    var src = text !== undefined ? text : $("source").value;
    // The frontmatter block is file structure, not prose: it doesn't belong
    // in the reading copy.
    var match = src.match(/^---\r?\n[\s\S]*?\r?\n---\r?\n/);
    if (match) src = src.slice(match[0].length);
    $("preview").innerHTML = MD.render(src);
    // Hydrate images through the token-checked API: img tags cannot carry
    // headers, so bytes are fetched and turned into object URLs.
    $("preview").querySelectorAll("img[data-src]").forEach(function (img) {
      var ref = img.getAttribute("data-src");
      if (/^(https?:)?\/\//.test(ref)) { img.src = ref; hydrateDone(img); return; }
      var post = state.editorPost.name;
      fetch("/api/collections/" + COLLECTION + "/entries/" + encodeURIComponent(post) +
        "/assets/" + encodeURIComponent(ref.split("?")[0]), {
        headers: { "X-AstroGUI-Token": token },
      }).then(function (res) {
        if (!res.ok) throw new Error("missing");
        return res.blob();
      }).then(function (blob) {
        img.src = URL.createObjectURL(blob);
        hydrateDone(img);
      }).catch(function () {
        // A missing reference must be visible, not silent.
        img.classList.add("broken");
        img.alt = "missing: " + ref;
        img.removeAttribute("data-src");
      });
    });
  }

  function hydrateDone(img) {
    img.addEventListener("load", function () { img.classList.remove("broken"); });
    img.removeAttribute("data-src");
  }

  function saveBody() {
    var p = state.editorPost;
    if (!p) return;
    if (p.readOnly) {
      // Loose file: the source pane holds the whole file; saving writes it
      // back verbatim through the raw endpoint — the spec's editing path
      // for posts the tool did not create.
      api("PUT", entryURL(p.name) + "/raw", {
        content: $("source").value,
        modTime: state.editorModTime,
      }).then(function (res) {
        state.editorModTime = res.modTime;
        state.dirty = false;
        $("editor-status").textContent = "saved as raw text " + new Date().toLocaleTimeString();
      }).catch(handleSaveError);
      return;
    }
    api("PUT", entryURL(p.name) + "/body", {
      body: $("source").value,
      modTime: state.editorModTime,
    }).then(function (res) {
      state.editorModTime = res.modTime;
      state.dirty = false;
      $("editor-status").textContent = res.saved === false ? "no change — file untouched" : "saved " + new Date().toLocaleTimeString();
    }).catch(handleSaveError);
  }

  function handleSaveError(err) {
    if (err.status === 409) {
      $("conflict-detail").textContent = "Another tool changed this file after you opened it. " +
        "The on-disk version was kept; your unsaved text is still in this editor.";
      $("conflict").showModal();
    } else {
      report("Save failed", err.message);
    }
  }

  function saveFields() {
    var p = state.editorPost;
    if (!p || p.readOnly) return;
    var fields = {};
    if ($("fm-title").value !== ((p.frontmatter && p.frontmatter.title) || "")) fields.title = $("fm-title").value;
    var currentDate = p.frontmatter && p.frontmatter.date ? String(p.frontmatter.date).slice(0, 10) : "";
    if ($("fm-date").value !== currentDate) fields.date = $("fm-date").value;
    if (!Object.keys(fields).length) {
      $("editor-status").textContent = "no field changed — nothing written";
      return;
    }
    api("PUT", entryURL(p.name) + "/frontmatter", { fields: fields, modTime: state.editorModTime })
      .then(function (res) {
        state.editorModTime = res.modTime;
        $("editor-status").textContent = res.changed ? "fields saved (comments and unknown fields preserved)" : "no field changed — nothing written";
        return openEditorSilent(p.name);
      })
      .catch(function (err) { report("Field save failed", err.message); });
  }

  function saveRaw() {
    var p = state.editorPost;
    if (!p) return;
    var content = rebuildWithFrontmatter(p, $("raw-fm").value);
    api("PUT", entryURL(p.name) + "/raw", { content: content, modTime: state.editorModTime })
      .then(function (res) {
        state.editorModTime = res.modTime;
        $("editor-status").textContent = "raw saved";
        openEditorSilent(p.name);
      })
      .catch(function (err) { report("Raw save failed", err.message); });
  }

  var FM_OPEN = "---\n", FM_CLOSE = "---\n";

  // Rebuilds the whole file from the edited frontmatter block plus the body,
  // byte-exact outside the frontmatter.
  function rebuildWithFrontmatter(p, fm) {
    fm = fm || "";
    if (fm !== "" && !fm.endsWith("\n")) fm += "\n";
    return FM_OPEN + fm + FM_CLOSE + p.body;
  }

  function openEditorSilent(name) {
    return api("GET", entryURL(name)).then(function (p) {
      var keepSrc = state.dirty ? $("source").value : null;
      state.editorPost = p;
      state.editorModTime = p.modTime || null;
      $("fm-title").value = (p.frontmatter && p.frontmatter.title) || "";
      $("fm-date").value = p.frontmatter && p.frontmatter.date ? String(p.frontmatter.date).slice(0, 10) : "";
      $("raw-fm").value = p.frontmatterRaw || "";
      if (keepSrc !== null) $("source").value = keepSrc;
    });
  }

  function preflightCheck() {
    var p = state.editorPost;
    if (!p) return;
    var btn = $("editor-check");
    btn.disabled = true;
    api("GET", entryURL(p.name)).then(function (fresh) {
      btn.disabled = false;
      var problems = fresh.preflight || [];
      // Restart the animation on repeated clicks.
      btn.classList.remove("success", "fail");
      void btn.offsetWidth;
      if (!problems.length) {
        // Success speaks through the button: green, with the verdict in the
        // status line. No modal for a yes.
        var published = fresh.state === "published";
        btn.classList.add("success");
        btn.textContent = published ? "✓ Healthy" : "✓ Ready";
        $("editor-status").textContent = published
          ? "Already in the collection and every check passes; your build includes it."
          : "Every check passes — drag the card to Published to publish it.";
        hideCheckPop();
        setTimeout(resetCheckBtn, 2400);
        return;
      }
      // Failure shows the list: anchored panel, one row per problem.
      btn.classList.add("fail");
      setTimeout(function () { btn.classList.remove("fail"); }, 900);
      showCheckPop(fresh.state, problems);
    }).catch(function (err) {
      btn.disabled = false;
      report("Check failed", err.message);
    });
  }

  function resetCheckBtn() {
    var btn = $("editor-check");
    btn.classList.remove("success", "fail");
    btn.textContent = "Check";
  }

  function showCheckPop(postState, problems) {
    var pop = $("check-pop");
    pop.innerHTML = "";
    var head = document.createElement("strong");
    head.textContent = postState === "published"
      ? "Published, but the checks flag problems"
      : "Not publishable yet";
    pop.appendChild(head);

    var ul = document.createElement("ul");
    problems.forEach(function (pr) {
      var li = document.createElement("li");
      var msg = document.createElement("span");
      msg.className = "msg";
      msg.textContent = pr.message;
      li.appendChild(msg);
      var m = /\(referenced at body line (\d+)\)/.exec(pr.message);
      if (m) {
        var jump = document.createElement("button");
        jump.className = "jump";
        jump.textContent = "line " + m[1];
        jump.title = "Select the reference in the editor";
        jump.addEventListener("click", function () { jumpToBodyLine(parseInt(m[1], 10)); });
        li.appendChild(jump);
      }
      ul.appendChild(li);
    });
    pop.appendChild(ul);

    if (postState === "published") {
      var note = document.createElement("p");
      note.className = "muted note";
      note.textContent = "Your build already sees this post — these references can fail it the same way the gate would have.";
      pop.appendChild(note);
    }

    var rect = $("editor-check").getBoundingClientRect();
    pop.style.top = (rect.bottom + 8) + "px";
    pop.style.left = Math.max(8, rect.right - 340) + "px";
    pop.hidden = false;
  }

  function hideCheckPop() {
    var pop = $("check-pop");
    if (pop) pop.hidden = true;
  }

  // bodyLineOffset: the source pane holds only the body for folder posts,
  // but the whole file for loose ones; map a body line onto the pane either
  // way.
  function bodyLineOffset() {
    var p = state.editorPost;
    if (!p || !p.readOnly || !p.hasFrontmatter) return 0;
    var fm = p.frontmatterRaw || "";
    if (fm !== "" && !fm.endsWith("\n")) fm += "\n";
    var prefix = "---\n" + fm + "---\n";
    return (prefix.match(/\n/g) || []).length;
  }

  function jumpToBodyLine(n) {
    var ta = $("source");
    var lines = ta.value.split("\n");
    var idx = Math.max(0, Math.min(lines.length - 1, n - 1 + bodyLineOffset()));
    var start = lines.slice(0, idx).join("\n").length + (idx ? 1 : 0);
    ta.focus();
    ta.setSelectionRange(start, start + lines[idx].length);
  }

  // ---- pasted images ------------------------------------------------------

  function pasteImage(ev) {
    var p = state.editorPost;
    if (!p || p.readOnly) return;
    var items = (ev.clipboardData || {}).items || [];
    for (var i = 0; i < items.length; i++) {
      if (items[i].type && items[i].type.indexOf("image/") === 0) {
        var file = items[i].getAsFile();
        var reader = new FileReader();
        reader.onload = function () {
          var data = String(reader.result).split(",")[1] || "";
          var name = file.name && file.name !== "image.png" ? file.name : "pasted-" + Date.now() + ".png";
          api("POST", entryURL(p.name) + "/assets", { name: name, data: data }).then(function (res) {
            insertAtCursor("![](" + res.name + ")");
            $("editor-status").textContent = "image saved into the post's own folder: " + res.name;
          }).catch(function (err) { report("Image save failed", err.message); });
        };
        reader.readAsDataURL(file);
        ev.preventDefault();
        return;
      }
    }
  }

  function insertAtCursor(text) {
    var ta = $("source");
    var start = ta.selectionStart, end = ta.selectionEnd;
    ta.value = ta.value.slice(0, start) + text + ta.value.slice(end);
    ta.selectionStart = ta.selectionEnd = start + text.length;
    ta.focus();
    updatePreview();
    state.dirty = true;
  }

  // ---- wiring -------------------------------------------------------------

  var COLLECTION = null;

  document.addEventListener("DOMContentLoaded", function () {
    api("GET", "/api/health").then(function (h) {
      COLLECTION = h.collection;
      $("projectline").textContent = h.project + " · " + h.collection;
      return refreshBoard();
    }).catch(function (err) {
      document.body.innerHTML = '<p style="padding:20px">astrogui: ' + MD.escapeHTML(err.message) + "</p>";
    });

    // The board tracks the filesystem, including changes made outside the
    // tool: poll and re-render without any page reload.
    setInterval(function () {
      if (!$("editor").open) refreshBoard().catch(function () {});
    }, 2000);

    $("capture-btn").addEventListener("click", capture);
    $("capture-input").addEventListener("keydown", function (ev) { if (ev.key === "Enter") capture(); });
    $("funnel-btn").addEventListener("click", toggleFunnel);
    $("commit-btn").addEventListener("click", function () { commit(false); });
    $("push-btn").addEventListener("click", function () { commit(true); });

    $("editor-back").addEventListener("click", function () { $("editor").close(); hideCheckPop(); refreshBoard(); });
    $("editor-save").addEventListener("click", saveBody);
    $("editor-check").addEventListener("click", preflightCheck);
    // The check panel dismisses on any click outside itself (or on its own
    // anchor button, which toggles it).
    document.addEventListener("click", function (ev) {
      var pop = $("check-pop");
      if (pop.hidden) return;
      if (!pop.contains(ev.target) && ev.target.id !== "editor-check") hideCheckPop();
    });
    $("fm-save").addEventListener("click", saveFields);
    $("fm-raw-toggle").addEventListener("click", function () {
      var wrap = $("raw-wrap");
      wrap.hidden = !wrap.hidden;
    });
    $("raw-save").addEventListener("click", saveRaw);
    $("source").addEventListener("input", function () { state.dirty = true; updatePreview(); });
    $("source").addEventListener("paste", pasteImage);
    $("source").addEventListener("keydown", function (ev) {
      if ((ev.metaKey || ev.ctrlKey) && ev.key === "s") { ev.preventDefault(); saveBody(); }
    });

    $("conflict-reload").addEventListener("click", function () {
      $("conflict").close();
      openEditor(state.editorPost.name); // the on-disk version wins
    });
    $("conflict-keep").addEventListener("click", function () { $("conflict").close(); });
    $("report-close").addEventListener("click", function () { $("report").close(); });
  });
})();
