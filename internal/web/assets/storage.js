"use strict";

// What the puzzle page keeps on this device (per-viewer convenience only):
// the progress of the puzzle under P.K.storageKey (none in the tutorial and
// in a coop round), the solved puzzles of the library with their best
// result, and the cookie that leads "/" back to an unsolved solo puzzle.
//
// Storage(P) is created by boot() in app.js; P is the page (see there).
function Storage(P) {
  const { K, d } = P;
  const key = K.storageKey;
  const num = (v) => (Number.isFinite(v) && v >= 0 ? v : 0);

  // The progress saved last time, or null.
  function load() {
    if (!key) return null;
    try { return JSON.parse(localStorage.getItem(key) || "null"); } catch { return null; }
  }
  // Keeps v as the progress.
  function save(v) {
    if (!key) return;
    try { localStorage.setItem(key, JSON.stringify(v)); } catch { /* private mode etc. */ }
  }

  // The library's solved puzzles by ID, {time, at, hints?}, for the overview
  // (/bibliothek, library.js).
  const LIB_SOLVED_KEY = "starbattle:library:solved";
  function libSolved() {
    try {
      const v = JSON.parse(localStorage.getItem(LIB_SOLVED_KEY) || "{}");
      return v && typeof v === "object" ? v : {};
    } catch { return {}; }
  }
  // Keeps the result if it is better: fewer hints (a motif's), then less time.
  function libRecord(ms, hints) {
    const all = libSolved();
    const prev = all[d.library.id];
    if (prev && Number.isFinite(prev.time)) {
      const ph = num(prev.hints);
      if (ph < hints || (ph === hints && prev.time <= ms)) return;
    }
    all[d.library.id] = { time: Math.round(ms), at: new Date().toISOString().slice(0, 10) };
    if (hints) all[d.library.id].hints = hints;
    try { localStorage.setItem(LIB_SOLVED_KEY, JSON.stringify(all)); } catch { /* private mode etc. */ }
  }

  // "/" (Solo, the start page, the installed app's start) leads back to an
  // unsolved solo puzzle: the server reads its URL from this cookie
  // (SoloCookie in server.go). Opened or played, a solo puzzle puts itself
  // there; solved, it takes itself out, unless another one is there by now.
  const SOLO_COOKIE = "starbattle.solo";
  const soloHere = SOLO_COOKIE + "=" + encodeURIComponent(location.pathname + location.search);
  function rememberSolo(done) {
    if (K.kind !== "solo") return;
    try {
      const mine = document.cookie.split("; ").includes(soloHere);
      if (!done && !mine) document.cookie = soloHere + "; path=/; max-age=7776000; samesite=lax"; // 90 days
      else if (done && mine) document.cookie = SOLO_COOKIE + "=; path=/; max-age=0; samesite=lax";
    } catch { /* cookies blocked */ }
  }

  return { load, save, libSolved, libRecord, rememberSolo };
}
