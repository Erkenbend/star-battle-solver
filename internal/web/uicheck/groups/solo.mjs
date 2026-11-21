// UI check, group solo: the puzzle page played alone, marks, hints, checks,
// hypothesis, the solving path, "Neues Puzzle" and a page saved with
// `starbattle html`.
import { HTMLFILE, cell, click, data, dblclick, drag, fresh, go, js, key, more, moreClose, obs, once, record, reload, sleep, solveBy, tapDone } from "../lib.mjs";

// Those that run on a phone; the others run on a desktop screen.
export const PHONE_SCENARIOS = new Set(["solophone"]);

export const scenarios = {
  // Solo play: clicks, undo, drag, hints, checks, storage, options, watch mode.
  async solo() {
    await fresh();
    await go("/?n=8&seed=42");
    await obs("start");
    for (const i of [0, 1, 9]) await click(cell(i));
    await tapDone();
    await dblclick(cell(9)); // cross → star, one undo step
    await obs("clicks");
    await click("#undo");
    await obs("undo");
    await key("y", 2); // redo has no button
    await obs("redo");
    await key("z", 2);
    await obs("ctrl-z");
    await drag([cell(56), cell(57), cell(58), cell(59)]);
    await obs("drag crosses");
    await drag([cell(56), cell(57)]);
    await obs("drag erase");
    await click("#star-tool"); // a tap sets a star at once
    await click(cell(60));
    await obs("star tool");
    await click(cell(60));
    await click("#star-tool");
    await click(cell(60)); // a cross again
    await tapDone();
    await click(cell(60));
    await obs("star tool off");
    await click("#info-btn");
    await obs("info");
    await click("#info-btn");
    await click("#hint");
    await obs("hint");
    await click("#hint");
    await obs("hint hidden");
    await click("#hint");
    await click("#play-msg .actions button.primary");
    await obs("hint applied");
    const d = await data();
    const wrong = d.solution[0] === 0 ? 7 : 0;
    await click(cell(wrong), { shift: true });
    await obs("wrong star");
    await click("#check");
    await obs("check");
    await click("#play-msg .actions button");
    await obs("show mistakes");
    await click("#hint");
    await obs("hint with mistakes");
    await click(cell(wrong), { shift: true });
    await reload();
    await obs("reloaded");
    await more("#reset");
    await obs("reset");
    await js(`localStorage.setItem("starbattle.autocross", "1"); window.dispatchEvent(new StorageEvent("storage", { key: "starbattle.autocross" }))`);
    await click(cell(d.solution[0]), { shift: true });
    await obs("autocross via storage event");
    await js(`localStorage.setItem("starbattle.autocross", "0"); window.dispatchEvent(new StorageEvent("storage", { key: "starbattle.autocross" }))`);
    await obs("autocross off");
    await solveBy(d.solution, d.n, new Set([0]));
    await obs("solved");
    await click(cell(5));
    await obs("frozen click");
    await click("#tab-watch");
    await obs("watch");
    await key("ArrowRight");
    await key("ArrowRight");
    await obs("watch 2");
    await key("End");
    await obs("watch end");
    await key("Home");
    await click("#steps > li:nth-child(4) > .row");
    await obs("watch row 3");
    await click("#tab-play");
    await obs("back to play");
    await go("/?n=8&seed=42#weg");
    await obs("opened with #weg");
  },

  // The same on a phone, where the page is shorter than the screen: what
  // comes with a tap (a hint, a check, the banner of the solve) must not
  // move the board, the banner must be on screen, and the action bar must
  // neither lie under the tab bar nor cut its words.
  async solophone() {
    await scenarios.solo();
  },

  // "Neues Puzzle": one size, or a random one of several ("Mehrere Größen"),
  // remembered on this device. New puzzles have random seeds, so only their
  // size is recorded.
  async sizes() {
    await fresh();
    await go("/?n=8&seed=42");
    const look = async (label, { random = false, anyN = false } = {}) => record({ label, ...(await js(`(() => {
      const s = document.getElementById("newgame-n"), p = document.getElementById("newgame-pool");
      return {
        path: location.pathname + location.search${random ? `.replace(/seed=\\d+/, "seed=RANDOM")` : ""}${anyN ? `.replace(/n=\\d+/, "n=N")` : ""},
        select: s.value,
        pool: (p.hidden ? "[hidden] " : "") + [...p.querySelectorAll("input:checked")].map((b) => b.value).join(","),
        sent: new URLSearchParams(new FormData(document.getElementById("newgame"))).toString(),
      };
    })()`)) });
    const choose = (v) => js(`(() => { const s = document.getElementById("newgame-n"); s.value = ${JSON.stringify(v)}; s.dispatchEvent(new Event("change")); })()`);
    const box = (k) => `#newgame-pool input[value="${k}"]`;
    const submit = async () => {
      const loaded = once("Page.loadEventFired");
      await more("#newgame button");
      await loaded;
      await sleep(250);
    };
    await look("one size");
    await choose("10");
    await submit();
    await look("new 10×10", { random: true });
    await choose("mix");
    await look("several sizes", { random: true });
    for (const k of [6, 7, 9]) await more(box(k));
    await more(box(8)); // the last size stays
    await more(box(5));
    await look("pool 5 and 8", { random: true });
    await submit();
    const n = await js(`new URLSearchParams(location.search).get("n")`);
    record({ label: "new from the pool", fromPool: n === "5" || n === "8" });
    await look("pool remembered", { random: true, anyN: true });
    await choose("6");
    await go("/?n=8&seed=42");
    await look("one size again");
    // "Neues Puzzle" beside the title: one tap, the size of the select.
    const loaded = once("Page.loadEventFired");
    await click("#newgame-go");
    await loaded;
    await sleep(250);
    await look("new in one tap", { random: true });
  },

  // Hypothesis mode: set, undo, reload, contradiction, discard, solve in it.
  async hypothesis() {
    await fresh();
    await go("/?n=8&seed=42");
    const d = await data();
    await click(cell(d.solution[0]), { shift: true });
    await click("#hyp");
    await obs("hyp start");
    const bad = 8 + ((d.solution[1] + 3) % 8);
    await click(cell(bad));
    await tapDone();
    await dblclick(cell(bad)); // cross → star, the hypothesis' first star
    await obs("hyp star");
    await click(cell(bad + 8));
    await obs("hyp cross");
    await click("#undo");
    await obs("hyp undo");
    await key("y", 2); // redo has no button
    await obs("hyp redo");
    await reload();
    await obs("hyp reloaded");
    await click("#hyp-contra");
    await obs("contradiction");
    await click("#undo");
    await obs("undo contradiction");
    await click("#hyp");
    await click(cell(bad), { shift: true });
    await click("#hyp-drop");
    await obs("dropped");
    await click("#hyp");
    await solveBy(d.solution, d.n, new Set([0]));
    await obs("solved in hyp");
    await more("#reset");
    await obs("reset");
  },

  // A page saved with `starbattle html`: options on the page itself.
  async file() {
    await go("file://" + HTMLFILE);
    await js(`localStorage.clear()`);
    await reload();
    await obs("file page");
    await more("#autocross");
    const d = await data();
    await click(cell(d.solution[0]), { shift: true }); // "Mehr" open: the board takes taps all the same
    await obs("autocross on");
    await more("#showproblems");
    await moreClose();
    await click(cell(d.solution[0] + 1), { shift: true });
    await obs("problems off");
    await more("#fadecolorsonly"); // a part of the option below: not yet
    await more("#fadedone");
    await obs("fade done on");
    await more("#fadecolorsonly");
    await moreClose();
    await click(cell(d.n + d.solution[1]), { shift: true }); // finishes its color
    await obs("fade colors only");
    await more("#fadedone"); // switches its part off with it
    await obs("fade off, colors only kept");
    await more("#fadedone");
    await more("#fadecolorsonly");
    await more("#autocross");
    await more("#showproblems");
    await more("#fadedone");
  },
};
