// UI check, group library: the library (/bibliothek), a hard puzzle and a
// motif.
import { BASE, cell, click, data, fresh, go, js, obs, record, solveBy, waitFor } from "../lib.mjs";

// Those that run on a phone; the others run on a desktop screen.
export const PHONE_SCENARIOS = new Set(["libraryphone"]);

export const scenarios = {
  // Library: a hard puzzle without help (trace only once solved), a motif.
  async library() {
    await fresh();
    await go("/bibliothek");
    const parts = await js(`JSON.parse(document.getElementById("data").textContent).parts`);
    const hard = parts.find((p) => p.kind === "schwer").entries[0];
    const motif = parts.find((p) => p.kind === "motive").entries[0];
    // The tab "Schwer" has some 190 previews: with a shape per cell (31,000
    // in all) switching to it took a second on a desktop, 3 s on a phone.
    // One shape per region (n of them) and the walls keep it fast.
    await click("#tab-schwer");
    record({
      label: "library tab schwer",
      selected: await js(`document.querySelector(".lib-tabs [aria-selected=true]").id`),
      tilesAll: await js(`document.querySelectorAll(".lib-tile").length === ${parts.find((p) => p.kind === "schwer").entries.length}`),
      shapesPerRegion: await js(`[...document.querySelectorAll(".lib-tile svg")].every((s) => s.childElementCount <= s.viewBox.baseVal.width + 1)`),
    });
    await go("/bibliothek?id=" + encodeURIComponent(hard.id));
    await obs("hard start");
    const t = await (await fetch(`${BASE}/bibliothek/weg?id=${encodeURIComponent(hard.id)}`)).json();
    await solveBy(t.solution, t.n, new Set([t.n - 1]));
    await obs("hard nearly");
    await click(cell((t.n - 1) * t.n + t.solution[t.n - 1]), { shift: true });
    await waitFor(`document.querySelectorAll("#steps .row").length > 1`);
    await obs("hard solved");
    await click("#tab-watch");
    await obs("hard watch");
    await go("/bibliothek?id=" + encodeURIComponent(hard.id) + "#weg");
    await waitFor(`document.querySelectorAll("#steps .row").length > 1`);
    await obs("hard reopened #weg");
    await go("/bibliothek?id=" + encodeURIComponent(motif.id));
    await obs("motif start");
    await click("#hint");
    await obs("motif hint");
    const md = await data();
    await solveBy(md.solution, md.n);
    await obs("motif solved");
  },

  // The library on a phone: once a puzzle is solved, its banner ("Weiter:
  // 10-4") must be on screen, and the board must not move. After library,
  // which sees the hard puzzle before the server has solved it.
  async libraryphone() {
    await scenarios.library();
  },
};
