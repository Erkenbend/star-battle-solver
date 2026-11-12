"use strict";

// The settings page (/einstellungen): the play options and the symbol of the
// stars from the server's catalog. Everything is stored on this device only,
// the symbol as a copy of its catalog entry (see markSpec in app.js). Open
// puzzle pages take changes over at once (their "storage" listener).
(() => {
  if (typeof document === "undefined" || !document.getElementById("symbol-list")) return;
  const L = Logic;
  const d = JSON.parse(document.getElementById("data").textContent);
  const $ = (id) => document.getElementById(id);
  const el = (tag, cls, text) => {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  };

  const KIND = {
    svg: "Form, färbt sich bei Fehlern rot",
    glyph: "Schriftzeichen, sieht je nach Gerät etwas anders aus",
    emoji: "Emoji, sieht auf jedem Gerät anders aus",
    sprite: "Bild",
  };
  function describe(s) {
    if (s.kind !== "sprite" || !(s.frames > 1)) return KIND[s.kind] || "";
    const n = s.clips ? s.clips.length : 1;
    return n > 1 ? `Animiertes Bild, spielt nach dem Setzen eine von ${n} Animationen` : "Animiertes Bild, spielt kurz nach dem Setzen";
  }

  // Previews use the page's colors: four region colors, the last cell wrong.
  const PREVIEW = [0, 3, 5, 1];
  function preview(spec) {
    const box = el("span", "symbol-preview");
    L.markClasses(box, spec);
    const tpl = L.markSVG(spec, document);
    PREVIEW.forEach((reg, k) => {
      const c = el("span", "cell star" + (k === PREVIEW.length - 1 ? " bad" : ""));
      const [h, s, l] = L.regionHSL(8, reg);
      c.style.setProperty("--h", h);
      c.style.setProperty("--s", s + "%");
      c.style.setProperty("--l", l + "%");
      c.style.setProperty("--ds", Math.round(s * 0.62) + "%");
      c.style.setProperty("--dl", Math.round(22 + (l - 60) * 0.75) + "%");
      const sym = el("span", "sym");
      sym.append(tpl.cloneNode(true));
      c.append(sym);
      box.append(c);
    });
    return box;
  }

  // Play options (see L.pref): saved as soon as switched.
  for (const [k, id] of [["autocross", "autocross"], ["straight", "straight"], ["showProblems", "showproblems"]]) {
    $(id).checked = L.pref(k);
    $(id).onchange = () => L.setPref(k, $(id).checked);
  }

  // The stored choice; refreshed from the catalog (e.g. a changed image), so
  // the copy does not go stale.
  const stored = L.loadMark();
  let current = (stored && typeof stored.id === "string" && stored.id) || L.DEFAULT_MARK.id;
  const fresh = d.symbols.find((s) => s.id === current);
  if (stored && fresh && JSON.stringify(fresh) !== JSON.stringify(stored)) L.saveMark(fresh);

  const list = $("symbol-list");
  for (const s of d.symbols) {
    const lab = el("label", "symbol-option");
    const input = el("input");
    input.type = "radio";
    input.name = "symbol";
    input.value = s.id;
    input.checked = s.id === current;
    const spec = L.markSpec(s, { sprites: true });
    const box = preview(spec);
    // An animated symbol plays as on the board: when chosen, and on hover;
    // each cell a clip of its own.
    const play = () => {
      for (const c of box.children) L.playMark(c, spec);
    };
    lab.addEventListener("mouseenter", play);
    input.addEventListener("change", () => {
      if (!input.checked) return;
      L.saveMark(s);
      current = s.id;
      $("status").textContent = `„${s.name}“ gespeichert.`;
      play();
    });
    const text = el("span", "symbol-text");
    text.append(el("strong", "", s.name), el("span", "muted", describe(s)));
    lab.append(input, box, text);
    list.append(lab);
  }
  if (!d.symbols.some((s) => s.id === current)) $("status").textContent = "Dein bisheriges Symbol gibt es nicht mehr; es bleibt, bis du ein anderes wählst.";
})();
