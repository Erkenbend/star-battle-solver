"use strict";

// The library overview (/bibliothek): one tile per puzzle with a preview of
// its regions, grouped by size. Which puzzles are solved, and in what time,
// lives on this device only (written by app.js when a library puzzle is
// solved), keyed by the puzzle's stable ID.
(() => {
  if (typeof document === "undefined" || !document.getElementById("groups")) return;
  const L = Logic;
  const d = JSON.parse(document.getElementById("data").textContent);
  const $ = (id) => document.getElementById(id);
  const el = (tag, cls, text) => {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  };
  const SVG = "http://www.w3.org/2000/svg";

  let solved = {};
  try {
    const v = JSON.parse(localStorage.getItem("starbattle:library:solved") || "{}");
    if (v && typeof v === "object") solved = v;
  } catch { /* private mode etc. */ }
  const hideKey = "starbattle.library.hideSolved";
  try { $("hide-solved").checked = localStorage.getItem(hideKey) === "1"; } catch { /* ignore */ }

  const fmtTime = (ms) => {
    const s = Math.floor(ms / 1000);
    const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), r = s % 60;
    const pad = (x) => String(x).padStart(2, "0");
    return h ? `${h}:${pad(m)}:${pad(r)}` : `${m}:${pad(r)}`;
  };

  // Regions as colored squares with thick walls between them.
  function preview(e) {
    const n = e.n;
    const svg = document.createElementNS(SVG, "svg");
    svg.setAttribute("viewBox", `0 0 ${n} ${n}`);
    svg.setAttribute("aria-hidden", "true");
    for (let r = 0; r < n; r++) {
      for (let c = 0; c < n; c++) {
        const rect = document.createElementNS(SVG, "rect");
        rect.setAttribute("x", c);
        rect.setAttribute("y", r);
        rect.setAttribute("width", 1.02);
        rect.setAttribute("height", 1.02);
        const [h, s, l] = L.regionHSL(n, e.regions[r][c]);
        rect.style.setProperty("--h", h);
        rect.style.setProperty("--s", s + "%");
        rect.style.setProperty("--l", l + "%");
        rect.style.setProperty("--ds", Math.round(s * 0.62) + "%");
        rect.style.setProperty("--dl", Math.round(22 + (l - 60) * 0.75) + "%");
        svg.append(rect);
      }
    }
    let p = "";
    for (let r = 0; r < n; r++) {
      for (let c = 0; c < n; c++) {
        if (c + 1 < n && e.regions[r][c] !== e.regions[r][c + 1]) p += `M${c + 1} ${r}V${r + 1}`;
        if (r + 1 < n && e.regions[r][c] !== e.regions[r + 1][c]) p += `M${c} ${r + 1}H${c + 1}`;
      }
    }
    const path = document.createElementNS(SVG, "path");
    path.setAttribute("class", "thick");
    path.setAttribute("d", p);
    svg.append(path);
    return svg;
  }

  function render() {
    const hide = $("hide-solved").checked;
    const isSolved = (e) => Object.hasOwn(solved, e.id);
    const k = d.entries.filter(isSolved).length;
    $("progress").textContent = `${k} von ${d.entries.length} gelöst`;
    const bySize = new Map();
    for (const e of [...d.entries].sort((a, b) => a.n - b.n || a.nr - b.nr)) {
      if (hide && isSolved(e)) continue;
      if (!bySize.has(e.n)) bySize.set(e.n, []);
      bySize.get(e.n).push(e);
    }
    const groups = [];
    for (const [n, list] of bySize) {
      const g = el("section", "lib-group");
      g.append(el("h2", "", `${n}×${n}`));
      const grid = el("div", "lib-grid");
      for (const e of list) {
        const a = el("a", "lib-tile");
        a.href = "/bibliothek?id=" + encodeURIComponent(e.id);
        const s = solved[e.id];
        a.classList.toggle("solved", !!s);
        a.setAttribute("aria-label", `Nr. ${e.nr}, ${n}×${n}${s ? ", gelöst" : ""}`);
        a.append(preview(e));
        const meta = el("div", "meta");
        meta.append(el("span", "nr", `Nr. ${e.nr}`));
        if (s) meta.append(el("span", "done", Number.isFinite(s.time) ? `✓ ${fmtTime(s.time)}` : "✓"));
        a.append(meta);
        grid.append(a);
      }
      g.append(grid);
      groups.push(g);
    }
    if (!groups.length) groups.push(el("p", "muted", "Alles gelöst. Respekt!"));
    $("groups").replaceChildren(...groups);
  }

  $("hide-solved").onchange = () => {
    try { localStorage.setItem(hideKey, $("hide-solved").checked ? "1" : "0"); } catch { /* ignore */ }
    render();
  };
  // Coming back from a solved puzzle (bfcache) shows the new check mark.
  window.addEventListener("pageshow", () => {
    try { solved = JSON.parse(localStorage.getItem("starbattle:library:solved") || "{}") || {}; } catch { /* ignore */ }
    render();
  });
  render();
})();
