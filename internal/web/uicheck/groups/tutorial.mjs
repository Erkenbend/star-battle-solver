// UI check, group tutorial: the tutorial (/anleitung) with its tour, for a
// new visitor.
import { cell, click, data, dblclick, drag, go, js, obs, once, sleep, waitFor } from "../lib.mjs";

// Those that run on a phone; the others run on a desktop screen.
export const PHONE_SCENARIOS = new Set(["tutorial"]);

export const scenarios = {
  // A new visitor goes through the tutorial and its tour and back.
  async tutorial() {
    await go("/party");
    await js(`localStorage.clear()`);
    await go("/?n=8&seed=42");
    await waitFor(`location.pathname === "/anleitung"`);
    await sleep(300);
    await obs("redirected");
    const d = await data();
    const n = d.n;
    for (const st of d.tutorial.steps) {
      for (const [r, c] of st.stars || []) await dblclick(cell(r * n + c));
      if ((st.crosses || []).length > 2) await drag(st.crosses.map(([r, c]) => cell(r * n + c)));
      else for (const [r, c] of st.crosses || []) await click(cell(r * n + c));
      await obs("step " + st.action);
    }
    await click("#hint");
    await obs("hint in tutorial");
    const marks = await js(`[...document.querySelectorAll("#board .cell")].map((e) => e.classList.contains("star"))`);
    for (let r = 0; r < n; r++) if (!marks[r * n + d.solution[r]]) await click(cell(r * n + d.solution[r]), { shift: true });
    await obs("solved");
    await click("#tutorial .actions button");
    for (let k = 0; k < d.tutorial.tour.length; k++) {
      await obs("tour " + k);
      if (!(await js(`!!document.querySelector(".tour-tip button.primary")`))) break;
      const last = await js(`document.querySelector(".tour-tip button.primary").textContent === "Fertig"`);
      if (last) {
        const loaded = once("Page.loadEventFired");
        await click(".tour-tip button.primary");
        await loaded;
        await sleep(300);
        break;
      }
      await click(".tour-tip button.primary");
    }
    await obs("back");
    await go("/anleitung");
    await obs("tutorial again");
    const loaded = once("Page.loadEventFired");
    await click("#tutorial .head button");
    await loaded;
    await sleep(300);
    await obs("skipped", { random: true });
  },
};
