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
