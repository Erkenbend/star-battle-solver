"use strict";

// Help while solving ("Hinweis" and "Prüfen", where P.K.help allows them).
// A hint is the next step of the solver's way that is not on the board yet,
// with its explanation, to apply or hide again; it counts once, until the
// puzzle is solved. Checking is free and only says whether the marks so far
// agree with the solution. Showing where they don't costs one hint per
// wrongly set field (auto-crosses next to a wrong star are shown for free);
// fields already paid for in their current state are not charged again,
// nor are wrong stars a party round has shown (they cost a life).
//
// Help(P) is created by boot() in app.js; P is the page (see there).
function Help(P) {
  const { L, d, el, elr, chip, message } = P;
  const saved = P.saved;
  const paid = new Set(saved && Array.isArray(saved.paid) ? saved.paid : []); // "i:mark" of wrong fields already shown

  // The hint button toggles: a second click hides the hint again.
  function toggleHint() {
    if (P.overlay && P.overlay.hint) {
      P.overlay = null;
      message(...P.intro());
      P.renderPlay();
    } else showHint();
  }

  function showHint() {
    const eff = L.effective(d, P.marks, P.prefs.autocross);
    const wrong = L.mistakes(d, eff.cells);
    if (wrong.length) {
      message(el("p", "", "Etwas passt nicht zur Lösung. Erst das korrigieren, dann gibt es den nächsten Hinweis."), showMistakesButton());
      return;
    }
    const h = L.nextHint(d, eff.cells);
    if (!h) {
      message(el("p", "", L.solved(d, eff.cells) ? "Schon gelöst." : "Hier kommt der Solver auch nicht weiter."));
      return;
    }
    const step = d.steps[h.step];
    const refs = L.refCells(d, step.refs);
    if (!P.done) P.hintCount++;
    P.overlay = { hint: new Set(h.marks.map(([r, c]) => L.idx(d, r, c))), ref: refs.units, refcell: refs.cells };
    const head = el("p", "");
    head.append(chip(step), " ", el("span", "muted", `Schritt ${h.step + 1} des Lösungswegs`));
    const reason = elr("p", "", step.reason, step.refs || {});
    const nodes = [head, reason];
    if (step.hypothesis) {
      const det = el("details");
      det.append(el("summary", "", "Gedankengang anzeigen"));
      const ol = el("ol");
      for (const s of step.hypothesis.steps) ol.append(elr("li", "", s.reason, s.refs || {}));
      det.append(ol, elr("p", "contra", "⇒ Widerspruch: " + step.hypothesis.contradiction, step.hypothesis.refs || {}));
      nodes.push(det);
    }
    const apply = el("button", "primary", "Anwenden");
    apply.onclick = () => {
      const next = Uint8Array.from(P.marks);
      for (const [r, c, s] of h.marks) next[L.idx(d, r, c)] = s ? L.STAR : L.CROSS;
      P.setMarks(next);
      message(el("p", "muted", "Angewendet. Für den nächsten Schritt wieder „Hinweis“."));
    };
    const a = el("div", "actions");
    a.append(apply);
    nodes.push(a);
    message(...nodes);
    P.renderPlay();
  }

  function checkMistakes() {
    if (!d.solution) {
      message(el("p", "", "Für dieses Puzzle ist keine eindeutige Lösung bekannt."));
      return;
    }
    P.overlay = null;
    const eff = L.effective(d, P.marks, P.prefs.autocross);
    if (!L.mistakes(d, eff.cells).length) message(el("p", "", L.solved(d, eff.cells) ? "Gelöst." : "Bisher alles richtig."));
    else message(el("p", "", "Da stimmt etwas nicht: mindestens ein Feld passt nicht zur Lösung."), showMistakesButton());
    P.renderPlay();
  }

  function showMistakesButton() {
    const b = el("button", "", "Fehler zeigen");
    b.title = "Kostet einen Hinweis pro falsch gesetztem Feld";
    b.onclick = showMistakes;
    const a = el("div", "actions");
    a.append(b, " ", el("span", "muted", "kostet 1 Hinweis pro falschem Feld"));
    return a;
  }

  function showMistakes() {
    const marks = P.marks;
    const eff = L.effective(d, marks, P.prefs.autocross);
    const wrong = L.mistakes(d, eff.cells);
    let cost = 0;
    const known = P.round ? P.round.wrongStars() : new Set(); // shown already, paid with a life
    for (const i of L.mistakes(d, marks)) {
      const k = i + ":" + marks[i];
      if (known.has(i)) continue;
      if (!paid.has(k)) { paid.add(k); cost++; }
    }
    if (!P.done) P.hintCount += cost;
    P.overlay = { bad: new Set(wrong) };
    const what = `${wrong.length === 1 ? "Ein Feld ist" : wrong.length + " Felder sind"} falsch (rot markiert).`;
    message(el("p", "", cost ? `${what} ${L.hintsText(cost)} angerechnet.` : what));
    P.renderPlay();
  }

  return {
    toggleHint,
    checkMistakes,
    paid: () => [...paid], // kept with the progress
    reset: () => paid.clear(), // starting anew
  };
}
