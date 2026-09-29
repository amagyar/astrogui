// astrogui interface. The session token lives in the URL fragment (never the
// query string); it is lifted into the X-AstroGUI-Token header for every API
// call, which a cross-origin page cannot do.
"use strict";

(function () {
  var MD = window.ASTROGUI_MARKDOWN;
  var EditorState = window.ASTROGUI_EDITOR_STATE;
  var token = (location.hash.match(/token=([a-f0-9]+)/) || [])[1] || "";
  var state = {
    board: null, editorPost: null, editorModTime: null, editorBaseline: null,
    conflictAction: null, conflictData: null, commitPush: false, funnel: null,
  };
  var $ = function (id) { return document.getElementById(id); };

  function editorValues() {
    return {
      source: $("source").value,
      title: $("fm-title").value,
      date: $("fm-date").value,
      raw: $("raw-fm").value,
    };
  }

  function changedEditorFields() {
    var current = editorValues();
    var baseline = state.editorBaseline || current;
    return EditorState.changed(current, baseline);
  }

  function hasUnsavedChanges() {
    return EditorState.dirty(changedEditorFields());
  }

  function markEditorFieldsSaved(fields) {
    if (!state.editorBaseline) state.editorBaseline = editorValues();
    var current = editorValues();
    fields.forEach(function (field) { state.editorBaseline[field] = current[field]; });
  }

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
        var heading = document.createElement("h3");
        var open = document.createElement("button");
        open.type = "button";
        open.className = "card-open";
        open.textContent = card.title;
        open.setAttribute("aria-label", "Open post: " + card.title);
        open.addEventListener("click", function () { openEditor(card.name); });
        heading.appendChild(open);
        el.appendChild(heading);
        var signals = [];
        if (card.stalled) signals.push('<span class="stalled">stalled ' + ago(card.meta.lastChanged) + "</span>");
        else signals.push("<span>changed " + ago(card.meta.lastChanged) + "</span>");
        signals.push("<span>age " + ago(card.meta.firstSeen) + "</span>");
        signals.push("<span>" + size(card.meta.size) + "</span>");
        if (card.meta.images) signals.push("<span>" + card.meta.images + " img</span>");
        if (card.loose) signals.push('<span class="loose">loose file</span>');
        if (card.snippet) {
          var snippet = document.createElement("div");
          snippet.className = "snippet";
          snippet.textContent = card.snippet;
          el.appendChild(snippet);
        }
        var signalLine = document.createElement("div");
        signalLine.className = "signals";
        signalLine.innerHTML = signals.join("");
        el.appendChild(signalLine);

        var moveLabel = document.createElement("label");
        moveLabel.className = "move-control";
        var moveName = document.createElement("span");
        moveName.className = "sr-only";
        moveName.textContent = "Move " + card.title;
        moveLabel.appendChild(moveName);
        var destination = document.createElement("select");
        destination.name = "destination-" + card.name;
        destination.setAttribute("aria-label", "Move " + card.title + " to another state");
        destination.disabled = !!card.readOnly;
        var placeholder = document.createElement("option");
        placeholder.value = "";
        placeholder.textContent = card.readOnly ? "Read-only" : "Move to…";
        destination.appendChild(placeholder);
        data.states.forEach(function (target) {
          if (target === st) return;
          var option = document.createElement("option");
          option.value = target;
          option.textContent = STATE_LABELS[target] || target;
          destination.appendChild(option);
        });
        destination.addEventListener("change", function () {
          if (destination.value) movePost(card.name, destination.value);
          destination.value = "";
        });
        moveLabel.appendChild(destination);
        el.appendChild(moveLabel);
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
    state.commitPush = push;
    api("GET", "/api/collections/" + COLLECTION + "/git-status").then(function (status) {
      var list = $("commit-files");
      list.innerHTML = "";
      (status.changes || []).forEach(function (change) {
        var li = document.createElement("li");
        li.textContent = change.status + "  " + change.path;
        list.appendChild(li);
      });
      $("commit-clean").hidden = !(status.clean || !(status.changes || []).length);
      $("commit-action").textContent = push
        ? "After confirmation: git add -A, commit, then push to the tracked remote."
        : "After confirmation: git add -A and commit the whole working tree.";
      $("commit-confirm").textContent = push ? "Stage all, commit & push" : "Stage all and commit";
      $("commit-review").showModal();
      $("commit-message").focus();
    }).catch(function (err) { report("Could not preview Git changes", err.message); });
  }

  function confirmCommit() {
    var message = $("commit-message").value.trim();
    if (!message) {
      report("No commit", "Enter a commit message or cancel. No Git action has run.");
      return;
    }
    var push = state.commitPush;
    $("commit-review").close();
    api("POST", "/api/collections/" + COLLECTION + "/commit", { message: message, push: push }).then(function (res) {
      report(res.ok ? (push ? "Committed and pushed" : "Committed") : "Action failed", res.command + "\n\n" + res.output);
    }).catch(function (err) {
      if (err.data && err.data.command) {
        report("Action failed", err.data.command + "\n\n" + (err.data.output || err.message));
      } else {
        report("Action failed", err.message);
      }
    });
  }

  // ---- editor -----------------------------------------------------------

  function openEditor(name) {
    api("GET", entryURL(name)).then(function (p) {
      state.editorPost = p;
      state.editorModTime = p.modTime || null;
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
      state.editorBaseline = editorValues();
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
      if (EditorState.isExternalImage(ref)) {
        var external = document.createElement("span");
        external.className = "external-image";
        external.textContent = "External image not loaded: " + ref;
        img.replaceWith(external);
        return;
      }
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
    if (!p) return Promise.resolve();
    if (p.readOnly) {
      // Loose file: the source pane holds the whole file; saving writes it
      // back verbatim through the raw endpoint — the spec's editing path
      // for posts the tool did not create.
      return api("PUT", entryURL(p.name) + "/raw", {
        content: $("source").value,
        modTime: state.editorModTime,
      }).then(function (res) {
        state.editorModTime = res.modTime;
        markEditorFieldsSaved(["source"]);
        $("editor-status").textContent = "saved as raw text " + new Date().toLocaleTimeString();
        return res;
      }).catch(function (err) { handleSaveError(err, "body"); throw err; });
    }
    return api("PUT", entryURL(p.name) + "/body", {
      body: $("source").value,
      modTime: state.editorModTime,
    }).then(function (res) {
      state.editorModTime = res.modTime;
      markEditorFieldsSaved(["source"]);
      $("editor-status").textContent = res.saved === false ? "no change — file untouched" : "saved " + new Date().toLocaleTimeString();
      return res;
    }).catch(function (err) { handleSaveError(err, "body"); throw err; });
  }

  function editorSnapshotForConflict() {
    var p = state.editorPost;
    if (!p || p.readOnly) return $("source").value;
    return "Structured fields and raw frontmatter:\n" + JSON.stringify({
      title: $("fm-title").value,
      date: $("fm-date").value,
      frontmatter: $("raw-fm").value,
    }, null, 2) + "\n\nEdited body:\n" + $("source").value;
  }

  function handleSaveError(err, action) {
    if (err.status === 409) {
      state.conflictAction = action;
      state.conflictData = err.data || {};
      $("conflict-detail").textContent = "The on-disk version was kept. Review both versions; replacing the disk version is an explicit action.";
      $("conflict-mine").value = editorSnapshotForConflict();
      $("conflict-disk").value = state.conflictData.current || "(current version unavailable)";
      $("conflict").showModal();
    } else {
      report("Save failed", err.message);
    }
  }

  function saveFields() {
    var p = state.editorPost;
    if (!p || p.readOnly) return Promise.resolve();
    var fields = {};
    if ($("fm-title").value !== ((p.frontmatter && p.frontmatter.title) || "")) fields.title = $("fm-title").value;
    var currentDate = p.frontmatter && p.frontmatter.date ? String(p.frontmatter.date).slice(0, 10) : "";
    if ($("fm-date").value !== currentDate) fields.date = $("fm-date").value;
    if (!Object.keys(fields).length) {
      $("editor-status").textContent = "no field changed — nothing written";
      return Promise.resolve();
    }
    return api("PUT", entryURL(p.name) + "/frontmatter", { fields: fields, modTime: state.editorModTime })
      .then(function (res) {
        state.editorModTime = res.modTime;
        markEditorFieldsSaved(Object.keys(fields));
        $("editor-status").textContent = res.changed ? "fields saved (comments and unknown fields preserved)" : "no field changed — nothing written";
        return openEditorSilent(p.name);
      })
      .catch(function (err) { handleSaveError(err, "fields"); throw err; });
  }

  function saveRaw() {
    var p = state.editorPost;
    if (!p) return Promise.resolve();
    var content = rebuildWithFrontmatter(p, $("raw-fm").value, $("source").value);
    return api("PUT", entryURL(p.name) + "/raw", { content: content, modTime: state.editorModTime })
      .then(function (res) {
        state.editorModTime = res.modTime;
        markEditorFieldsSaved(["raw", "source"]);
        $("editor-status").textContent = "raw saved";
        return openEditorSilent(p.name);
      })
      .catch(function (err) { handleSaveError(err, "raw"); throw err; });
  }

  var FM_OPEN = "---\n", FM_CLOSE = "---\n";

  // Rebuilds the whole file from the edited frontmatter block plus the body,
  // byte-exact outside the frontmatter.
  function rebuildWithFrontmatter(p, fm, body) {
    fm = fm || "";
    if (fm !== "" && !fm.endsWith("\n")) fm += "\n";
    return FM_OPEN + fm + FM_CLOSE + (body === undefined ? p.body : body);
  }

  function openEditorSilent(name) {
    return api("GET", entryURL(name)).then(function (p) {
      var previousBaseline = state.editorBaseline || editorValues();
      var changed = changedEditorFields();
      state.editorPost = p;
      state.editorModTime = p.modTime || null;
      if (!changed.source) $("source").value = p.readOnly ? fullFileText(p) : p.body;
      if (!changed.title) $("fm-title").value = (p.frontmatter && p.frontmatter.title) || "";
      if (!changed.date) $("fm-date").value = p.frontmatter && p.frontmatter.date ? String(p.frontmatter.date).slice(0, 10) : "";
      if (!changed.raw) $("raw-fm").value = p.frontmatterRaw || "";
      state.editorBaseline = {
        source: changed.source ? previousBaseline.source : $("source").value,
        title: changed.title ? previousBaseline.title : $("fm-title").value,
        date: changed.date ? previousBaseline.date : $("fm-date").value,
        raw: changed.raw ? previousBaseline.raw : $("raw-fm").value,
      };
      updatePreview(p.readOnly ? p.body : $("source").value);
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
    }

  function saveAllEditorChanges() {
    if (!state.editorPost) return Promise.resolve();
    var changed = changedEditorFields();
    var plan = EditorState.savePlan(!!state.editorPost.readOnly, changed);
    var actions = plan.map(function (name) {
      return name === "raw" ? saveRaw : name === "fields" ? saveFields : saveBody;
    });
    return actions.reduce(function (promise, save) {
      return promise.then(save);
    }, Promise.resolve());
  }

  function closeEditor() {
    $("editor").close();
    hideCheckPop();
    refreshBoard();
  }

  function requestEditorClose() {
    if (hasUnsavedChanges()) {
      $("unsaved").showModal();
      return;
    }
    closeEditor();
  }

  function runSaveAction(action) {
    if (action === "fields") return saveFields();
    if (action === "raw") return saveRaw();
    return saveBody();
  }

  function replaceDiskWithEditorVersion() {
    var modTime = state.conflictData && state.conflictData.currentModTime;
    if (!modTime) {
      report("Conflict recovery failed", "The current file version could not be identified. Reload it before saving.");
      return;
    }
    var action = state.conflictAction;
    state.editorModTime = modTime;
    $("conflict").close();
    runSaveAction(action).catch(function () {});
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
    $("commit-cancel").addEventListener("click", function () { $("commit-review").close(); });
    $("commit-confirm").addEventListener("click", confirmCommit);

    $("editor-back").addEventListener("click", requestEditorClose);
    $("editor").addEventListener("cancel", function (ev) {
      if (hasUnsavedChanges()) {
        ev.preventDefault();
        $("unsaved").showModal();
      }
    });
    $("editor-save").addEventListener("click", function () { saveAllEditorChanges().catch(function () {}); });
    $("editor-check").addEventListener("click", preflightCheck);
    // The check panel dismisses on any click outside itself (or on its own
    // anchor button, which toggles it).
    document.addEventListener("click", function (ev) {
      var pop = $("check-pop");
      if (pop.hidden) return;
      if (!pop.contains(ev.target) && ev.target.id !== "editor-check") hideCheckPop();
    });
    $("fm-save").addEventListener("click", function () { saveFields().catch(function () {}); });
    $("fm-raw-toggle").addEventListener("click", function () {
      var wrap = $("raw-wrap");
      wrap.hidden = !wrap.hidden;
    });
    $("raw-save").addEventListener("click", function () { saveRaw().catch(function () {}); });
    $("source").addEventListener("input", function () { updatePreview(); });
    $("source").addEventListener("paste", pasteImage);
    $("source").addEventListener("keydown", function (ev) {
      if ((ev.metaKey || ev.ctrlKey) && ev.key === "s") { ev.preventDefault(); saveAllEditorChanges().catch(function () {}); }
    });

    $("conflict-reload").addEventListener("click", function () {
      $("conflict").close();
      openEditor(state.editorPost.name); // the on-disk version wins
    });
    $("conflict-back").addEventListener("click", function () {
      $("conflict").close();
      $("source").focus();
    });
    $("conflict-use-mine").addEventListener("click", replaceDiskWithEditorVersion);
    $("unsaved-keep").addEventListener("click", function () { $("unsaved").close(); });
    $("unsaved-discard").addEventListener("click", function () {
      $("unsaved").close();
      closeEditor();
    });
    $("unsaved-save").addEventListener("click", function () {
      $("unsaved").close();
      saveAllEditorChanges().then(closeEditor).catch(function () {});
    });
    $("report-close").addEventListener("click", function () { $("report").close(); });
  });
})();
