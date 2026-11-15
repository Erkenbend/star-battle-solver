"use strict";

// Coop mode of a party round (d.party.mode "coop"): one board for all. The
// server keeps the board and orders everybody's changes
// (internal/party/coop.go). We show its board plus our own changes not yet
// confirmed (pending), so input never waits for the network. Changes are
// operations [num, i, from, to] (L.applyOps): a change based on an outdated
// view of a cell is rejected instead of overwriting what somebody else did.
// One request at a time keeps our operations in order; repeating one after a
// lost reply is harmless, the server skips numbers it has seen (acked).
//
// Coop(P, R) is created by PartyRound (round.js) with the page P and the
// round R. app.js sends the page's changes through commit and undo.
function Coop(P, R) {
  const { L, n, $ } = P;
  const party = R.state;
  const coop = { server: new Uint8Array(n * n), version: -1, pending: [], nextNum: 0, ready: false, sending: false, voted: false, lost: new Set() };
  // The board comes from the server, nothing from storage.
  P.marks = new Uint8Array(n * n);
  P.done = party.gaveUp = false;
  P.hintCount = 0;

  function show() {
    showBoard(L.applyOps(coop.server, coop.pending).cells);
    P.renderPlay();
  }
  // The shared board as we see it; a hypothesis stays on top of it.
  function showBoard(cells) {
    if (!P.hyp.rebase(cells)) P.marks = cells;
  }

  // ops: [i, from, to] as seen on the board now; one undo step, or with
  // merge part of the last one (a cell changed twice keeps one op).
  function commit(ops, undoable = true, merge = false) {
    P.overlay = null;
    if (ops.length && undoable) {
      P.undoStack.push(merge && P.undoStack.length ? mergeOps(P.undoStack.pop(), ops) : ops);
      P.redoStack.length = 0;
    }
    for (const op of ops) coop.pending.push([++coop.nextNum, ...op]);
    show();
    flush();
  }

  function mergeOps(a, b) {
    const m = new Map(a.map(([i, from, to]) => [i, [from, to]]));
    for (const [i, from, to] of b) m.set(i, [m.has(i) ? m.get(i)[0] : from, to]);
    return [...m].filter(([, [from, to]]) => from !== to).map(([i, [from, to]]) => [i, from, to]);
  }

  // Undo and redo take back only our own changes: a cell somebody else
  // changed since is left alone (and blinks). Both stacks hold operations as
  // they were applied, so taking one back means applying it reversed, and
  // what that applies goes onto the other stack.
  function undo(from, to) {
    const marks = P.marks;
    const ops = from.pop().map(([i, a, b]) => [i, b, a]);
    const ok = ops.filter(([i, a]) => marks[i] === a);
    flash(ops.filter(([i, a]) => marks[i] !== a).map(([i]) => i), "conflict");
    if (ok.length) to.push(ok);
    commit(ok, false);
  }

  function flush() {
    if (coop.sending || !coop.pending.length || !R.live()) return;
    coop.sending = true;
    R.post("mark", { ops: coop.pending.slice() })
      .then((r) => {
        coop.sending = false;
        R.snap(r.snapshot, Date.now());
        flush();
      })
      .catch((e) => {
        coop.sending = false;
        if (e.status === 0) { // no connection: try again
          setTimeout(flush, 1500);
          return;
        }
        coop.pending = []; // e.g. the round is over: the server's board counts
        R.error(e);
        show();
      });
  }

  // A new state from the server: its board replaces ours, our confirmed
  // operations drop out and the rest is replayed on top.
  function snap(s) {
    const c = s.coop;
    if (c) {
      const first = !coop.ready;
      coop.server = L.decodeBoard(c.board);
      if (first) coop.nextNum = Math.max(coop.nextNum, c.acked);
      coop.ready = true;
      // Our own changes that did not make it (somebody was faster) blink red,
      // once each: as soon as the new board shows they cannot apply any more,
      // or when the server confirms that (only a cell's last change counts).
      const last = new Map();
      for (const op of coop.pending) if (op[0] <= c.acked) last.set(op[1], op);
      const lostOps = [...last.values()].filter((op) => coop.server[op[1]] !== op[3]);
      coop.pending = coop.pending.filter((op) => op[0] > c.acked);
      const doomed = new Set(L.applyOps(coop.server, coop.pending).rejected);
      lostOps.push(...coop.pending.filter((op) => doomed.has(op[0])));
      const lost = [];
      for (const op of lostOps) {
        if (!coop.lost.has(op[0])) lost.push(op[1]);
        coop.lost.add(op[0]);
      }
      coop.voted = !!s.you && c.votes.includes(s.you);
      if (c.solved) { // by whoever: a hypothesis of ours is moot
        P.done = true;
        P.hyp.drop();
      }
      if (!P.drag) { // a drag in progress keeps its view until it ends
        const before = P.marks;
        showBoard(L.applyOps(coop.server, coop.pending).cells);
        if (!first) flash(L.diffOps(before, P.marks).map(([i]) => i).filter((i) => !lost.includes(i)), "remote");
      }
      flash(lost, "conflict");
    }
    const end = s.ranking.length ? s.ranking[0].timeMs : 0;
    if (c && c.solved) {
      P.elapsed = end;
      P.runningSince = null;
    } else if (s.phase === "playing") {
      P.elapsed = 0;
      P.runningSince = party.startLocal;
    } else { // given up together
      party.gaveUp = true;
      P.elapsed = end;
      P.runningSince = null;
    }
    P.renderPlay();
    flush();
  }

  function vote() {
    if (P.locked()) return;
    if (!coop.voted && !confirm("Für Aufgeben stimmen? Die Runde endet, wenn alle dafür sind. Bis dahin geht es weiter.")) return;
    coop.voted = !coop.voted;
    P.renderPlay();
    R.post("vote", { giveUp: coop.voted })
      .then((r) => R.snap(r.snapshot, Date.now()))
      .catch(R.error);
  }

  // Lets cells blink briefly: "remote" for changes by others, "conflict"
  // for our own changes that lost.
  function flash(cells, cls) {
    for (const i of cells) {
      const e = P.cellEls[i];
      e.classList.remove(cls);
      void e.offsetWidth; // restarts the animation
      e.classList.add(cls);
      clearTimeout(e[cls]);
      e[cls] = setTimeout(() => e.classList.remove(cls), 1200);
    }
  }

  return {
    commit,
    undo,
    snap,
    vote,
    // Whether s is newer than the last state: event stream and replies may
    // overtake each other.
    newer(s) {
      if (s.version < coop.version) return false;
      coop.version = s.version;
      return true;
    },
    controls() {
      $("giveup").textContent = coop.voted ? "Doch nicht aufgeben" : "Aufgeben";
      $("giveup").classList.toggle("voted", coop.voted);
    },
  };
}
