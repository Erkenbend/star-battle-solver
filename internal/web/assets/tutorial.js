"use strict";

// The tutorial (/anleitung, d.tutorial, see tutorial.go) on the puzzle page:
// a card below the board (beside it on wide screens) guides through the
// steps, marking the cells each one asks for; the player's moves decide
// which step is shown. Once solved,
// a tour points at the page's buttons one by one, then the player goes on to
// the page they came from.
//
// Tutorial(P) is created by boot() in app.js; P is the page (see there).
// app.js calls draw with the board after every change.
function Tutorial(P) {
  const { L, d, $, el, elr } = P;
  const tut = { step: -1, tour: null };
  try { L.tutorialDue(localStorage); } catch { /* storage blocked */ } // shown: not again by itself
  document.body.classList.add("tutorial-mode");
  $("clock").hidden = true;

  function tutLeave() {
    let from = new URLSearchParams(location.search).get("weiter");
    if (from == null) { // the "?" link: back to where it was clicked
      try {
        const u = new URL(document.referrer);
        if (u.origin === location.origin && u.pathname !== location.pathname) from = u.pathname + u.search;
      } catch { /* no referrer */ }
    }
    location.href = L.returnPath(from);
  }
  function tutDraw(cells) {
    const steps = d.tutorial.steps;
    const k = L.tutorialStep(d, steps, cells);
    const goal = new Set();
    if (k < steps.length) {
      for (const [list, v] of [[steps[k].stars, L.STAR], [steps[k].crosses, L.CROSS]]) {
        for (const [r, c] of list || []) if (cells[L.idx(d, r, c)] !== v) goal.add(L.idx(d, r, c));
      }
    }
    P.cellEls.forEach((e, i) => e.classList.toggle("goal", goal.has(i)));
    if (k === tut.step) return;
    tut.step = k;
    const skip = el("button", "quiet", "Überspringen");
    skip.onclick = tutLeave;
    const head = el("div", "head");
    if (k < steps.length) {
      head.append(el("span", null, `Anleitung · Schritt ${k + 1} von ${steps.length}`), skip);
      $("tutorial").replaceChildren(head, el("strong", null, steps[k].action), elr("p", null, steps[k].text));
    } else {
      const go = el("button", "primary", "Weiter");
      go.onclick = () => tourShow(0);
      const actions = el("div", "actions");
      actions.append(go);
      head.append(el("span", null, "Anleitung"), skip);
      $("tutorial").replaceChildren(head, el("strong", null, "Gelöst!"), el("p", null, "Zum Schluss ein kurzer Blick auf die Knöpfe."), actions);
    }
    $("tutorial").hidden = false;
  }

  // The tour: a dimmed page with a hole around the element of stop k and a
  // tip next to it. The page below takes no clicks meanwhile.
  function tourShow(k) {
    const stops = d.tutorial.tour;
    const target = k < stops.length ? $(stops[k].target) : null;
    if (k < stops.length && (!target || !target.getClientRects().length)) return tourShow(k + 1); // not on this page
    if (!target) return tutLeave();
    if (P.mode !== "play") P.setMode("play");
    if (!tut.tour) {
      const block = el("div", "tour-block"), spot = el("div", "tour-spot"), tip = el("div", "tour-tip");
      tip.setAttribute("role", "dialog");
      tip.setAttribute("aria-live", "polite");
      document.body.append(block, spot, tip);
      tut.tour = { block, spot, tip, target: null };
      const place = () => tut.tour && tourPlace();
      window.addEventListener("resize", place);
      window.addEventListener("scroll", place, { passive: true });
      document.addEventListener("keydown", (ev) => {
        if (ev.key === "Escape" && tut.tour) tutLeave();
      });
    }
    const last = k === stops.length - 1;
    const ok = el("button", "primary", last ? "Fertig" : "OK");
    ok.onclick = () => tourShow(k + 1);
    const skip = el("button", "quiet", "Überspringen");
    skip.onclick = tutLeave;
    const actions = el("div", "actions");
    actions.append(el("span", "muted", `${k + 1}/${stops.length}`), ...(last ? [] : [skip]), ok);
    tut.tour.tip.replaceChildren(el("p", null, stops[k].text), actions);
    if (tut.tour.target) tut.tour.target.classList.remove("tour-target");
    target.classList.add("tour-target");
    tut.tour.target = target;
    target.scrollIntoView({ block: "nearest" });
    tourPlace();
    ok.focus({ preventScroll: true });
  }
  function tourPlace() {
    const { spot, tip, target } = tut.tour;
    const r = target.getBoundingClientRect(), pad = 4, gap = 10, margin = 12;
    Object.assign(spot.style, { left: r.left - pad + "px", top: r.top - pad + "px", width: r.width + 2 * pad + "px", height: r.height + 2 * pad + "px" });
    const w = tip.offsetWidth, h = tip.offsetHeight;
    const left = Math.max(margin, Math.min(r.left + r.width / 2 - w / 2, innerWidth - w - margin));
    let top = r.bottom + pad + gap; // below, or above if there is no room
    if (top + h > innerHeight - margin && r.top - pad - gap - h >= margin) top = r.top - pad - gap - h;
    Object.assign(tip.style, { left: left + "px", top: top + "px" });
  }

  return { draw: tutDraw };
}
