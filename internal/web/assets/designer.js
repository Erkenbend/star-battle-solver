// Designer page (starbattle design): draw a pattern, check it on the server
// while drawing, let the server complete it in several ways, pick one, save it.
(function designer() {
  const L = Logic;
  const cfg = JSON.parse(document.getElementById("data").textContent);
  const $ = (id) => document.getElementById(id);
  const el = (tag, cls, text) => {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  };
  const SVG = "http://www.w3.org/2000/svg";
  const FREE = -1;
  const LETTERS = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz";
  const STORE = "starbattle-designer";

  let n = 8;
  let pattern = [];  // n×n, region or FREE
  let color = 0;     // current color, FREE = eraser
  let cands = [];    // completions of the last request
  let candPattern = null; // the pattern they were computed for
  let chosen = -1;
  let problem = null; // cells with a problem, as "r,c"
  let valid = false;  // the last check found no problem

  const setColor = (e, reg) => {
    const [h, s, l] = L.regionHSL(n, reg);
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
      if (v && v.pattern && v.pattern.length === v.n && v.n >= cfg.minN && v.n <= cfg.maxN) return v;
    } catch (e) { /* no storage */ }
    return null;
  };
  const store = () => {
    try { localStorage.setItem(STORE, JSON.stringify({ n, pattern })); } catch (e) { /* no storage */ }
  };

  const emptyPattern = (k) => Array.from({ length: k }, () => new Array(k).fill(FREE));

  // ---- controls ----
  for (let k = cfg.minN; k <= cfg.maxN; k++) $("size").append(new Option(`${k}×${k}`, k));
  for (let k = 1; k <= cfg.maxCandidates; k++) $("count").append(new Option(k, k));
  $("count").value = cfg.candidates;

  $("size").addEventListener("change", () => {
    const k = +$("size").value;
    if (drawnCount() > 0 && !confirm("Die Zeichnung geht beim Ändern der Größe verloren. Weiter?")) {
      $("size").value = n;
      return;
    }
    start(k, emptyPattern(k));
  });
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
      const p = await post("/api/check", { pattern }, ctl.signal);
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

  let genCtl = null;
  $("go").addEventListener("click", async () => {
    if (genCtl) return;
    const ctl = (genCtl = new AbortController());
    const k = +$("count").value;
    const snapshot = pattern.map((row) => row.slice());
    $("go").disabled = true;
    $("stop").hidden = false;
    $("info").textContent = `Berechne ${k} Vorschl${k === 1 ? "ag" : "äge"} parallel …` + (n >= 13 ? " Bei dieser Größe kann das eine Weile dauern." : "");
    const t0 = performance.now();
    try {
      const res = await post("/api/complete", { pattern: snapshot, candidates: k }, ctl.signal);
      if (res.problem) {
        setProblem(res.problem);
        setStatus("bad", res.problem.message);
        $("info").textContent = "Das Muster ist ungültig, siehe links.";
        return;
      }
      // Hardest first: the library collects puzzles the rules alone cannot solve.
      cands = res.candidates.sort((a, b) => b.rulesFree - a.rulesFree || b.stats.lookahead - a.stats.lookahead);
      candPattern = snapshot;
      const secs = ((performance.now() - t0) / 1000).toFixed(1);
      let msg = `${cands.length} Vorschl${cands.length === 1 ? "ag" : "äge"} in ${secs} s, schwerste zuerst.`;
      if (res.failed) msg += ` ${res.failed} Versuch${res.failed === 1 ? "" : "e"} ohne Ergebnis.`;
      if (!cands.length) msg = `Kein Vorschlag gefunden (${secs} s). Der Generator schafft dieses Muster nicht; etwas weniger oder anders zeichnen hilft meist.`;
      $("info").textContent = msg;
      showCands(cands.length ? 0 : -1);
      markStale();
    } catch (e) {
      $("info").textContent = e.name === "AbortError" ? "Abgebrochen." : "Fehler: " + e.message;
    } finally {
      genCtl = null;
      $("stop").hidden = true;
      $("go").disabled = !valid;
    }
  });
  $("stop").addEventListener("click", () => genCtl && genCtl.abort());

  // Candidates stay after the drawing changes, but are marked as belonging
  // to the earlier drawing.
  function markStale() {
    const stale = !!candPattern && cands.length > 0 && JSON.stringify(candPattern) !== JSON.stringify(pattern);
    $("cands").classList.toggle("stale", stale);
    $("stale").hidden = !stale;
  }

  // ---- candidates ----
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
        rect.setAttribute("class", "cell-r" + (opts.highlight && candPattern[r][c] === FREE ? " undrawn" : ""));
        const [h, s, l] = L.regionHSL(k, cand.regions[r][c]);
        rect.style.setProperty("--h", h);
        rect.style.setProperty("--s", s + "%");
        rect.style.setProperty("--l", l + "%");
        rect.style.setProperty("--ds", Math.round(s * 0.62) + "%");
        rect.style.setProperty("--dl", Math.round(22 + (l - 60) * 0.75) + "%");
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

  function showCands(sel) {
    const box = $("cands");
    box.replaceChildren();
    cands.forEach((c, i) => {
      const b = el("button", "cand");
      b.setAttribute("aria-pressed", String(i === sel));
      b.append(preview(c, { highlight: false }));
      const m = el("div", "meta");
      m.innerHTML = difficulty(c); // numbers only
      b.append(m);
      b.addEventListener("click", () => showCands(i));
      box.append(b);
    });
    chosen = sel;
    showDetail();
  }

  function showDetail() {
    const c = cands[chosen];
    $("detail").hidden = !c;
    $("save-msg").textContent = "";
    if (!c) return;
    $("big").replaceChildren(preview(c, { big: true, highlight: $("show-pattern").checked, solution: $("show-solution").checked }));
    $("detail-meta").innerHTML = difficulty(c);
    const text = c.regions.map((row) => row.map((v) => LETTERS[v]).join("")).join("/");
    $("try").href = "/play?p=" + encodeURIComponent(text);
  }
  $("show-pattern").addEventListener("change", showDetail);
  $("show-solution").addEventListener("change", showDetail);

  $("save").addEventListener("click", async () => {
    const c = cands[chosen];
    if (!c) return;
    const msg = $("save-msg");
    msg.className = "";
    msg.textContent = "Speichere …";
    try {
      const res = await post("/api/save", { pattern: candPattern, regions: c.regions, seed: c.seed, name: $("name").value });
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
