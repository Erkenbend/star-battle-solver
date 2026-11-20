"use strict";

// Against the clock, a party round with many puzzles in a set time
// (d.party.mode "clock"). Each puzzle of the round has its own page
// (d.party.puzzle, from 1), the same sequence for everybody. Solving it
// (checked by the server) or skipping it moves us on: the server's state says
// which puzzle is ours, and we follow it to that page as soon as the puzzle
// is prepared. There are no hints. Our stars count when the time is up, so
// they are reported as we go, and once more at the end.
//
// Race(P, R) is created by PartyRound (round.js) with the page P and the
// round R.
function Race(P, R) {
  const { L, d, $ } = P;
  const party = R.state;
  const race = { sent: null, skipping: false };
  party.gaveUp = false;
  P.hintCount = 0;

  function raceDeadline() {
    const s = party.snap;
    return s && s.clock && party.startLocal !== null ? party.startLocal + s.clock.minutes * 60000 : Infinity;
  }
  function raceLeft() {
    const s = party.snap;
    if (!s || !s.clock) return 0;
    const total = s.clock.minutes * 60000;
    if (s.phase !== "playing") return 0;
    return Math.max(0, Math.min(total, raceDeadline() - Date.now()));
  }
  function raceClock() {
    const me = party.snap ? R.me() : null;
    let t = `⏱ noch ${P.fmtTime(Math.ceil(raceLeft() / 1000) * 1000)}`;
    if (me) t += ` · ${me.solved || 0} gelöst`;
    $("clock").textContent = t;
  }
  const puzzlesText = (k) => (k === 1 ? "1 Puzzle" : `${k} Puzzles`);
  function raceBanner() {
    const s = party.snap;
    if (!s) return ["Verbinde mit der Party …", ""];
    const me = R.me();
    if (!me) return ["Du spielst in dieser Runde nicht mit.", "what-if"];
    if (!R.started()) return [`Gleich geht es los: ${Math.max(1, Math.ceil((party.startLocal - Date.now()) / 1000))} …`, ""];
    if (s.phase !== "playing" || Date.now() >= raceDeadline()) {
      const solved = me.solved || 0;
      let t = `Die Zeit ist um! ${puzzlesText(solved)} gelöst`;
      if (me.stars) t += `, im letzten ${me.stars > 0 ? "+" : "−"}${Math.abs(me.stars)} ★`;
      return [t + ".", solved ? "good" : "what-if"];
    }
    if (me.puzzle > d.party.puzzle) {
      const how = P.done ? "Gelöst!" : "Übersprungen.";
      return me.puzzle > s.clock.sizes.length ? [`${how} Das nächste Puzzle wird noch vorbereitet …`, P.done ? "good" : ""] : [`${how} Weiter geht es …`, P.done ? "good" : ""];
    }
    if (P.done) return ["Gelöst! Weiter geht es …", "good"];
    return null;
  }
  function raceSkipButton(frozen) {
    const s = party.snap;
    const b = $("giveup");
    const lim = s && s.clock ? s.clock.skips : 0;
    b.hidden = lim === 0;
    if (lim === 0) return;
    const me = R.me();
    const left = lim < 0 ? Infinity : lim - ((me && me.skips) || 0);
    b.textContent = lim < 0 ? "Überspringen" : `Überspringen (${Math.max(0, left)} übrig)`;
    b.disabled = frozen || race.skipping || left <= 0;
  }
  function raceSkip() {
    if (P.locked() || race.skipping) return;
    race.skipping = true;
    P.renderPlay();
    R.post("skip", { puzzle: d.party.puzzle })
      .then((r) => R.snap(r.snapshot, Date.now()))
      .catch(R.error)
      .finally(() => { race.skipping = false; P.renderPlay(); });
  }
  function raceSolve() {
    if (party.solving) return;
    party.solving = true;
    const eff = L.effective(d, P.marks, P.prefs.autocross);
    R.post("solve", { puzzle: d.party.puzzle, stars: L.starColumns(d, eff.cells) })
      .then((r) => R.snap(r.snapshot, Date.now()))
      .catch(R.error)
      .finally(() => { party.solving = false; });
  }
  // Reports the cells with a star, if they changed since the last report.
  function raceReport() {
    const cells = [];
    P.hyp.committed().forEach((v, i) => { if (v === L.STAR) cells.push(i); }); // not those of a hypothesis
    const key = cells.join(",");
    if (key === race.sent) return;
    race.sent = key;
    R.post("progress", { puzzle: d.party.puzzle, cells }).catch(() => { race.sent = null; });
  }
  function raceSnap(s) {
    const me = R.me();
    if (me && s.phase === "playing" && me.puzzle !== d.party.puzzle && me.puzzle <= s.clock.sizes.length) {
      Party.goTo(`/party/${d.party.code}/round/${d.party.round}/${me.puzzle}`, true); // our puzzle now
      return;
    }
    if (me && s.phase === "playing" && me.puzzle === d.party.puzzle && P.done) R.solve(); // the report got lost
    P.renderPlay();
  }

  return {
    deadline: raceDeadline,
    clock: raceClock,
    banner: raceBanner,
    controls: raceSkipButton,
    skip: raceSkip,
    solve: raceSolve,
    report: raceReport,
    snap: raceSnap,
  };
}
