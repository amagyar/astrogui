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

  return { changed: changed, dirty: dirty, savePlan: savePlan, isExternalImage: isExternalImage };
});
