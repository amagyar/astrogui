// Pure editor-state helpers shared by the interface and Node tests.
(function (root, factory) {
  if (typeof module === "object" && module.exports) module.exports = factory();
  else root.ASTROGUI_EDITOR_STATE = factory();
})(typeof self !== "undefined" ? self : this, function () {
  "use strict";

  var fields = ["source", "title", "date", "raw"];

  function changed(current, baseline) {
    var result = {};
    fields.forEach(function (field) { result[field] = current[field] !== baseline[field]; });
    return result;
  }

  function dirty(changes) {
    return fields.some(function (field) { return !!changes[field]; });
  }

  function savePlan(readOnly, changes) {
    if (readOnly) return changes.source ? ["body"] : [];
    if (changes.raw) {
      var rawPlan = ["raw"];
      if (changes.title || changes.date) rawPlan.push("fields");
      return rawPlan;
    }
    var plan = [];
    if (changes.source) plan.push("body");
    if (changes.title || changes.date) plan.push("fields");
    return plan;
  }

  function isExternalImage(ref) {
    return /^(https?:)?\/\//i.test(String(ref || ""));
  }

  // blockedImageReason reports why an image reference must not be fetched
  // for the preview, or null when it may be hydrated through the asset API:
  // external references are never requested, and a loose (read-only) post
  // holds no assets of its own to fetch.
  function blockedImageReason(ref, readOnly) {
    if (isExternalImage(ref)) return "External image not loaded: " + ref;
    if (readOnly) return "Image not loaded: loose posts hold no assets of their own: " + ref;
    return null;
  }

  return { changed: changed, dirty: dirty, savePlan: savePlan, isExternalImage: isExternalImage, blockedImageReason: blockedImageReason };
});
