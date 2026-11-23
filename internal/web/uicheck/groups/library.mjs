// UI check, group library: the library (/bibliothek), a hard puzzle and a
// motif.
import { BASE, cell, click, copied, data, follow, fresh, go, js, obs, record, reload, solveBy, waitFor } from "../lib.mjs";

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
    await copied("hard result copied", "#banner .share");
    await click("#tab-watch");
    await obs("hard watch");
    await go("/bibliothek?id=" + encodeURIComponent(hard.id) + "#weg");
    await waitFor(`document.querySelectorAll("#steps .row").length > 1`);
    await obs("hard reopened #weg");
    // Unsolved, a hard puzzle opens in the play tab even with #weg (another
    // one: the first must still be unsolved by the server at its start).
    const hard2 = parts.find((p) => p.kind === "schwer").entries[1];
    await go("/bibliothek?id=" + encodeURIComponent(hard2.id) + "#weg");
    await obs("other hard #weg unsolved");
    await go("/bibliothek?id=" + encodeURIComponent(motif.id));
    await obs("motif start");
    await click("#hint");
    await obs("motif hint");
    const md = await data();
    await solveBy(md.solution, md.n);
    await obs("motif solved");
    await copied("motif result copied", "#banner .share");
    await reload(); // kept under the motif's ID: solved, its time and hints
    await obs("motif reloaded");
    // The way on: the next unsolved motif, below the banner.
    await follow("#next-go");
    await obs("next motif");
    // The last unsolved motif: no way on, the banner says all are solved.
    const all = await js(`JSON.parse(localStorage.getItem("starbattle:library:solved") || "{}")`);
    const here = await data();
    for (const e of parts.find((p) => p.kind === "motive").entries) if (e.id !== here.library.id && !all[e.id]) all[e.id] = { time: 60000, at: "2026-01-01" };
    await js(`localStorage.setItem("starbattle:library:solved", ${JSON.stringify(JSON.stringify(all))})`);
    await solveBy(here.solution, here.n);
    await obs("last motif solved");
    // The overview shows what was solved, how fast and with how many hints.
    await go("/bibliothek");
    for (const tab of ["#tab-motive", "#tab-schwer"]) {
      await click(tab);
      record({
        label: "overview " + tab,
        solved: await js(`[...document.querySelectorAll(".lib-tile.solved")].filter((t) => ${JSON.stringify([hard.id, motif.id])}.some((id) => t.getAttribute("href").endsWith(encodeURIComponent(id)))).map((t) => t.textContent.replace(/\\d+:\\d\\d/g, "T")).join(" | ")`),
      });
    }
  },

  // The library on a phone: once a puzzle is solved, its banner and "Als
  // nächstes: 10-4" below it must be on screen, and the board must not move. After library,
  // which sees the hard puzzle before the server has solved it.
  async libraryphone() {
    await scenarios.library();
  },
};
