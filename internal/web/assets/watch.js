"use strict";

// Watch mode ("Lösungsweg") of the puzzle page: the solver's trace step by
// step, with the board after each step, the refuted chains of assumptions
// (Gedankengang) to unfold, and playback.
//
// Watch(P) is created by boot() in app.js; P is the page (see there). It
// returns render (show the current position), stop (playback), setTrace (a
// trace that arrived later) and init (build the list once).
function Watch(P) {
  const { L, d, $, el, elr, chip, drawBoard, banner } = P;
  let prefix = L.prefixStates(d, d.steps);
  let expanded = d.steps.map((s) => !!s.hypothesis);
  // A position is {k} (board after k steps, explaining step k-1) or {k, j}
  // (inside the refuted chain of step k-1, after its sub-steps 0..j). The chain
  // comes before its conclusion: "Angenommen … Widerspruch … also X".
  let positions = [];
  let cursor = 0;
  let timer = null;
  const rowEls = new Map(); // "k" or "k.j" → row element

  function buildPositions() {
    const cur = positions[cursor];
    positions = [{ k: 0 }];
    d.steps.forEach((s, i) => {
      if (s.hypothesis && expanded[i]) s.hypothesis.steps.forEach((_, j) => positions.push({ k: i + 1, j }));
      positions.push({ k: i + 1 });
    });
    if (cur) {
      const same = positions.findIndex((p) => p.k === cur.k && p.j === cur.j);
      cursor = same >= 0 ? same : positions.findIndex((p) => p.k === cur.k && p.j === undefined);
    }
  }

  function buildList() {
    const ol = $("steps");
    ol.replaceChildren();
    rowEls.clear();
    const start = el("li");
    const r0 = el("div", "row");
    r0.append(el("span", "num", "0"), el("span"), el("span", "text muted", "Leeres Brett"));
    r0.onclick = () => go({ k: 0 });
    start.append(r0);
    ol.append(start);
    rowEls.set("0", r0);
    d.steps.forEach((s, i) => {
      const li = el("li");
      const row = el("div", "row");
      const text = elr("span", "text", s.reason || s.source);
      if (s.hypothesis) {
        const t = el("button", "toggle-chain", expanded[i] ? "Gedankengang ausblenden" : "Gedankengang zeigen");
        t.onclick = (ev) => {
          ev.stopPropagation();
          expanded[i] = !expanded[i];
          buildPositions();
          buildList();
          render();
        };
        text.append(t);
      }
      row.append(el("span", "num", i + 1), chip(s), text);
      row.onclick = () => go({ k: i + 1 });
      li.append(row);
      rowEls.set(String(i + 1), row);
      if (s.hypothesis && expanded[i]) {
        const chain = el("ol", "chain");
        s.hypothesis.steps.forEach((h, j) => {
          const cli = el("li");
          const crow = el("div", "row");
          crow.append(el("span", "num", `${i + 1}.${j + 1}`), chip(h), elr("span", "text", h.reason || h.source));
          crow.onclick = () => go({ k: i + 1, j });
          cli.append(crow);
          chain.append(cli);
          rowEls.set(`${i + 1}.${j}`, crow);
        });
        chain.append(elr("li", "end", "⇒ Widerspruch: " + s.hypothesis.contradiction));
        li.append(chain);
      }
      ol.append(li);
    });
  }

  function go(pos) {
    const i = positions.findIndex((p) => p.k === pos.k && p.j === pos.j);
    if (i >= 0) {
      cursor = i;
      render();
    }
  }

  function viewAt(pos) {
    const toSet = (ms) => new Set(ms.map(([r, c]) => L.idx(d, r, c)));
    if (pos.j === undefined) {
      if (pos.k === 0) return { cells: prefix[0], step: null, marks: {} };
      const step = d.steps[pos.k - 1];
      const refs = L.refs(d, step.reason);
      return { cells: prefix[pos.k], step, marks: { new: toSet(step.marks), ref: refs.units, refcell: refs.cells } };
    }
    const parent = d.steps[pos.k - 1];
    const h = parent.hypothesis;
    const cells = Uint8Array.from(prefix[pos.k - 1]);
    for (let j = 0; j <= pos.j; j++) L.apply(d, cells, h.steps[j]);
    const step = h.steps[pos.j];
    const refs = L.refs(d, step.reason);
    const marks = { new: toSet(step.marks), ref: refs.units, refcell: refs.cells };
    const last = pos.j === h.steps.length - 1;
    if (last) marks.bad = new Set((h.cells || []).map(([r, c]) => L.idx(d, r, c)));
    return { cells, step, parent, last, marks };
  }

  // A trace that arrived later (loadTrace).
  function setTrace() {
    prefix = L.prefixStates(d, d.steps);
    expanded = d.steps.map((s) => !!s.hypothesis);
    positions = [];
    cursor = 0;
    buildPositions();
    buildList();
  }

  function render() {
    const lib = P.lib;
    $("panel-watch").classList.toggle("pending", !!(lib && lib.pending));
    if (lib && lib.pending && P.done) P.loadTrace(); // e.g. opened with #weg
    const pos = positions[cursor];
    const v = viewAt(pos);
    drawBoard(v);
    const cur = $("current");
    cur.replaceChildren();
    const head = el("div", "head");
    if (pos.k === 0) {
      head.append(el("span", "", `Start · ${d.steps.length} Schritte`));
      cur.append(head, el("p", "reason muted", "Leeres Brett. Mit ▶ oder → geht es Schritt für Schritt durch die Lösung."));
    } else {
      const num = pos.j === undefined ? `Schritt ${pos.k} von ${d.steps.length}` : `Gedankengang ${pos.k}.${pos.j + 1}`;
      head.append(chip(v.step), el("span", "", num));
      cur.append(head, elr("p", "reason", v.step.reason || v.step.source));
      if (v.last) cur.append(elr("p", "contra", "⇒ Widerspruch: " + v.parent.hypothesis.contradiction));
    }
    if (pos.j !== undefined) {
      const first = v.parent.hypothesis.steps[0].marks.find((m) => m[2] === 1);
      banner(first ? `Gedankenexperiment: Was wäre, wenn auf (${first[0]},${first[1]}) ein Stern stünde?` : "Gedankenexperiment", "what-if");
    } else if (d.status === "pending") banner(null);
    else if (pos.k === d.steps.length && d.status === "solved") banner("Gelöst.", "good");
    else if (pos.k === d.steps.length && d.status !== "solved") banner(d.status === "stuck" ? "Hier kommt der Solver nicht weiter." : "Widerspruch: " + (d.error || ""), "what-if");
    else banner(null);

    for (const r of rowEls.values()) r.classList.remove("active");
    const key = pos.j === undefined ? String(pos.k) : `${pos.k}.${pos.j}`;
    const row = rowEls.get(key);
    if (row) {
      row.classList.add("active");
      // Scroll only the list (scrollIntoView would also move the page).
      const list = $("steps");
      if (row.offsetTop < list.scrollTop) list.scrollTop = row.offsetTop - 8;
      else if (row.offsetTop + row.offsetHeight > list.scrollTop + list.clientHeight)
        list.scrollTop = row.offsetTop + row.offsetHeight - list.clientHeight + 8;
    }
    $("first").disabled = $("prev").disabled = cursor === 0;
    $("last").disabled = $("next").disabled = cursor === positions.length - 1;
    if (cursor === positions.length - 1) stop();
  }

  const PLAY = '<svg viewBox="0 0 16 16"><path d="M4 2v12l10-6z"/></svg> Abspielen';
  const PAUSE = '<svg viewBox="0 0 16 16"><path d="M3 2h3.5v12H3zM9.5 2H13v12H9.5z"/></svg> Pause';
  function stop() {
    clearInterval(timer);
    timer = null;
    $("play").innerHTML = PLAY;
  }
  function step(delta) {
    cursor = Math.max(0, Math.min(positions.length - 1, cursor + delta));
    render();
  }
  $("first").onclick = () => { stop(); cursor = 0; render(); };
  $("last").onclick = () => { stop(); cursor = positions.length - 1; render(); };
  $("prev").onclick = () => { stop(); step(-1); };
  $("next").onclick = () => { stop(); step(1); };
  $("play").onclick = () => {
    if (timer) return stop();
    if (cursor === positions.length - 1) cursor = 0;
    $("play").innerHTML = PAUSE;
    step(1);
    timer = setInterval(() => step(1), +$("speed").value);
  };
  $("speed").onchange = () => {
    if (timer) {
      clearInterval(timer);
      timer = setInterval(() => step(1), +$("speed").value);
    }
  };

  return {
    render,
    stop,
    setTrace,
    init() {
      buildPositions();
      buildList();
    },
  };
}
