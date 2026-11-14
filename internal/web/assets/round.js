"use strict";

// A party round on the puzzle page (d.party): a round played together, see
// party.go and party.js. The server's state arrives as events (Party.live,
// party.js): the common start, the clock, our result and everybody's
// progress. The board opens at the start; solving or giving up is reported,
// and so is progress (stars set, hints taken) while playing. A lost report is
// repeated with the next state. This file is what all modes share and the
// mode against each other ("versus"); coop.js and race.js add the others.
//
// PartyRound(P) is created by boot() in app.js; P is the page (see there).
function PartyRound(P) {
  const { L, d, n, $ } = P;
  const saved = P.saved;
  const party = { gaveUp: !!(saved && saved.gaveUp), snap: null, startLocal: null, live: false, solving: false, sentStars: -1, sentHints: -1 };
  let partyView = null;

  // What coop.js and race.js use of the round.
  const R = { state: party, me: partyMe, started: partyStarted, live: partyLive, post: partyPost, error: partyError, snap: partySnap, solve: partySolve };
  const coop = d.party.mode === "coop" ? Coop(P, R) : null;
  const race = d.party.mode === "clock" ? Race(P, R) : null;

  function partyMe() {
    const s = party.snap;
    return s && s.you ? s.ranking.find((e) => e.id === s.you) : null;
  }
  function partyStarted() {
    return party.startLocal !== null && Date.now() >= party.startLocal;
  }
  function partyLive() {
    const me = partyMe();
    if (race && me && (me.puzzle !== d.party.puzzle || Date.now() >= race.deadline())) return false;
    return !!me && party.snap.phase === "playing" && partyStarted() && (me.status === "playing" || me.status === "absent");
  }
  function partyPost(action, body) {
    return Party.post(d.party.code, action, { round: d.party.round, hints: P.hintCount, ...body });
  }
  function partyError(e) {
    if (partyView) partyView.error(e.message);
  }

  // The solution with every other cell crossed, shown after giving up.
  function solutionView() {
    const cells = new Uint8Array(n * n).fill(L.CROSS), auto = new Uint8Array(n * n).fill(1);
    if (d.solution) d.solution.forEach((c, r) => { cells[L.idx(d, r, c)] = L.STAR; auto[L.idx(d, r, c)] = 0; });
    return { cells, auto, marks: {} };
  }

  // [text, class] for the banner while not solved, or null.
  function partyBanner() {
    if (race) return race.banner();
    const s = party.snap;
    if (party.gaveUp) return [coop ? "Gemeinsam aufgegeben. Die Lösung ist eingeblendet." : "Aufgegeben. Die Lösung ist eingeblendet.", "what-if"];
    if (!s) return ["Verbinde mit der Party …", ""];
    const me = partyMe();
    if (!me) return ["Du spielst in dieser Runde nicht mit.", "what-if"];
    if (!partyStarted()) return [`Gleich geht es los: ${Math.max(1, Math.ceil((party.startLocal - Date.now()) / 1000))} …`, ""];
    if (s.phase !== "playing") return ["Die Runde ist vorbei.", "what-if"];
    if (coop && s.coop && s.coop.votes.length) {
      const k = s.coop.votes.length, m = s.ranking.filter((e) => e.status === "playing").length;
      return [`${k} von ${m} wollen aufgeben. Die Runde endet, wenn alle dafür sind.`, ""];
    }
    return null;
  }

  function partyTabs() {
    const open = !race && (P.done || party.gaveUp); // no solving trace against the clock
    $("tab-watch").hidden = !open;
    if (!open && P.mode === "watch") P.setMode("play");
  }

  function partySolve() {
    if (race) return race.solve();
    if (party.solving) return;
    party.solving = true;
    const eff = L.effective(d, P.marks, P.prefs.autocross);
    partyPost("solve", { stars: L.starColumns(d, eff.cells) })
      .then((r) => {
        P.elapsed = r.result.timeMs; // the server's time counts
        P.runningSince = null;
        P.hintCount = Math.max(P.hintCount, r.result.hints);
        P.renderPlay();
      })
      .catch(partyError)
      .finally(() => { party.solving = false; });
  }

  let partyTimer = null;
  function partyReport() {
    const was = party.live;
    party.live = partyLive();
    if (race) {
      if (!party.live && was) race.report(); // e.g. the time is up: the last state counts
      if (!party.live || partyTimer) return;
      partyTimer = setTimeout(() => {
        partyTimer = null;
        if (partyLive()) race.report();
      }, 400);
      return;
    }
    if (coop || !party.live || partyTimer) return;
    partyTimer = setTimeout(() => {
      partyTimer = null;
      if (!partyLive()) return;
      let stars = 0;
      for (const v of P.hyp.committed()) if (v === L.STAR) stars++; // not those of a hypothesis
      if (stars === party.sentStars && P.hintCount === party.sentHints) return;
      party.sentStars = stars;
      party.sentHints = P.hintCount;
      partyPost("progress", { count: stars }).catch(() => { party.sentStars = -1; });
    }, 700);
  }

  function partyGiveUp() {
    if (coop) return coop.vote();
    if (P.locked() || !confirm("Wirklich aufgeben? Die Lösung wird angezeigt, und du landest hinter allen, die lösen.")) return;
    party.gaveUp = true;
    P.hyp.drop();
    P.clockStop();
    P.overlay = null;
    P.message(...P.intro());
    partyPost("giveup", {}).catch(partyError);
    P.renderPlay();
  }

  function partySnap(s, at) {
    if (coop && !coop.newer(s)) return; // event stream and replies may overtake each other
    party.snap = s;
    if (s.round !== d.party.round) { // a new round: the lobby sends us on
      location.href = "/party/" + d.party.code;
      return;
    }
    party.startLocal = at + s.startInMs - s.elapsedMs;
    if (coop) return coop.snap(s);
    if (race) return race.snap(s);
    const me = partyMe();
    if (me) {
      P.hintCount = Math.max(P.hintCount, me.hints);
      if (me.status === "gaveup") party.gaveUp = true;
      if (me.status === "solved") {
        P.elapsed = me.timeMs;
        P.runningSince = null;
        if (!P.done && !party.gaveUp && d.solution) { // e.g. storage cleared
          P.hyp.drop();
          const marks = new Uint8Array(n * n);
          d.solution.forEach((c, r) => { marks[L.idx(d, r, c)] = L.STAR; });
          P.marks = marks;
          P.done = true;
        }
      } else if (s.phase === "playing" && P.done && !party.gaveUp) {
        partySolve(); // the report got lost
      } else if (s.phase === "playing" && party.gaveUp && me.status !== "gaveup") {
        partyPost("giveup", {}).catch(partyError);
      } else if (!P.done && !party.gaveUp && s.phase === "playing") {
        P.elapsed = 0;
        P.runningSince = party.startLocal;
      } else {
        P.clockStop();
      }
    }
    P.renderPlay();
  }

  function start() {
    $("party-box").hidden = false;
    $("party-lobby").href = "/party/" + d.party.code;
    $("giveup").hidden = false;
    $("giveup").onclick = partyGiveUp;
    $("reset").textContent = "Alles löschen";
    $("reset").title = coop ? "Löscht das Brett für alle; die Zeit läuft weiter" : "Löscht deine Markierungen; Zeit und Hinweise zählen weiter";
    if (coop) { // no hints when playing together
      $("hint").hidden = $("check").hidden = true;
      $("giveup").title = "Stimme fürs Aufgeben; die Runde endet, wenn alle dafür sind";
    }
    if (race) { // no hints against the clock; skipping instead of giving up
      $("hint").hidden = $("check").hidden = true;
      $("giveup").onclick = race.skip;
      $("giveup").title = "Weiter zum nächsten Puzzle, ohne dieses zu lösen";
      $("reset").title = "Löscht deine Markierungen; die Zeit läuft weiter";
      $("clock").title = "Verbleibende Zeit der Runde und deine gelösten Puzzles";
    }
    P.elapsed = 0; // the server's clock; our result's time once it is known
    if (location.hash === "#weg") history.replaceState(null, "", location.pathname);
    partyView = Party.live(d.party.code, $("party-panel"), {
      onSnap: partySnap,
      onGone() { $("party-lobby").parentElement.hidden = true; }, // the panel shows the ways on
    });
  }

  return {
    coop,
    race,
    get gaveUp() { return party.gaveUp; },
    started: partyStarted,
    live: partyLive,
    solutionView,
    banner: partyBanner,
    tabs: partyTabs,
    solve: partySolve,
    report: partyReport,
    start,
    // The round's part of renderPlay; frozen: no input at all.
    controls(frozen) {
      document.body.classList.toggle("party-wait", !party.gaveUp && !P.done && !partyStarted());
      $("giveup").disabled = frozen;
      $("reset").disabled = frozen;
      if (coop) coop.controls();
      if (race) race.controls(frozen);
    },
    // Every second: the countdown, the start, and the time left.
    tick() {
      if (!partyStarted() || party.live !== partyLive()) P.renderPlay();
    },
  };
}
