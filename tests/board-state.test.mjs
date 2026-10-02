import { createRequire } from "node:module";
import { test } from "node:test";
import assert from "node:assert/strict";

const require = createRequire(import.meta.url);
const BoardState = require("../internal/server/web/board-state.js");

const card = {
  name: "a", state: "ideas", title: "A",
  meta: { lastChanged: "2026-10-02T10:00:00Z", size: 10 },
  stalled: false, readOnly: false,
};
const board = (cards) => ({ states: ["ideas", "wip", "published"], columns: { ideas: cards, wip: [], published: [] } });

test("identical listings share a fingerprint", () => {
  assert.equal(BoardState.fingerprint(board([card])), BoardState.fingerprint(board([{ ...card }])));
  // Column order is part of the shape: same data, same states order.
  assert.equal(
    BoardState.fingerprint(board([card])),
    BoardState.fingerprint({ states: ["ideas", "wip", "published"], columns: { ideas: [card], wip: [], published: [] } })
  );
});

test("display-relevant changes alter the fingerprint", () => {
  const changes = [
    (c) => ({ ...c, name: "b" }),
    (c) => ({ ...c, title: "B" }),
    (c) => ({ ...c, state: "wip" }),
    (c) => ({ ...c, stalled: true }),
    (c) => ({ ...c, readOnly: true }),
    (c) => ({ ...c, meta: { ...c.meta, size: 11 } }),
    (c) => ({ ...c, meta: { ...c.meta, lastChanged: "2026-10-02T11:00:00Z" } }),
    () => null, // a card disappearing
  ];
  const base = BoardState.fingerprint(board([card]));
  for (const change of changes) {
    const next = change(card);
    assert.notEqual(BoardState.fingerprint(board(next ? [next] : [])), base, String(change));
  }
});

test("response noise the display ignores does not alter the fingerprint", () => {
  const a = { ...board([card]), staleness: "720h" };
  const b = { ...board([{ ...card }]), staleness: "1h" };
  assert.equal(BoardState.fingerprint(a), BoardState.fingerprint(b));
});
