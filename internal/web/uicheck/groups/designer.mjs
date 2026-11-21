// UI check, group designer: the page of `starbattle design` (run.sh starts
// it with -dir designs in a temp directory). A tool for developers, used on a
// desktop: drawing and erasing, the check while drawing, the strict mode,
// the search with its completions, saving and trying one.
import { DESIGNER, click, follow, go, js, press, pull, record, reload, send, settle, sleep, waitFor } from "../lib.mjs";

export const PHONE_SCENARIOS = new Set();

const dc = (r, c) => `#canvas .c[data-r="${r}"][data-c="${c}"]`;
const color = (k) => `#palette button[data-reg="${k}"]`; // -1: the eraser

// Waits for the check of the drawing (debounced, then on the server).
const checked = () => settle(`!document.getElementById("status").classList.contains("busy")`);

// What the designer shows. The completions are random (random seeds), and
// so is their order (hardest first): of them only how many, whether one is
// selected and whether it keeps the drawing (see detail).
async function look(label, more = {}) {
  await sleep(100);
  const o = await js(`(() => {
    const $ = (id) => document.getElementById(id);
    const t = (e) => (e.hidden ? "[hidden] " : "") + e.textContent.trim().replace(/\\s+/g, " ");
    const btn = (id) => id + ":" + ($(id).hidden ? "H" : "") + ($(id).disabled ? "D" : "");
    const cells = [...$("canvas").children];
    const n = Math.round(Math.sqrt(cells.length));
    const rows = [];
    for (let r = 0; r < n; r++) rows.push(cells.slice(r * n, (r + 1) * n).map((c) => c.textContent || ".").join(""));
    const cands = [...$("cands").children];
    return {
      size: $("size").value, strict: $("strict").checked,
      palette: [...$("palette").children].map((b) => (b.getAttribute("aria-pressed") === "true" ? "*" : "") + b.textContent.trim()).join(" "),
      canvas: rows.join("/"),
      problem: cells.filter((c) => c.classList.contains("problem")).map((c) => c.dataset.r + "," + c.dataset.c).join(" "),
      status: $("status").className.replace("status", "").trim() + ": " + $("status").textContent,
      buttons: ["go", "stop", "clear"].map(btn).join(" | "),
      info: t($("info")), stale: t($("stale")).startsWith("[hidden]") ? "hidden" : "shown",
      cands: cands.length + (cands.length ? ", " + cands.filter((b) => b.getAttribute("aria-pressed") === "true").length + " selected" : "") + ($("cands").classList.contains("stale") ? ", faded" : ""),
      detail: $("detail").hidden ? "hidden" : "shown",
    };
  })()`);
  o.info = o.info.replace(/ · \d+ doppelt/, "").replace(/\d+(?:[.,]\d+)*/g, "#");
  record({ label, ...o, ...more });
}

// The selected completion: whether the cells drawn on the canvas kept their
// color, whether its numbers are there, what the big picture shows.
async function detail() {
  return js(`(() => {
    const $ = (id) => document.getElementById(id);
    const L = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz";
    const rows = new URL($("try").href).searchParams.get("p").split("/");
    const keeps = [...$("canvas").children].every((c) => !c.textContent || rows[+c.dataset.r][+c.dataset.c] === c.textContent);
    const svg = $("big").querySelector("svg");
    return {
      keepsDrawing: rows.length === +$("size").value && keeps,
      meta: /^Regeln allein: .*\\d+ Schritte · Nr\\. \\d+ · \\d+ ms$/.test($("detail-meta").textContent),
      stars: svg.querySelectorAll("circle.star").length,
      faded: svg.querySelectorAll("rect.undrawn").length,
      saveMsg: ($("save-msg").className ? $("save-msg").className + ": " : "") + $("save-msg").textContent,
    };
  })()`);
}

async function tick(sel) {
  await js(`document.getElementById(${JSON.stringify(sel)}).click()`);
  await checked();
}

export const scenarios = {
  async designer() {
    await go(DESIGNER + "/");
    await js(`localStorage.clear()`);
    await reload();
    await checked();
    await look("start");

    // Drawing: a drag paints with the chosen color; a click on a cell of
    // that color, a right click or the eraser frees it again.
    await pull([dc(0, 0), dc(0, 1), dc(1, 1), dc(1, 0)]);
    await checked();
    await look("drag A");
    await click(color(1));
    await pull([dc(6, 5), dc(6, 6), dc(6, 7)]);
    await checked();
    await look("drag B");
    await press(dc(6, 7));
    await checked();
    await look("click on B erases");
    await press(dc(1, 1), { right: true });
    await checked();
    await look("right click erases");
    await click(color(-1));
    await press(dc(1, 0));
    await checked();
    await look("eraser");
    await click(color(0));
    await press(dc(1, 0));
    await press(dc(1, 1));
    await checked();
    await reload();
    await checked();
    await look("reloaded");

    // The check while drawing: a part of C shut in by A and D cannot reach
    // the other part.
    await press(dc(1, 2));
    await click(color(3));
    await press(dc(0, 3));
    await click(color(2));
    await press(dc(0, 2));
    await press(dc(5, 0));
    await checked();
    await look("color cut off");
    for (const [r, c] of [[0, 2], [5, 0], [0, 3], [1, 2]]) await press(dc(r, c), { right: true });
    await checked();
    await look("cut off color erased");
    // Strict: a color in two parts is fine for the generator, which joins
    // them, but not when colors stay as drawn.
    await click(color(0));
    await press(dc(3, 7));
    await checked();
    await look("A in two parts");
    await tick("strict");
    await look("A in two parts, strict");
    await tick("strict");
    await press(dc(3, 7));
    await checked();

    // The search: completions arrive, the first is selected.
    await js(`document.getElementById("count").value = "2"`);
    await click("#go");
    await waitFor(`document.getElementById("stop").hidden && document.querySelectorAll("#cands .cand").length > 0`, 60000);
    await look("searched", await detail());
    await click("#show-solution");
    await click("#show-pattern");
    record({ label: "solution shown, pattern not highlighted", ...(await detail()) });
    await click("#show-pattern");
    await click("#show-solution");
    const first = await js(`document.getElementById("try").href`);
    await click(`#cands .cand[aria-pressed="false"]`);
    await look("other selected", { other: (await js(`document.getElementById("try").href`)) !== first });

    // Saving: with a name, the name again, the same puzzle without a name.
    await js(`document.getElementById("name").value = "Herz"`);
    await click("#save");
    await settle(`document.getElementById("save-msg").className`);
    record({ label: "saved", ...(await detail()) });
    await click("#save");
    await settle(`document.getElementById("save-msg").className === "err"`);
    record({ label: "saved again", ...(await detail()) });
    await js(`document.getElementById("name").value = ""`);
    await click("#save");
    await settle(`document.getElementById("save-msg").className === "err"`);
    record({ label: "saved again without a name", ...(await detail()) });
    await click(`#cands .cand[aria-pressed="false"]`);
    await click("#save");
    await settle(`document.getElementById("save-msg").className`);
    record({ label: "other saved without a name", ...(await detail()) });

    // A change of the drawing: the completions stay, marked as earlier.
    await click(color(2));
    await press(dc(4, 4));
    await checked();
    await look("drawn after the search");

    // "Ausprobieren": the completion as a puzzle page of the designer. A
    // browser new to its origin gets the tutorial first, as in serve, and
    // comes back to the puzzle when skipping it.
    const tryURL = await js(`document.getElementById("try").href`);
    const text = new URL(tryURL).searchParams.get("p");
    const tried = async (label) => record({
      label, path: await js(`location.pathname`),
      cells: await js(`document.querySelectorAll("#board .cell").length`),
      samePuzzle: (await js(`(() => {
        const e = document.getElementById("data"), d = e && JSON.parse(e.textContent);
        return d && d.regions ? d.regions.map((row) => row.map((v) => d.letters[v]).join("")).join("/") : "";
      })()`)) === text,
    });
    await go(tryURL);
    await waitFor(`location.pathname === "/anleitung"`).catch(() => {});
    await tried("tried: the tutorial first");
    await follow("#tutorial .head button");
    await tried("tried: tutorial skipped");
    await go(tryURL);
    await tried("tried again");
    await go(DESIGNER + "/");
    await checked();
    await look("back");

    // "Anhalten" while the answer is under way (a slow connection).
    await send("Network.emulateNetworkConditions", { offline: false, latency: 3000, downloadThroughput: -1, uploadThroughput: -1 });
    try {
      await click("#go");
      await sleep(300);
      await look("searching");
      await click("#stop");
      await settle(`document.getElementById("stop").hidden`);
      await look("stopped");
    } finally {
      await send("Network.emulateNetworkConditions", { offline: false, latency: 0, downloadThroughput: -1, uploadThroughput: -1 });
    }

    // Another size loses the drawing, after asking; so does "Leeren".
    const size = async (k) => {
      await js(`(() => { const s = document.getElementById("size"); s.value = "${k}"; s.dispatchEvent(new Event("change")); })()`);
      await checked();
    };
    await js(`window.confirm = () => false`);
    await size(10);
    await look("size declined");
    await js(`window.confirm = () => true`);
    await size(10);
    await look("size 10");
    await pull([dc(9, 0), dc(9, 1)]);
    await checked();
    await js(`window.confirm = () => false`);
    await click("#clear");
    await checked();
    await look("clear declined");
    await js(`window.confirm = () => true`);
    await click("#clear");
    await checked();
    await look("cleared");
  },
};
