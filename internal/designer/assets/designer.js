// Designer page (starbattle design): draw a pattern, check it on the server
// while drawing, let the server search for several completions, pick one,
// give its regions other colors if wanted, save it.
(function designer() {
  const L = Logic;
  const cfg = JSON.parse(document.getElementById("data").textContent);
  const $ = (id) => document.getElementById(id);
  const { el } = Dom;
  const SVG = "http://www.w3.org/2000/svg";
  const FREE = -1;
  const LETTERS = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz";
  const STORE = "starbattle-designer";

  let n = 8;
  let pattern = [];  // n×n, region or FREE
  let color = 0;     // current color, FREE = eraser
  let cands = [];    // completions of the last request, hardest first
  let candPattern = null; // the pattern they were computed for
  let candStrict = false; // and the mode
  let chosen = null;  // the selected candidate
  let problem = null; // cells with a problem, as "r,c"
  let valid = false;  // the last check found no problem
  // Colors chosen for a completion's regions, by seed: the color of each
  // region, null while grey. Without an entry, the generator's colors.
  const colorsOf = new Map();
  let recoloring = false; // the color bar of the detail view is open
  let paint = 0;          // the color it paints with

  const setColor = (e, reg, k = n) => {
    const [h, s, l] = L.regionHSL(k, reg);
    e.style.setProperty("--h", h);
    e.style.setProperty("--s", s + "%");
    e.style.setProperty("--l", l + "%");
    e.style.setProperty("--ds", Math.round(s * 0.62) + "%");
    e.style.setProperty("--dl", Math.round(22 + (l - 60) * 0.75) + "%");
  };

  // Local storage is only a convenience: the drawing survives a reload.
  const load = () => {
    try {
      const v = JSON.parse(localStorage.getItem(STORE));
      if (v && v.pattern && v.pattern.length === v.n && v.n >= cfg.minN && v.n <= cfg.maxN) {
        $("strict").checked = !!v.strict;
        return v;
      }
    } catch (e) { /* no storage */ }
    return null;
  };
  const store = () => {
    try { localStorage.setItem(STORE, JSON.stringify({ n, pattern, strict: $("strict").checked })); } catch (e) { /* no storage */ }
  };

  const emptyPattern = (k) => Array.from({ length: k }, () => new Array(k).fill(FREE));

  // ---- controls ----
  for (let k = cfg.minN; k <= cfg.maxN; k++) $("size").append(new Option(`${k}×${k}`, k));
  for (const k of cfg.choices) $("count").append(new Option(k, k));
  $("count").value = cfg.candidates;

  $("size").addEventListener("change", () => {
    const k = +$("size").value;
    if (drawnCount() > 0 && !confirm("Die Zeichnung geht beim Ändern der Größe verloren. Weiter?")) {
      $("size").value = n;
      return;
    }
    start(k, emptyPattern(k));
  });
  $("strict").addEventListener("change", () => changed());
  $("clear").addEventListener("click", () => {
    if (drawnCount() > 0 && !confirm("Zeichnung leeren?")) return;
    start(n, emptyPattern(n));
  });

  function drawnCount() {
    let k = 0;
    for (const row of pattern) for (const v of row) if (v !== FREE) k++;
    return k;
  }

  function buildPalette() {
    const pal = $("palette");
    pal.replaceChildren();
    for (let reg = 0; reg < n; reg++) {
      const b = el("button");
      setColor(b, reg);
      b.dataset.reg = reg;
      b.title = "Farbe " + LETTERS[reg];
      b.append(LETTERS[reg], el("span", "cnt", ""));
      b.addEventListener("click", () => pick(reg));
      pal.append(b);
    }
    const er = el("button", "eraser", "⌫");
    er.dataset.reg = FREE;
    er.title = "Radierer";
    er.addEventListener("click", () => pick(FREE));
    pal.append(er);
    pick(color < n ? color : 0);
  }
  function pick(reg) {
    color = reg;
    for (const b of $("palette").children) b.setAttribute("aria-pressed", String(+b.dataset.reg === reg));
  }
  function updateCounts() {
    const cnt = new Array(n).fill(0);
    for (const row of pattern) for (const v of row) if (v !== FREE) cnt[v]++;
    for (const b of $("palette").children) {
      const reg = +b.dataset.reg;
      if (reg !== FREE) b.querySelector(".cnt").textContent = cnt[reg] || "";
    }
  }

  // ---- canvas ----
  const cells = [];
  function buildCanvas() {
    const cv = $("canvas");
    cv.replaceChildren();
    cv.style.setProperty("--n", n);
    cells.length = 0;
    for (let r = 0; r < n; r++) {
      for (let c = 0; c < n; c++) {
        const d = el("div", "c");
        d.dataset.r = r;
        d.dataset.c = c;
        cv.append(d);
        cells.push(d);
      }
    }
    paintAll();
  }
  function paintCell(r, c) {
    const d = cells[r * n + c];
    const v = pattern[r][c];
    d.classList.toggle("free", v === FREE);
    d.classList.toggle("drawn", v !== FREE);
    d.classList.toggle("problem", !!problem && problem.has(r + "," + c));
    d.textContent = v === FREE ? "" : LETTERS[v];
    if (v !== FREE) setColor(d, v);
  }
  function paintAll() {
    for (let r = 0; r < n; r++) for (let c = 0; c < n; c++) paintCell(r, c);
    updateCounts();
  }

  // Dragging paints one value: the current color, or FREE if the drag starts
  // on a cell that already has it (click to erase) or with the right button.
  let painting = null;
  const cellAt = (x, y) => {
    const t = document.elementFromPoint(x, y);
    return t && t.classList.contains("c") && t.parentElement === $("canvas") ? t : null;
  };
  const apply = (t) => {
    const r = +t.dataset.r, c = +t.dataset.c;
    if (pattern[r][c] === painting) return;
    pattern[r][c] = painting;
    paintCell(r, c);
    changed();
  };
  $("canvas").addEventListener("contextmenu", (e) => e.preventDefault());
  $("canvas").addEventListener("pointerdown", (e) => {
    const t = cellAt(e.clientX, e.clientY);
    if (!t) return;
    e.preventDefault();
    const cur = pattern[+t.dataset.r][+t.dataset.c];
    painting = e.button === 2 || (color !== FREE && cur === color) ? FREE : color;
    $("canvas").setPointerCapture(e.pointerId);
    apply(t);
  });
  $("canvas").addEventListener("pointermove", (e) => {
    if (painting === null) return;
    const t = cellAt(e.clientX, e.clientY);
    if (t) apply(t);
  });
  const endPaint = () => { painting = null; };
  $("canvas").addEventListener("pointerup", endPaint);
  $("canvas").addEventListener("pointercancel", endPaint);

  // ---- server ----
  async function post(path, body, signal) {
    const res = await fetch(path, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body), signal });
    if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
    return res.json();
  }

  function setStatus(kind, text) {
    const s = $("status");
    s.className = "status " + kind;
    s.textContent = text;
  }
  function setProblem(p) {
    problem = p && p.cells ? new Set(p.cells.map(([r, c]) => r + "," + c)) : null;
    paintAll();
  }

  // Check the pattern after every change, debounced; a newer check aborts
  // an older one.
  let checkTimer = 0, checkCtl = null;
  function changed() {
    markStale();
    updateCounts();
    store();
    clearTimeout(checkTimer);
    checkTimer = setTimeout(check, 250);
    setStatus("busy", "Prüfe …");
    valid = false;
    $("go").disabled = true;
  }
  async function check() {
    if (checkCtl) checkCtl.abort();
    const ctl = (checkCtl = new AbortController());
    try {
      const p = await post("/api/check", { pattern, strict: $("strict").checked }, ctl.signal);
      if (ctl !== checkCtl) return;
      setProblem(p);
      valid = !p;
      if (p) {
        setStatus("bad", p.message);
      } else {
        const k = drawnCount();
        setStatus("ok", k === 0
          ? "Noch nichts gezeichnet. Ohne Muster entsteht ein ganz normales Zufallspuzzle."
          : `Muster gültig (${k} von ${n * n} Feldern gezeichnet).`);
      }
      $("go").disabled = !valid || genCtl !== null;
    } catch (e) {
      if (e.name !== "AbortError") setStatus("bad", "Prüfen fehlgeschlagen: " + e.message);
    }
  }

  // The server searches until it has k completions, its budget of attempts
  // is used up or the page stops it. Candidates arrive one by one (NDJSON,
  // see designer.go), so one can be inspected and saved early.
  let genCtl = null;
  const plural = (k, one, many) => `${k.toLocaleString("de-DE")} ${k === 1 ? one : many}`;
  const num = (k) => k.toLocaleString("de-DE");
  // Hardest first: the library collects puzzles the rules alone cannot solve.
  const harder = (a, b) => b.rulesFree - a.rulesFree || b.stats.lookahead - a.stats.lookahead || a.seed - b.seed;
  $("go").addEventListener("click", async () => {
    if (genCtl) return;
    const ctl = (genCtl = new AbortController());
    const k = +$("count").value;
    const snapshot = pattern.map((row) => row.slice());
    const strict = $("strict").checked;
    cands = [];
    colorsOf.clear();
    recoloring = false;
    candPattern = snapshot;
    candStrict = strict;
    showCands(null);
    markStale();
    $("go").disabled = true;
    $("stop").hidden = false;
    const t0 = performance.now();
    const secs = () => ((performance.now() - t0) / 1000).toLocaleString("de-DE", { minimumFractionDigits: 1, maximumFractionDigits: 1 });
    let counts = { tried: 0, accepted: 0, duplicates: 0 };
    const counters = (limit) => `angenommen/versucht: ${num(counts.accepted)}/${num(counts.tried)}` +
      (limit ? ` von höchstens ${num(cfg.maxTries)}` : "") +
      (counts.duplicates ? ` · ${num(counts.duplicates)} doppelt` : "");
    const progress = () => {
      $("info").textContent = `Suche läuft seit ${secs()} s · ${num(cands.length)} von ${num(k)} Vorschlägen · ${counters(true)}. ` +
        "Fertige Vorschläge lassen sich schon speichern.";
    };
    progress();
    const timer = setInterval(progress, 500);
    const summary = (prefix) => cands.length
      ? `${prefix} ${secs()} s: ${plural(cands.length, "Vorschlag", "Vorschläge")}, schwerste zuerst · ${counters()}.`
      : `${prefix} ${secs()} s: kein Vorschlag · ${counters()}.`;
    const handle = (ev) => {
      if (ev.problem) {
        setProblem(ev.problem);
        setStatus("bad", ev.problem.message);
        valid = false;
        $("info").textContent = "Das Muster ist ungültig, siehe links.";
        return;
      }
      if (ev.progress) {
        counts = ev.progress;
        return;
      }
      if (ev.candidate) {
        cands.push(ev.candidate);
        cands.sort(harder);
        showCands(chosen ? chosen.seed : ev.candidate.seed);
        progress();
        return;
      }
      if (ev.done) {
        counts = ev.done;
        $("info").textContent = cands.length
          ? summary("Fertig nach") + (cands.length < k ? ` Mehr waren in ${num(counts.tried)} Versuchen nicht zu finden.` : "")
          : `Kein Vorschlag gefunden (${secs()} s, ${counters()}). Der Generator schafft dieses Muster nicht; etwas weniger oder anders zeichnen hilft meist.` +
            (strict ? " Im strengen Modus bleibt ihm oft zu wenig Spielraum, etwa wenn kleine freie Flächen ringsum eingeschlossen sind; ohne „Streng“ klappt es eher." : "");
      }
    };
  try {
      const res = await fetch("/api/complete", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ pattern: snapshot, candidates: k, strict }),
        signal: ctl.signal,
      });
      if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
      const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
      let buf = "";
      for (;;) {
        const { value, done } = await reader.read();
        if (done) break;
        buf += value;
        let i;
        while ((i = buf.indexOf("\n")) >= 0) {
          const line = buf.slice(0, i).trim();
          buf = buf.slice(i + 1);
          if (line) handle(JSON.parse(line));
        }
      }
    } catch (e) {
      $("info").textContent = e.name === "AbortError" ? summary("Angehalten nach") : "Fehler: " + e.message;
    } finally {
      clearInterval(timer);
      genCtl = null;
      $("stop").hidden = true;
      $("go").disabled = !valid;
    }
  });
  $("stop").addEventListener("click", () => genCtl && genCtl.abort());

  // Candidates stay after the drawing changes, but are marked as belonging
  // to the earlier drawing.
  function markStale() {
    const stale = !!candPattern && cands.length > 0 && (JSON.stringify(candPattern) !== JSON.stringify(pattern) || candStrict !== $("strict").checked);
    $("cands").classList.toggle("stale", stale);
    $("stale").hidden = !stale;
  }

  // ---- candidates ----
  // The color of region reg of cand, null for grey.
  const colorOf = (cand, reg) => {
    const m = colorsOf.get(cand.seed);
    return m ? m[reg] : reg;
  };
  const allColored = (cand) => {
    const m = colorsOf.get(cand.seed);
    return !m || m.every((v) => v !== null);
  };
  // The regions of cand in their chosen colors.
  const colored = (cand) => cand.regions.map((row) => row.map((v) => colorOf(cand, v)));

  function preview(cand, opts) {
    const k = cand.regions.length;
    const svg = document.createElementNS(SVG, "svg");
    svg.setAttribute("viewBox", `-0.05 -0.05 ${k + 0.1} ${k + 0.1}`);
    svg.setAttribute("role", "img");
    for (let r = 0; r < k; r++) {
      for (let c = 0; c < k; c++) {
        const rect = document.createElementNS(SVG, "rect");
        rect.setAttribute("x", c);
        rect.setAttribute("y", r);
        rect.setAttribute("width", 1.02);
        rect.setAttribute("height", 1.02);
        const col = colorOf(cand, cand.regions[r][c]);
        rect.setAttribute("class", (col === null ? "grey-r" : "cell-r") + (opts.highlight && candPattern[r][c] === FREE ? " undrawn" : ""));
        rect.dataset.r = r;
        rect.dataset.c = c;
        if (col !== null) setColor(rect, col, k);
        svg.append(rect);
      }
    }
    let grid = "", walls = "";
    for (let r = 0; r < k; r++) {
      for (let c = 0; c < k; c++) {
        if (c + 1 < k) (cand.regions[r][c] !== cand.regions[r][c + 1] ? (walls += `M${c + 1} ${r}V${r + 1}`) : (grid += `M${c + 1} ${r}V${r + 1}`));
        if (r + 1 < k) (cand.regions[r][c] !== cand.regions[r + 1][c] ? (walls += `M${c} ${r + 1}H${c + 1}`) : (grid += `M${c} ${r + 1}H${c + 1}`));
      }
    }
    const g = document.createElementNS(SVG, "path");
    g.setAttribute("class", "grid");
    g.setAttribute("stroke-width", 0.02);
    g.setAttribute("d", grid);
    const w = document.createElementNS(SVG, "path");
    w.setAttribute("class", "walls");
    w.setAttribute("stroke-width", opts.big ? 0.07 : 0.09);
    w.setAttribute("d", walls + `M0 0H${k}V${k}H0Z`);
    svg.append(g, w);
    if (opts.solution) {
      cand.solution.forEach((c, r) => {
        const s = document.createElementNS(SVG, "circle");
        s.setAttribute("class", "star");
        s.setAttribute("cx", c + 0.5);
        s.setAttribute("cy", r + 0.5);
        s.setAttribute("r", 0.28);
        s.setAttribute("stroke-width", 0.04);
        svg.append(s);
      });
    }
    return svg;
  }

  function difficulty(c) {
    const parts = [];
    parts.push(c.rulesFree === 0
      ? "Regeln allein: <strong>gelöst</strong>"
      : `Regeln allein: <strong>bleiben bei ${c.rulesFree} freien Feldern hängen</strong>`);
    if (c.stats.lookahead || c.stats.guesses) parts.push(`Gedankenexperimente: ${c.stats.lookahead}, geraten: ${c.stats.guesses}`);
    parts.push(`${c.steps} Schritte · Nr. ${c.seed} · ${c.millis} ms`);
    return parts.join("<br>");
  }

  // Shows the candidates and selects the one with seed sel (none if null).
  // The detail view is rebuilt only when the selection changes, so a name
  // being typed or a save message survives new arrivals.
  function showCands(sel) {
    const box = $("cands");
    box.replaceChildren();
    const next = cands.find((c) => c.seed === sel) || null;
    cands.forEach((c) => {
      const b = el("button", "cand");
      b.setAttribute("aria-pressed", String(c === next));
      b.append(preview(c, { highlight: false }));
      const m = el("div", "meta");
      m.innerHTML = difficulty(c); // numbers only
      b.append(m);
      b.addEventListener("click", () => showCands(c.seed));
      box.append(b);
    });
    if (next !== chosen) {
      // A coloring left half done is dropped; the bar belongs to one
      // completion.
      if (chosen && !allColored(chosen)) colorsOf.delete(chosen.seed);
      recoloring = false;
      chosen = next;
      showDetail();
    }
  }


  function showDetail() {
    const c = chosen;
    $("detail").hidden = !c;
    $("save-msg").textContent = "";
    if (!c) return;
    $("big").replaceChildren(preview(c, { big: true, highlight: $("show-pattern").checked, solution: $("show-solution").checked }));
    $("big").classList.toggle("painting", recoloring);
    $("detail-meta").innerHTML = difficulty(c);
    showRecolor();
    // Saved and tried in the chosen colors, once every region has one.
    const ready = allColored(c);
    $("save").disabled = !ready;
    if (ready) {
      const text = colored(c).map((row) => row.map((v) => LETTERS[v]).join("")).join("/");
      $("try").href = "/play?p=" + encodeURIComponent(text);
      $("try").removeAttribute("aria-disabled");
    } else {
      $("try").removeAttribute("href");
      $("try").setAttribute("aria-disabled", "true");
    }
  }
  $("show-pattern").addEventListener("change", showDetail);
  $("show-solution").addEventListener("change", showDetail);

  // ---- colors ----
  // "Farben neu verteilen" turns every region grey; then a color of the bar
  // and a click on a region paint it. Each color is on one region at most:
  // painting it elsewhere moves it.
  function showRecolor() {
    const c = chosen;
    $("recolor").hidden = !recoloring;
    $("recolor-btn").setAttribute("aria-pressed", String(recoloring));
    if (!recoloring) return;
    const k = c.regions.length;
    const m = colorsOf.get(c.seed);
    const pal = $("recolor-palette");
    pal.replaceChildren();
    for (let reg = 0; reg < k; reg++) {
      const b = el("button", m.includes(reg) ? "used" : null);
      b.append(LETTERS[reg], el("span", "cnt", m.includes(reg) ? "✓" : ""));
      setColor(b, reg, k);
      b.dataset.reg = reg;
      b.title = "Farbe " + LETTERS[reg] + (m.includes(reg) ? " (schon vergeben, wandert mit dem nächsten Klick)" : "");
      b.setAttribute("aria-pressed", String(reg === paint));
      b.addEventListener("click", () => {
        paint = reg;
        showRecolor();
      });
      pal.append(b);
    }
    const grey = m.filter((v) => v === null).length;
    $("recolor-rest").disabled = grey === 0;
    $("recolor-reset").disabled = grey === k;
    $("recolor-done").disabled = grey > 0;
    $("recolor-note").textContent = (grey
      ? `Noch ${grey} von ${k} Zonen grau. Speichern und Ausprobieren gehen, sobald jede Zone eine Farbe hat. `
      : "Jede Zone hat eine Farbe; so wird das Puzzle gespeichert und ausprobiert. ") +
      "Farbe wählen, dann eine Zone im Bild anklicken. Jede Farbe sitzt auf höchstens einer Zone (✓) und wandert mit dem nächsten Klick dorthin; ein Klick auf eine Zone in der gewählten Farbe macht sie wieder grau. " +
      "„Rest automatisch verteilen“ gibt jeder grauen Zone ihre ursprüngliche Farbe, wenn die noch frei ist, sonst eine freie.";
  }
  // Redraws the candidates (their previews show the colors too) and the
  // detail view after a change of the colors.
  const recolored = () => {
    showCands(chosen.seed);
    showDetail();
  };
  $("recolor-btn").addEventListener("click", () => {
    if (!chosen) return;
    colorsOf.set(chosen.seed, new Array(chosen.regions.length).fill(null));
    recoloring = true;
    paint = 0;
    $("show-pattern").checked = false;
    recolored();
  });
  $("big").addEventListener("click", (e) => {
    const t = e.target;
    if (!recoloring || !chosen || t.tagName !== "rect") return;
    const m = colorsOf.get(chosen.seed);
    const reg = chosen.regions[+t.dataset.r][+t.dataset.c];
    if (m[reg] === paint) {
      m[reg] = null;
    } else {
      const was = m.indexOf(paint);
      if (was >= 0) m[was] = null;
      m[reg] = paint;
    }
    recolored();
  });
  $("recolor-rest").addEventListener("click", () => {
    const m = colorsOf.get(chosen.seed);
    for (let reg = 0; reg < m.length; reg++) if (m[reg] === null && !m.includes(reg)) m[reg] = reg;
    const free = m.map((_, col) => col).filter((col) => !m.includes(col));
    for (let reg = 0; reg < m.length; reg++) if (m[reg] === null) m[reg] = free.shift();
    recolored();
  });
  $("recolor-reset").addEventListener("click", () => {
    colorsOf.get(chosen.seed).fill(null);
    recolored();
  });
  $("recolor-done").addEventListener("click", () => {
    recoloring = false;
    showDetail();
  });

  $("save").addEventListener("click", async () => {
    const c = chosen;
    if (!c || !allColored(c)) return;
    const m = colorsOf.get(c.seed);
    const colors = m && m.some((col, reg) => col !== reg) ? m : undefined;
    const msg = $("save-msg");
    msg.className = "";
    msg.textContent = "Speichere …";
    try {
      const res = await post("/api/save", { pattern: candPattern, regions: c.regions, seed: c.seed, colors, name: $("name").value, strict: candStrict });
      msg.className = "saved";
      msg.textContent = `Gespeichert als ${cfg.dir}/${res.file}`;
    } catch (e) {
      msg.className = "err";
      msg.textContent = e.message;
    }
  });

  // ---- start ----
  function start(k, pat) {
    n = k;
    pattern = pat;
    $("size").value = n;
    problem = null;
    buildPalette();
    buildCanvas();
    changed();
  }
  const saved = load();
  start(saved ? saved.n : 8, saved ? saved.pattern : emptyPattern(8));
})();
