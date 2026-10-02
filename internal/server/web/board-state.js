// Pure board-state helpers shared by the interface and Node tests.
(function (root, factory) {
  if (typeof module === "object" && module.exports) module.exports = factory();
  else root.ASTROGUI_BOARD_STATE = factory();
})(typeof self !== "undefined" ? self : this, function () {
  "use strict";

  // fingerprint summarizes a board listing by the fields a card's display
  // depends on: identity, state, title, modification time, size, and the
  // stalled and read-only flags. Two responses with the same fingerprint
  // leave the board untouched, so an unchanged refresh cannot interrupt the
  // user. A body edit that keeps its byte size still moves the modification
  // time, so no display-relevant change can slip through.
  function fingerprint(data) {
    var parts = [];
    (data.states || []).forEach(function (st) {
      var cards = (data.columns && data.columns[st]) || [];
      parts.push("#" + st + ":" + cards.length);
      cards.forEach(function (card) {
        var meta = card.meta || {};
        parts.push([
          card.name, card.state, card.title,
          meta.lastChanged, meta.size,
          card.stalled ? 1 : 0, card.readOnly ? 1 : 0,
        ].join("|"));
      });
    });
    return parts.join("\n");
  }

  return { fingerprint: fingerprint };
});
