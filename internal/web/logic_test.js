// Tests for the page logic in assets/app.js. Run by TestJSLogic (web_test.go):
//   node logic_test.js data1.json data2.json ...
"use strict";
const assert = require("node:assert/strict");
const L = require("./assets/app.js");

const files = process.argv.slice(2);
assert.ok(files.length > 0, "no data files given");

function solutionCells(d) {
  const cells = new Uint8Array(d.n * d.n).fill(L.CROSS);
  d.solution.forEach((c, r) => (cells[L.idx(d, r, c)] = L.STAR));
  return cells;
}

for (const f of files) {
  const d = JSON.parse(require("node:fs").readFileSync(f, "utf8"));
  const name = `${f} (${d.n}×${d.n})`;

  // Replaying the trace ends in the solution.
  const prefix = L.prefixStates(d, d.steps);
  assert.equal(prefix.length, d.steps.length + 1);
  assert.deepEqual(prefix.at(-1), solutionCells(d), name + ": trace does not end in the solution");
  assert.ok(L.solved(d, prefix.at(-1)), name + ": final state not solved");
  assert.equal(L.problems(d, prefix.at(-1)).length, 0);
  assert.equal(L.mistakes(d, prefix.at(-1)).length, 0);
  // The solved board in the party server's format is the solution.
  assert.deepEqual(L.starColumns(d, prefix.at(-1)), d.solution);
  assert.deepEqual(L.starColumns(d, prefix[0]), new Array(d.n).fill(-1));
  // No state along the way breaks a rule or disagrees with the solution.
  prefix.forEach((s, k) => {
    assert.equal(L.problems(d, s).length, 0, `${name}: problem after step ${k}`);
    assert.equal(L.mistakes(d, s).length, 0, `${name}: mistake after step ${k}`);
  });

  // Following only the hints solves the puzzle, with and without auto-crosses.
  for (const auto of [false, true]) {
    let marks = new Uint8Array(d.n * d.n);
    let guard = 0;
    for (;;) {
      const eff = L.effective(d, marks, auto);
      if (L.solved(d, eff.cells)) break;
      const h = L.nextHint(d, eff.cells);
      assert.ok(h, `${name}: no hint before solved (auto=${auto})`);
      for (const [r, c, s] of h.marks) marks[L.idx(d, r, c)] = s ? L.STAR : L.CROSS;
      assert.equal(L.mistakes(d, L.effective(d, marks, auto).cells).length, 0, name + ": hint led to a mistake");
      assert.ok(++guard <= d.steps.length, name + ": hints do not make progress");
    }
  }

  // A hint on a partially solved board is the first step with something new.
  const k = Math.floor(d.steps.length / 2);
  const h = L.nextHint(d, prefix[k]);
  assert.ok(h && h.step >= k, name + ": hint goes back in the trace");
  for (let j = k; j < h.step; j++) {
    assert.ok(d.steps[j].marks.every(([r, c]) => prefix[k][L.idx(d, r, c)] !== L.FREE));
  }

  // Every hypothesis chain ends in a contradiction that names cells on the board.
  for (const st of d.steps) {
    if (!st.hypothesis) continue;
    assert.ok(st.hypothesis.steps.length > 0, name + ": empty chain");
    assert.equal(st.hypothesis.steps[0].marks.find((m) => m[2] === 1) !== undefined, true, name + ": chain does not start with a star");
    assert.ok(st.hypothesis.contradiction.length > 0);
  }

  // No region letter is left as plain text once swatches replace them.
  const texts = [];
  for (const st of d.steps) {
    texts.push(st.reason);
    if (st.hypothesis) texts.push(st.hypothesis.contradiction, ...st.hypothesis.steps.map((h) => h.reason));
  }
  for (const t of texts) {
    for (const seg of L.colorSegments(d, t)) {
      if (typeof seg === "string") assert.doesNotMatch(seg, /\bFarben? [A-Z]\b/, `${name}: letter left in "${t}"`);
    }
  }

  // Every explanation that names a unit or cell is understood by refs.
  for (const st of d.steps) {
    if (/(Zeile|Spalte|Farbe) [0-9A-Z]/.test(st.reason)) {
      const r = L.refs(d, st.reason);
      assert.ok(r.units.size > 0, `${name}: no unit found in "${st.reason}"`);
    }
  }
}

// Small hand-made checks on a 5×5 layout.
{
  const d = {
    n: 5,
    letters: "ABCDE",
    regions: [
      [3, 1, 1, 1, 1],
      [3, 3, 1, 2, 2],
      [3, 3, 1, 0, 0],
      [4, 3, 3, 0, 0],
      [4, 0, 0, 0, 0],
    ],
    solution: [2, 4, 1, 3, 0],
    steps: [],
  };
  const at = (r, c) => L.idx(d, r, c);

  // Auto-crosses: a star on (0,0) crosses row, column, region D and neighbors.
  const marks = new Uint8Array(25);
  marks[at(0, 0)] = L.STAR;
  const eff = L.effective(d, marks, true);
  assert.equal(eff.cells[at(0, 4)], L.CROSS);
  assert.equal(eff.cells[at(4, 0)], L.CROSS);
  assert.equal(eff.cells[at(1, 1)], L.CROSS); // neighbor and region D
  assert.equal(eff.cells[at(3, 2)], L.CROSS); // region D
  assert.equal(eff.cells[at(2, 3)], L.FREE);
  assert.ok(eff.auto[at(0, 4)] && !eff.auto[at(0, 0)]);
  assert.equal(L.effective(d, marks, false).cells[at(0, 4)], L.FREE);

  // Problems: two stars in a row, touching stars, a unit without room.
  const two = new Uint8Array(25);
  two[at(0, 0)] = two[at(0, 3)] = L.STAR;
  assert.ok(L.problems(d, two).some((p) => p.msg === "Zeile 0 hat 2 Sterne"));
  const touch = new Uint8Array(25);
  touch[at(1, 1)] = touch[at(2, 2)] = L.STAR;
  assert.ok(L.problems(d, touch).some((p) => p.msg === "Sterne auf (1,1) und (2,2) berühren sich"));
  const full = new Uint8Array(25);
  for (let c = 0; c < 5; c++) full[at(4, c)] = L.CROSS;
  assert.ok(L.problems(d, full).some((p) => p.msg === "Zeile 4 hat keinen Platz mehr für einen Stern"));
  assert.ok(L.problems(d, full).some((p) => p.msg === "Farbe E hat keinen Platz mehr für einen Stern") === false);

  // Mistakes against the solution (row 0 has its star in column 2).
  const wrong = new Uint8Array(25);
  wrong[at(0, 0)] = L.STAR;
  wrong[at(0, 2)] = L.CROSS;
  assert.deepEqual(L.mistakes(d, wrong), [at(0, 0), at(0, 2)]);

  // Explanations: units, ranges, colors and cells.
  const rows = (r) => [0, 1, 2, 3, 4].map((c) => at(r, c));
  const has = (set, cells) => cells.every((i) => set.has(i));
  let r = L.refs(d, "Zeilen 3 und 4 haben nur noch freie Felder in Spalten 0 und 1 → alle anderen Felder der Spalten 0 und 1 X");
  assert.ok(has(r.units, [...rows(3), ...rows(4), at(0, 0), at(0, 1)]));
  assert.ok(!r.units.has(at(0, 2)));
  r = L.refs(d, "Zeilen 1 bis 3 haben nur noch freie Felder der Farben B, C und D");
  assert.ok(has(r.units, [...rows(1), ...rows(2), ...rows(3), at(0, 1), at(1, 3), at(0, 0)]));
  assert.ok(!r.units.has(at(4, 4))); // region A, row 4: not named
  r = L.refs(d, "Angenommen, auf (2,3) ist ein Stern → X in Zeile 2, Spalte 3, Farbe A und Nachbarschaft");
  assert.ok(r.cells.has(at(2, 3)) && r.cells.size === 1);
  assert.ok(has(r.units, [...rows(2), at(4, 3), at(4, 1)]));
  assert.ok(!r.units.has(at(0, 0))); // "und Nachbarschaft" is not region N
  r = L.refs(d, "Farbe E hat nur noch freie Felder in Spalte 0 → alle anderen Felder der Spalte 0 X");
  assert.ok(has(r.units, [at(3, 0), at(4, 0), at(0, 0)]) && !r.units.has(at(4, 1)));

  assert.deepEqual(L.ruleLabel({ rule: "Confinement", level: 2 }), { name: "Eingesperrt", cls: "l1" });
  assert.equal(L.ruleLabel({ rule: "Hypothesis", level: 100 }).name, "Annahme");
  assert.equal(L.ruleLabel({ rule: "Hypothesis", level: 101 }).name, "Raten");
}

console.log(`ok: ${files.length} puzzles`);

// Drag painting.
{
  const d = { n: 5 };
  assert.deepEqual(L.line(d, 0, 4), [0, 1, 2, 3, 4]);
  assert.deepEqual(L.line(d, 0, 24), [0, 6, 12, 18, 24]); // diagonal
  assert.deepEqual(L.line(d, 7, 7), [7]);
  assert.deepEqual(L.line(d, 0, 12).length, 3); // (0,0) → (2,2) via (1,1)
  const m = new Uint8Array(25);
  m[2] = L.STAR;
  assert.equal(L.paint(m, [0, 1, 2, 3], false), true);
  assert.deepEqual([...m.slice(0, 5)], [1, 1, 2, 1, 0]); // star kept
  assert.equal(L.paint(m, [0, 2, 4], true), true);
  assert.deepEqual([...m.slice(0, 5)], [0, 1, 2, 1, 0]);
  assert.equal(L.paint(m, [4], true), false);

  // Fat-finger guard snaps to row or column (cell size 10px, start at (2,2) = 12).
  const snap = (dx, dy) => L.snapTarget(d, 12, dx, dy, 10);
  assert.equal(snap(3, 2), 12); // jitter stays on the start cell
  assert.equal(snap(20, 8), 14); // sideways wobble ignored: row
  assert.equal(snap(-7, -18), 2); // column upwards
  assert.equal(snap(18, 16), 14); // no diagonals: the larger offset wins
  assert.equal(snap(-12, 14), 17);
  assert.equal(snap(80, 3), 14); // clipped at the board edge
  assert.equal(snap(-3, -80), 2);
}
console.log("drag ok");

// Color swatches instead of region letters.
{
  const d = { letters: "ABCDE" };
  assert.deepEqual(L.colorSegments(d, "Farben B und C liegen nur noch in Zeile 0"), ["Farben ", { region: 1 }, " und ", { region: 2 }, " liegen nur noch in Zeile 0"]);
  assert.deepEqual(L.colorSegments(d, "Farben A, B und D"), ["Farben ", { region: 0 }, ", ", { region: 1 }, " und ", { region: 3 }]);
  assert.deepEqual(L.colorSegments(d, "X in Zeile 2, Spalte 3, Farbe E und Nachbarschaft"), ["X in Zeile 2, Spalte 3, Farbe ", { region: 4 }, " und Nachbarschaft"]);
  assert.deepEqual(L.colorSegments(d, "Zeile 3 hat 2 Sterne"), ["Zeile 3 hat 2 Sterne"]);
  assert.deepEqual(L.colorSegments(d, ""), []);
}
console.log("swatches ok");

// Coop: operations on the shared board, as the server applies them.
{
  const { FREE: F, CROSS: X, STAR: S } = L;
  const before = Uint8Array.from([F, X, S, F]);
  const after = Uint8Array.from([X, X, F, S]);
  const ops = L.diffOps(before, after);
  assert.deepEqual(ops, [[0, F, X], [2, S, F], [3, F, S]]);
  const numbered = ops.map((o, k) => [k + 1, ...o]);
  assert.deepEqual(L.applyOps(before, numbered), { cells: after, rejected: [] });
  assert.deepEqual([...before], [F, X, S, F], "applyOps must not change its input");

  // Rebase: my pending ops on a newer server board where somebody else set
  // a star on cell 0 and the same star on cell 3.
  const server = Uint8Array.from([S, X, S, S]);
  const r = L.applyOps(server, numbered);
  assert.deepEqual([...r.cells], [S, X, F, S]); // their star stays, mine on 3 agrees
  assert.deepEqual(r.rejected, [1]);

  assert.deepEqual([...L.decodeBoard("0120")], [F, X, S, F]);
}
console.log("coop ops ok");

// Library: the next unsolved puzzle after the current one, by size and
// number, wrapping around.
{
  const entries = [{ id: "c", nr: 1, n: 6 }, { id: "b", nr: 2, n: 5 }, { id: "d", nr: 1, n: 7 }, { id: "a", nr: 1, n: 5 }];
  assert.equal(L.nextUnsolved(entries, {}, "a").id, "b");
  assert.equal(L.nextUnsolved(entries, { b: {}, c: {} }, "a").id, "d");
  assert.equal(L.nextUnsolved(entries, { a: {} }, "d").id, "b"); // wraps, skips solved
  assert.equal(L.nextUnsolved(entries, { a: {}, b: {}, c: {} }, "d"), null); // the current one does not count
  assert.equal(L.nextUnsolved(entries, null, "b").id, "c");
  // Region colors: every region gets one, and n ≤ 12 stays in the palette.
  for (const n of [4, 9, 14]) {
    for (let r = 0; r < n; r++) assert.equal(L.regionHSL(n, r).length, 3);
  }
}
console.log("library ok");

// Star symbol: stored catalog entries become drawable specs; anything broken,
// and images on a page without the server, fall back to the star.
{
  const star = L.DEFAULT_MARK;
  for (const bad of [null, undefined, 42, "star", {}, { kind: "gif" }, { kind: "svg" },
    { kind: "svg", path: 'M0 0"/><script>' }, { kind: "glyph", glyph: "" }, { kind: "glyph", glyph: "x".repeat(30) },
    { kind: "sprite", url: "https://example.com/a.svg" }, { kind: "sprite", url: "/symbole/../a.svg" }]) {
    assert.equal(L.markSpec(bad, { sprites: true }), star, JSON.stringify(bad));
  }
  const heart = L.markSpec({ id: "heart", kind: "svg", path: "M50 90C20 68Z", scale: 5 });
  assert.deepEqual([heart.kind, heart.tint, heart.scale], ["svg", true, 1]);
  const blossom = L.markSpec({ id: "blossom", kind: "glyph", glyph: "✿︎", scale: 0.8 });
  assert.deepEqual([blossom.kind, blossom.tint, blossom.scale, blossom.glyph], ["glyph", true, 0.8, "✿︎"]);
  assert.equal(L.markSpec({ id: "cat-face", kind: "emoji", glyph: "🐱️" }).tint, false);
  const sprite = { id: "hop", kind: "sprite", url: "/symbole/hop.svg?v=abc123", frames: 8, fps: 8 };
  const s = L.markSpec(sprite, { sprites: true });
  assert.deepEqual([s.kind, s.tint, s.url, s.frames, s.fps], ["sprite", false, sprite.url, 8, 8]);
  assert.deepEqual(s.clips, [{ start: 0, frames: 8 }]); // without clips: all frames
  assert.equal(L.markSpec(sprite, { sprites: false }), star); // page opened as a file
  assert.equal(L.markSpec(sprite), star);
  // Frames without a usable speed: a still image of the first frame.
  const still = L.markSpec({ ...sprite, fps: 0 }, { sprites: true });
  assert.deepEqual([still.frames, still.fps], [1, 0]);
  // Clips are taken over if all fit the frames, else all frames are one.
  const clips = [{ name: "a", start: 1, frames: 3 }, { start: 4, frames: 4 }];
  assert.deepEqual(L.markSpec({ ...sprite, clips }, { sprites: true }).clips, [{ start: 1, frames: 3 }, { start: 4, frames: 4 }]);
  for (const bad of [[], "x", [{ start: 5, frames: 4 }], [{ start: 0, frames: 0 }], [{ start: -1, frames: 2 }], [null], [{ start: 0.5, frames: 2 }]]) {
    assert.deepEqual(L.markSpec({ ...sprite, clips: bad }, { sprites: true }).clips, [{ start: 0, frames: 8 }], JSON.stringify(bad));
  }
  // A clip plays for about three seconds in whole runs, at least one; still images never.
  assert.equal(L.markRuns({ frames: 8 }, 8), 3); // 1 s per run
  assert.equal(L.markRuns({ frames: 40 }, 4), 1);
  assert.equal(L.markRuns({ frames: 4 }, 30), 23);
  assert.equal(L.markRuns({ frames: 1 }, 0), 0);
  // Clips are picked at random, never the last one twice if there are others.
  const seq = (xs) => { let i = 0; return () => xs[i++ % xs.length]; };
  assert.equal(L.pickClip(1, 0, seq([0.9])), 0);
  assert.deepEqual([0, 0.4, 0.6, 0.99].map((r) => L.pickClip(3, -1, seq([r]))), [0, 1, 1, 2]);
  assert.deepEqual([0, 0.4, 0.6, 0.99].map((r) => L.pickClip(3, 1, seq([r]))), [0, 0, 2, 2]);
  assert.deepEqual([0, 0.99].map((r) => L.pickClip(2, 0, seq([r]))), [1, 1]);
  assert.equal(L.pickClip(3, 7, seq([0.99])), 2); // a last pick of another symbol
  for (let prev = 0; prev < 4; prev++) {
    const seen = new Set();
    for (let r = 0; r < 1; r += 0.01) seen.add(L.pickClip(4, prev, () => r));
    assert.deepEqual([...seen].sort(), [0, 1, 2, 3].filter((k) => k !== prev));
  }
}
console.log("symbols ok");

// Play options: defaults without storage, keys mapped back for other tabs.
{
  assert.equal(L.pref("autocross"), false);
  assert.equal(L.pref("straight"), false); // no touch screen in node
  assert.equal(L.pref("showProblems"), true);
  assert.equal(L.prefOfKey("starbattle.showProblems"), "showProblems");
  assert.equal(L.prefOfKey("starbattle.symbol"), null);
  assert.equal(L.prefOfKey(null), null);
}
console.log("options ok");
