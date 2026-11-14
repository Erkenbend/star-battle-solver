"use strict";

// Hypothesis mode ("Annahme") of the puzzle page: when stuck, a star is set
// on trial and followed until it leads to a contradiction. The marks tried
// out lie as a layer (over: cell → mark) over the board they started from
// (base); the page's marks show both. root is the hypothesis' star, the first
// one set in the mode. "Widerspruch" goes back to base with a cross on root,
// "Annahme verwerfen" just back to base; solving in the mode takes the marks
// over as usual. The mode has its own undo and redo (entries {over, root}),
// so they stop where it began. Only base counts for a party round: it gives
// the stars reported (against each other, against the clock), and in coop it
// is the board everybody shares, whose changes by others keep arriving under
// the layer (rebase).
//
// Hypothesis(P) is created by boot() in app.js; P is the page (see there).
function Hypothesis(P) {
  const { L, d, n, $ } = P;
  let hyp = null; // {base, over, root, undo, redo}

  // Takes up a hypothesis saved with the progress (store in app.js).
  function restore(saved) {
    if (!saved || !saved.hyp || !Array.isArray(saved.hyp.cells) || saved.hyp.cells.length !== n * n) return;
    const base = P.marks, cells = Uint8Array.from(saved.hyp.cells);
    const r = saved.hyp.root;
    const root = Number.isInteger(r) && cells[r] === L.STAR && base[r] !== L.STAR ? r : L.hypRoot(base, base, cells, -1);
    hyp = { base, over: L.overlay(base, cells), root, undo: [], redo: [] };
    P.marks = L.compose(base, hyp.over);
  }

  function start() {
    if (P.locked() || hyp) return;
    hyp = { base: P.marks, over: new Map(), root: -1, undo: [], redo: [] };
    P.overlay = null;
    P.message(...P.intro()); // a hint shown is no longer at hand
    P.renderPlay();
  }
  function set(over, root) {
    hyp.over = over;
    hyp.root = root;
    P.marks = L.compose(hyp.base, over);
    P.overlay = null;
    P.renderPlay();
  }
  // A change of the marks in the mode: one undo step of its own.
  function change(next) {
    hyp.undo.push({ over: hyp.over, root: hyp.root });
    hyp.redo.length = 0;
    set(L.overlay(hyp.base, next), L.hypRoot(hyp.base, P.marks, next, hyp.root));
  }
  function step(from, to) {
    if (!from.length) return;
    to.push({ over: hyp.over, root: hyp.root });
    const s = from.pop();
    set(s.over, s.root);
  }
  // Leaves the mode without a trace (no rendering): marks are base again.
  function drop() {
    if (!hyp) return;
    P.marks = hyp.base;
    hyp = null;
  }
  // Ends the mode with next as an ordinary change of base (one undo step;
  // in coop sent to the server like any other).
  function end(next) {
    drop();
    P.setMarks(next);
  }
  function contradict() {
    if (P.locked() || !hyp || hyp.root < 0) return;
    const next = Uint8Array.from(hyp.base);
    next[hyp.root] = L.CROSS;
    end(next);
  }
  function discard() {
    if (P.locked() || !hyp) return;
    drop();
    P.renderPlay();
  }
  function text() {
    const who = P.coop ? " Die anderen sehen sie nicht." : P.party ? " Sie zählt nicht für deinen Stand." : "";
    if (hyp.root < 0) return "Annahme: Setze einen Stern und verfolge, wohin er führt." + who;
    const r = Math.floor(hyp.root / n), c = hyp.root % n;
    return `Annahme: Stern auf (${r},${c}). Führt sie zu einem Widerspruch, kommt dort ein Kreuz hin („Widerspruch“).` + who;
  }
  $("hyp").onclick = start;
  $("hyp-contra").onclick = contradict;
  $("hyp-drop").onclick = discard;

  return {
    get active() { return !!hyp; },
    // The undo and redo stacks of the mode (the page's own outside it).
    get undo() { return hyp.undo; },
    get redo() { return hyp.redo; },
    restore,
    change,
    undoStep: () => step(hyp.undo, hyp.redo),
    redoStep: () => step(hyp.redo, hyp.undo),
    drop,
    // Solved within the mode: the marks count as usual.
    accept: () => end(P.marks),
    // The marks that count: without the layer.
    committed: () => (hyp ? hyp.base : P.marks),
    // The shared board changed under the layer (coop); false outside the mode.
    rebase(cells) {
      if (!hyp) return false;
      hyp.base = cells;
      P.marks = L.compose(cells, hyp.over);
      return true;
    },
    // Marks the cells tried out (auto-crosses included) in view, for drawBoard.
    decorate(view, cells) {
      P.boardEl.classList.toggle("hypothesis", !!hyp);
      if (!hyp) return;
      view.marks.hyp = new Set(L.diffOps(L.effective(d, hyp.base, P.prefs.autocross).cells, cells).map(([i]) => i));
      if (hyp.root >= 0) view.marks.hyproot = new Set([hyp.root]);
    },
    // The mode's buttons; frozen: no input at all.
    controls(frozen) {
      $("hyp").hidden = !!hyp;
      $("hyp").disabled = frozen;
      $("hyp-contra").hidden = $("hyp-drop").hidden = !hyp;
      $("hyp-contra").disabled = frozen || !hyp || hyp.root < 0;
      $("hyp-drop").disabled = frozen;
    },
    text,
    // Adds the layer to the progress to store.
    save(v) {
      if (hyp) v.hyp = { cells: [...P.marks], root: hyp.root };
    },
  };
}
