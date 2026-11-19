// Behaviour check of the puzzle page (UI check): plays every mode in a
// headless Chromium via the DevTools protocol (node >= 22, no packages),
// records what the page shows after each action and compares that with
// expected.jsonl. Normally started by run.sh, which builds starbattle and
// starts the server and Chromium first.
//
// Usage: node uicheck.mjs <base-url> <devtools-host:port> <html-file> <out-file> [-update] [scenario...]
//
// Every line of expected.jsonl is one observation: the board's cell classes,
// banner, messages, buttons, clock, steps of the solving path and so on, or
// the server's view of a party ("server: …"). Values that differ between
// runs of the same build are normalized when recorded: times, party codes,
// the random puzzles of parties (their boards as the classes that occur, no
// rule violations, steps or problems) and the random puzzle after the tutorial. A failed
// step or a JavaScript error is an observation too, so it shows as a
// difference.
//
// With -update, the observations replace expected.jsonl (of the scenarios
// run). Do that after an intended change of the page, and check its diff.
import fs from "node:fs";
import path from "node:path";

const args = process.argv.slice(2);
const UPDATE = args.includes("-update");
const [BASE, DEVTOOLS, HTMLFILE, OUTFILE, ...ONLY] = args.filter((a) => a !== "-update");
const EXPECTED = path.join(path.dirname(new URL(import.meta.url).pathname), "expected.jsonl");
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// ---------- DevTools protocol ----------
// A new tab, retried for a while: on a busy machine the browser may answer
// its port before it is ready (ECONNREFUSED, an empty reply).
async function newTab() {
  for (const t0 = Date.now(); ; await sleep(500)) {
    try {
      const r = await fetch(`http://${DEVTOOLS}/json/new?about:blank`, { method: "PUT" });
      if (r.ok) return await r.json();
      throw new Error(`HTTP ${r.status}`);
    } catch (e) {
      if (Date.now() - t0 > 30000) throw new Error(`no browser at ${DEVTOOLS}: ${e.cause?.code || e.message}`);
    }
  }
}
const target = await newTab().catch((e) => {
  console.error(e.message);
  process.exit(2);
});
const ws = new WebSocket(target.webSocketDebuggerUrl);
let seq = 0;
const pending = new Map();
const listeners = [];
const errors = [];
ws.onmessage = (m) => {
  const d = JSON.parse(m.data);
  if (d.id && pending.has(d.id)) {
    const { res, rej } = pending.get(d.id);
    pending.delete(d.id);
    d.error ? rej(new Error(JSON.stringify(d.error))) : res(d.result);
  } else if (d.method) {
    if (d.method === "Runtime.exceptionThrown") errors.push(d.params.exceptionDetails.exception?.description || d.params.exceptionDetails.text);
    if (d.method === "Runtime.consoleAPICalled" && d.params.type === "error") errors.push(d.params.args.map((a) => a.value ?? a.description).join(" "));
    for (const l of [...listeners]) l(d);
  }
};
await new Promise((r, rej) => {
  ws.onopen = r;
  ws.onerror = () => rej(new Error("cannot connect to the browser's tab"));
});
// The browser gone in the middle: fail now instead of waiting for timeouts.
ws.onclose = () => {
  console.error("the connection to the browser closed");
  process.exit(2);
};
const send = (method, params = {}) => new Promise((res, rej) => { pending.set(++seq, { res, rej }); ws.send(JSON.stringify({ id: seq, method, params })); });
// Waits for the next event of this kind, at most ms: a load that never comes
// must not hang the run.
const once = (method, ms = 15000) => new Promise((r, rej) => {
  const l = (d) => { if (d.method === method) { listeners.splice(listeners.indexOf(l), 1); clearTimeout(t); r(d.params); } };
  const t = setTimeout(() => { listeners.splice(listeners.indexOf(l), 1); rej(new Error("no " + method)); }, ms);
  listeners.push(l);
});

await send("Page.enable");
await send("Runtime.enable");
await send("Emulation.setDeviceMetricsOverride", { width: 1200, height: 900, deviceScaleFactor: 1, mobile: false });

async function js(expr) {
  const r = await send("Runtime.evaluate", { expression: expr, awaitPromise: true, returnByValue: true });
  if (r.exceptionDetails) throw new Error(expr + ": " + JSON.stringify(r.exceptionDetails));
  return r.result.value;
}
// Loads url afresh. A URL that differs from the current one only in its #
// would just scroll, so it goes through about:blank.
async function go(url) {
  const full = url.startsWith("file:") ? url : BASE + url;
  const here = await js("location.href");
  if (here.split("#")[0] === full.split("#")[0] && full.includes("#")) {
    const blank = once("Page.loadEventFired");
    await send("Page.navigate", { url: "about:blank" });
    await blank;
  }
  const loaded = once("Page.loadEventFired");
  await send("Page.navigate", { url: full });
  await loaded;
  await sleep(250);
}
async function reload() {
  const loaded = once("Page.loadEventFired");
  await send("Page.reload");
  await loaded;
  await sleep(250);
}
// Waits until expr holds in the page; fails the scenario after ms.
async function waitFor(expr, ms = 20000) {
  const t0 = Date.now();
  while (Date.now() - t0 < ms) {
    if (await js(`!!(${expr})`)) return;
    await sleep(50);
  }
  throw new Error("timeout: " + expr);
}
// Waits until expr holds in the page, at most ms, and goes on either way: the
// observation that follows shows whether it happened.
async function settle(expr, ms = 8000) {
  const t0 = Date.now();
  while (Date.now() - t0 < ms && !(await js(`!!(${expr})`))) await sleep(50);
}
async function pos(sel) {
  const r = await js(`(() => { const e = document.querySelector(${JSON.stringify(sel)}); if (!e) return null; e.scrollIntoView({block: "nearest"}); const b = e.getBoundingClientRect(); return [b.x + b.width / 2, b.y + b.height / 2]; })()`);
  if (!r) throw new Error("no element " + sel);
  return r;
}
async function click(sel, { shift = false } = {}) {
  const [x, y] = await pos(sel);
  const modifiers = shift ? 8 : 0;
  await send("Input.dispatchMouseEvent", { type: "mouseMoved", x, y, modifiers });
  await send("Input.dispatchMouseEvent", { type: "mousePressed", x, y, button: "left", buttons: 1, clickCount: 1, modifiers });
  await send("Input.dispatchMouseEvent", { type: "mouseReleased", x, y, button: "left", buttons: 0, clickCount: 1, modifiers });
  await sleep(60);
}
const tapDone = () => settle(`!document.querySelector("#board[data-pending]")`, 2000); // a click waiting for its double
async function dblclick(sel) {
  await click(sel);
  await click(sel);
}
async function drag(sels) {
  const ps = [];
  for (const s of sels) ps.push(await pos(s));
  await send("Input.dispatchMouseEvent", { type: "mouseMoved", x: ps[0][0], y: ps[0][1] });
  await send("Input.dispatchMouseEvent", { type: "mousePressed", x: ps[0][0], y: ps[0][1], button: "left", buttons: 1, clickCount: 1 });
  for (const [x, y] of ps.slice(1)) {
    await send("Input.dispatchMouseEvent", { type: "mouseMoved", x, y, button: "left", buttons: 1 });
    await sleep(30);
  }
  const [x, y] = ps[ps.length - 1];
  await send("Input.dispatchMouseEvent", { type: "mouseReleased", x, y, button: "left", buttons: 0, clickCount: 1 });
  await sleep(60);
}
async function key(k, mods = 0) {
  const codes = { ArrowRight: 39, ArrowLeft: 37, Home: 36, End: 35, " ": 32, z: 90, y: 89, Escape: 27 };
  await send("Input.dispatchKeyEvent", { type: "keyDown", key: k, windowsVirtualKeyCode: codes[k], modifiers: mods });
  await send("Input.dispatchKeyEvent", { type: "keyUp", key: k, windowsVirtualKeyCode: codes[k], modifiers: mods });
  await sleep(60);
}
const cell = (i) => `#board .cell[data-i="${i}"]`;
const hasClass = (i, c) => `document.querySelector('${cell(i)}').classList.contains("${c}")`;
const bannerHas = (t) => `document.getElementById("banner").textContent.includes(${JSON.stringify(t)})`;
async function fresh() {
  await go("/party"); // any page of the origin
  await js(`localStorage.clear(); localStorage.setItem("starbattle.tutorial", "1")`); // no tutorial first
}
async function api(p, body) {
  const r = await fetch(BASE + p, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  const v = await r.json();
  if (!r.ok) throw new Error(p + ": " + JSON.stringify(v));
  return v;
}
const data = () => js(`JSON.parse(document.getElementById("data").textContent)`);

// ---------- observations ----------
const observed = [];
let scenario = "";
let party = false; // a party scenario: random puzzle
function record(o) {
  observed.push(JSON.stringify({ scenario, ...o }));
}

// What the page shows, normalized (see the top).
async function obs(label, { random = false } = {}) {
  await sleep(120);
  await tapDone();
  const o = await js(`(() => {
    const q = (s) => document.querySelector(s);
    const t = (e) => e ? (e.hidden ? "[hidden] " : "") + e.textContent.trim().replace(/\\s+/g, " ") : null;
    const txt = (id) => t(document.getElementById(id));
    const btn = (id) => { const e = document.getElementById(id); return e ? id + ":" + (e.hidden ? "H" : "") + (e.disabled ? "D" : "") + ":" + e.textContent.trim().replace(/\\s+/g, " ") : id + ":-"; };
    const cls = ["star", "cross", "auto", "hint", "bad", "hyp", "hyproot", "new", "dim", "goal", "refcell", "conflict", "remote", "done"];
    return {
      path: location.pathname + location.search + location.hash,
      body: document.body.className, board: q("#board") && q("#board").className,
      cells: [...document.querySelectorAll("#board .cell")].map((e) => cls.filter((c) => e.classList.contains(c)).join("+") || "."),
      banner: txt("banner"), msg: txt("play-msg"), problems: txt("problems"), summary: txt("summary"),
      clock: txt("clock"), legend: txt("legend"), current: txt("current"),
      steps: document.querySelectorAll("#steps .row").length, active: t(q("#steps .row.active .num")),
      tutorial: txt("tutorial"), tour: t(q(".tour-tip")), tourTarget: q(".tour-target") && q(".tour-target").id,
      buttons: ["undo", "redo", "reset", "hint", "check", "hyp", "hyp-contra", "hyp-drop", "giveup", "tab-play", "tab-watch", "first", "prev", "next", "last", "newgame", "site-nav", "party-box", "options-link", "play-options"].map(btn).join(" | "),
      selected: [...document.querySelectorAll("[aria-selected=true], [aria-current=page]")].map((e) => e.id).join(","),
      party: t(q("#party-panel")),
    };
  })()`);
  if (party) { // a random puzzle: which classes occur, not where, and no rule violations
    o.cells = [...new Set(o.cells.flatMap((c) => c.split("+")))].filter((c) => c !== "." && c !== "bad").sort();
    delete o.steps;
    delete o.problems;
  } else {
    o.cells = o.cells.join(" ");
  }
  if (random) { // the random puzzle after the tutorial: only where it is
    for (const k of Object.keys(o)) if (k !== "path") delete o[k];
    o.path = o.path.replace(/seed=\d+/, "seed=RANDOM");
  }
  const norm = (s) => {
    if (typeof s !== "string") return s;
    s = s.replace(/\b\d+:\d\d(:\d\d)?\b/g, "T").replace(/\/party\/[A-Z]{4}/g, "/party/CODE").replace(/Party [A-Z]{4}/g, "Party CODE");
    s = s.replace(/^\/.*\/(p\.html)$/, "$1");
    if (party) s = s.replace(/\d+/g, "#");
    return s;
  };
  for (const k of Object.keys(o)) o[k] = norm(o[k]);
  record({ label, ...o });
}

// The server's view of Alice (the browser) in party p, as Bob's client gets
// it. With until, waits (at most a few seconds) until it holds. With board,
// her stars are compared with those on the page (without a hypothesis), for
// steps whose effect depends on the random puzzle (a hint).
async function server(p, label, until = () => true, { board = false } = {}) {
  let s, alice;
  for (const t0 = Date.now(); Date.now() - t0 < 8000; await sleep(100)) {
    const r = await fetch(`${BASE}/party/${p.code}/join`, { method: "POST", body: JSON.stringify({ token: p.token }) });
    s = (await r.json()).snapshot;
    alice = s && s.ranking.find((e) => e.name === "Alice");
    if (s && until(alice || {}, s)) break;
  }
  const coop = s && s.coop ? [...s.coop.board].filter((c) => c !== "0").sort().join("") : undefined;
  const a = alice && { status: alice.status, stars: alice.stars, hints: alice.hints, errors: alice.errors, solved: alice.solved, skips: alice.skips, puzzle: alice.puzzle, moves: alice.moves };
  if (a && board) {
    const onBoard = await js(`document.querySelectorAll("#board .cell.star:not(.hyp)").length`);
    a.stars = a.stars === onBoard ? "as on the board" : `${a.stars}, on the board ${onBoard}`;
  }
  record({ label: "server: " + label, phase: s && s.phase, alice: a, coop });
}

async function solveBy(sol, n, skip = new Set()) {
  for (let r = 0; r < n; r++) if (!skip.has(r)) await click(cell(r * n + sol[r]), { shift: true });
}

// A party in mode with Alice in the browser and Bob via the API, up to the
// start of round 1 (5×5 puzzles); options are more fields of the form.
async function partyStart(mode, options = {}) {
  await fresh();
  const r = await fetch(BASE + "/party", { method: "POST", body: new URLSearchParams({ n: "5", mode, ...options }), redirect: "manual" });
  const code = r.headers.get("location").split("/").pop();
  const bob = await api(`/party/${code}/join`, { name: "Bob" });
  await js(`localStorage.setItem("starbattle.name", "Alice")`);
  await go("/party/" + code);
  await click("#join button[type=submit]");
  await waitFor(`[...document.querySelectorAll("#party-panel button")].some((b) => b.textContent === "Bereit") && !document.querySelector("#party-panel .actions .muted")`);
  await api(`/party/${code}/ready`, { token: bob.token, ready: true });
  const loaded = once("Page.loadEventFired");
  await js(`[...document.querySelectorAll("#party-panel button")].find((b) => b.textContent === "Bereit").click()`);
  await loaded;
  await sleep(200);
  await obs("countdown");
  await waitFor(`!document.body.classList.contains("party-wait") && document.querySelector("#board .cell")`);
  const d = await data();
  return { code, token: bob.token, sol: d.solution || (await solutionOf(d.text)), n: d.n };
}

// The solution of a puzzle whose page has none (against the clock), from the
// server's /play page of the same puzzle.
async function solutionOf(text) {
  const html = await (await fetch(BASE + "/play?p=" + encodeURIComponent(text.trim().split("\n").join("/")))).text();
  const m = html.match(/<script id="data" type="application\/json">(.*?)<\/script>/s);
  return JSON.parse(m[1]).solution;
}

// ---------- scenarios ----------
const scenarios = {
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
    await click("#redo");
    await obs("redo");
    await key("z", 2);
    await obs("ctrl-z");
    await drag([cell(56), cell(57), cell(58), cell(59)]);
    await obs("drag crosses");
    await drag([cell(56), cell(57)]);
    await obs("drag erase");
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
    await click("#reset");
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
      await click("#newgame button");
      await loaded;
      await sleep(250);
    };
    await look("one size");
    await choose("10");
    await submit();
    await look("new 10×10", { random: true });
    await choose("mix");
    await look("several sizes", { random: true });
    for (const k of [6, 7, 9]) await click(box(k));
    await click(box(8)); // the last size stays
    await click(box(5));
    await look("pool 5 and 8", { random: true });
    await submit();
    const n = await js(`new URLSearchParams(location.search).get("n")`);
    record({ label: "new from the pool", fromPool: n === "5" || n === "8" });
    await look("pool remembered", { random: true, anyN: true });
    await choose("6");
    await go("/?n=8&seed=42");
    await look("one size again");
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
    await click("#redo");
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
    await click("#reset");
    await obs("reset");
  },

  // A page saved with `starbattle html`: options on the page itself.
  async file() {
    await go("file://" + HTMLFILE);
    await js(`localStorage.clear()`);
    await reload();
    await obs("file page");
    await click("#autocross");
    const d = await data();
    await click(cell(d.solution[0]), { shift: true });
    await obs("autocross on");
    await click("#showproblems");
    await click(cell(d.solution[0] + 1), { shift: true });
    await obs("problems off");
    await click("#fadecolorsonly"); // a part of the option below: not yet
    await click("#fadedone");
    await obs("fade done on");
    await click("#fadecolorsonly");
    await click(cell(d.n + d.solution[1]), { shift: true }); // finishes its color
    await obs("fade colors only");
    await click("#fadedone"); // switches its part off with it
    await obs("fade off, colors only kept");
    await click("#fadedone");
    await click("#fadecolorsonly");
    await click("#autocross");
    await click("#showproblems");
    await click("#fadedone");
  },

  // Library: a hard puzzle without help (trace only once solved), a motif.
  async library() {
    await fresh();
    await go("/bibliothek");
    const parts = await js(`JSON.parse(document.getElementById("data").textContent).parts`);
    const hard = parts.find((p) => p.kind === "schwer").entries[0];
    const motif = parts.find((p) => p.kind === "motive").entries[0];
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

  // A party round against each other: progress, hints, hypothesis (not
  // reported), solving, the end of the round; then giving up.
  async versus() {
    party = true;
    const p = await partyStart("versus");
    await obs("started");
    await click(cell(p.sol[0]), { shift: true });
    await click(cell(p.n + p.sol[1]), { shift: true });
    await obs("two stars");
    await server(p, "after two stars", (me) => me.stars === 2);
    await click("#hint");
    await click("#play-msg .actions button.primary");
    await server(p, "after hint", (me) => me.hints === 1, { board: true });
    await click("#hyp");
    await click(cell(2 * p.n + ((p.sol[2] + 2) % p.n)), { shift: true });
    await sleep(1200); // what must not be reported: time for a report to come
    await obs("hyp in party");
    await server(p, "hyp not reported", undefined, { board: true });
    await click("#hyp-drop");
    await click("#reset");
    await obs("reset in party");
    await solveBy(p.sol, p.n);
    await waitFor(`!document.getElementById("tab-watch").hidden`);
    await obs("solved");
    await server(p, "solved", (me) => me.status === "solved");
    await api(`/party/${p.code}/giveup`, { token: p.token, round: 1, hints: 0 });
    await settle(bannerHas("vorbei") + " || document.querySelector('#party-panel').textContent.includes('Ergebnis')");
    await obs("round over");
    await reload();
    await obs("reloaded after round");

    const q = await partyStart("versus");
    await js(`window.confirm = () => true`);
    await click(cell(q.sol[0]), { shift: true });
    await click("#giveup");
    await settle(bannerHas("Aufgegeben"));
    await obs("gave up");
    await server(q, "gave up", (me) => me.status === "gaveup");
  },

  // Against each other with 2 lives: a star where the solution has none
  // costs one, once per cell, even if it breaks no rule; a cross on a star of
  // the solution and a star in a hypothesis cost none; the second wrong star
  // ends the round for Alice.
  async lives() {
    party = true;
    const p = await partyStart("versus", { lives: "2" });
    await obs("started");
    const c0 = p.sol[0];
    const wrongIn = (r) => r * p.n + [...Array(p.n).keys()].find((c) => c !== p.sol[r] && c !== c0);
    await click(cell(c0), { shift: true }); // right
    await click(cell(wrongIn(2)), { shift: true }); // two rows further: no rule against it
    await obs("wrong star");
    await server(p, "one error", (me) => me.errors === 1);
    await click("#undo");
    await obs("undone");
    await click("#redo");
    await sleep(1200); // what must not count: time for a report to come
    await obs("redone");
    await server(p, "redo not counted");
    await click("#undo");
    await click(cell(p.n + p.sol[1])); // a cross on a star of the solution
    await tapDone();
    await click("#hyp");
    await click(cell(wrongIn(3)), { shift: true });
    await sleep(1200);
    await obs("wrong star in hyp");
    await server(p, "cross and hyp not counted");
    await click("#hyp-drop");
    await click(cell(wrongIn(3)), { shift: true });
    await settle(bannerHas("Alle Leben verloren"));
    await obs("out of lives");
    record({ label: "out of lives: wrong star in red", bad: await js(`document.querySelectorAll("#board .cell.bad").length === 1`) });
    await server(p, "out", (me) => me.status === "out");
    await reload();
    await obs("reloaded when out");
    await click("#reveal");
    await obs("solution shown");
    await reload();
    await obs("reloaded with solution");
  },

  // A coop round: one board for both, changes both ways, undo, a
  // hypothesis that stays local, giving up by vote; then solving together.
  async coop() {
    party = true;
    const p = await partyStart("coop");
    await obs("started");
    await click(cell(p.sol[0]));
    await tapDone();
    await dblclick(cell(p.sol[0])); // cross → star, one undo step
    await server(p, "browser star", (me, s) => s.coop.board.includes("2"));
    const other = p.n + ((p.sol[1] + 2) % p.n);
    await api(`/party/${p.code}/mark`, { token: p.token, round: 1, ops: [[1, other, 0, 1]] });
    await waitFor(hasClass(other, "cross"));
    await obs("remote cross");
    await click("#undo");
    await settle(`!${hasClass(p.sol[0], "star")}`);
    await obs("undo own");
    await server(p, "after undo", (me, s) => !s.coop.board.includes("2") && s.coop.board[p.sol[0]] === "1");
    await click("#redo");
    await settle(hasClass(p.sol[0], "star"));
    await obs("redo own");
    await server(p, "after redo", (me, s) => s.coop.board.includes("2"));
    await click("#undo");
    await click("#redo");
    await server(p, "after undo and redo again", (me, s) => s.coop.board.includes("2"));
    await click("#hyp");
    await click(cell(2 * p.n + p.sol[2]), { shift: true });
    await sleep(1200); // what must not be sent: time for it to arrive
    await server(p, "hyp stays local");
    await api(`/party/${p.code}/mark`, { token: p.token, round: 1, ops: [[2, 3 * p.n + p.sol[3], 0, 2]] });
    await settle(hasClass(3 * p.n + p.sol[3], "star"));
    await obs("remote under hyp");
    await click("#hyp-drop");
    await js(`window.confirm = () => true`);
    await click("#giveup");
    await settle(`document.getElementById("giveup").textContent === "Doch nicht aufgeben"`);
    await obs("voted");
    await api(`/party/${p.code}/vote`, { token: p.token, round: 1, giveUp: true });
    await settle(bannerHas("aufgegeben"));
    await obs("given up together");

    const q = await partyStart("coop");
    await solveBy(q.sol, q.n);
    await settle(bannerHas("Gemeinsam gelöst"));
    await obs("solved together");
  },

  // Against the clock: progress, solving (on to puzzle 2), reload, skipping.
  async race() {
    party = true;
    const p = await partyStart("clock");
    await obs("started");
    await click(cell(p.sol[0]), { shift: true });
    await click(cell(p.n + p.sol[1]), { shift: true });
    await server(p, "progress", (me) => me.stars === 2);
    const loaded = once("Page.loadEventFired");
    await solveBy(p.sol, p.n, new Set([0, 1]));
    await loaded;
    await waitFor(`document.querySelector("#board .cell") && !document.body.classList.contains("party-wait")`);
    await obs("puzzle 2");
    const d2 = await data();
    await click(cell(d2.n + 1), { shift: true });
    await reload();
    await obs("puzzle 2 reloaded");
    const loaded2 = once("Page.loadEventFired");
    await click("#giveup");
    await loaded2;
    await waitFor(`document.querySelector("#board .cell")`);
    await obs("skipped to 3");
    await server(p, "after skip", (me) => me.puzzle === 3);
  },
};

for (const [name, f] of Object.entries(scenarios)) {
  if (ONLY.length && !ONLY.includes(name)) continue;
  scenario = name;
  party = false;
  try {
    await f();
  } catch (e) {
    record({ label: "FAILED", error: String(e.message || e).slice(0, 300) });
  }
  if (errors.length) record({ label: "JS ERRORS", errors: errors.splice(0) });
}
ws.onclose = null;
ws.close();

// ---------- compare with expected.jsonl ----------
const ran = new Set(ONLY.length ? ONLY : Object.keys(scenarios));
const old = fs.existsSync(EXPECTED) ? fs.readFileSync(EXPECTED, "utf8").split("\n").filter(Boolean) : [];
const keep = (lines) => lines.filter((l) => ran.has(JSON.parse(l).scenario));
fs.writeFileSync(OUTFILE, observed.join("\n") + "\n");

if (UPDATE) {
  // The scenarios run replace theirs, the others stay, in scenario order.
  const order = Object.keys(scenarios);
  const all = [...old.filter((l) => !ran.has(JSON.parse(l).scenario)), ...observed];
  all.sort((a, b) => order.indexOf(JSON.parse(a).scenario) - order.indexOf(JSON.parse(b).scenario));
  fs.writeFileSync(EXPECTED, all.join("\n") + "\n");
  console.log(`expected.jsonl: ${observed.length} observations of ${[...ran].join(", ")} written`);
  process.exit(0);
}

// Observations by scenario and label; a label that occurs again (e.g. the
// countdown of a second party) gets its number.
function byID(lines) {
  const m = new Map(), seen = new Map();
  for (const o of lines.map((l) => JSON.parse(l))) {
    let k = o.scenario + " / " + o.label;
    const c = (seen.get(k) || 0) + 1;
    seen.set(k, c);
    if (c > 1) k += ` (${c})`;
    m.set(k, o);
  }
  return m;
}
const wantBy = byID(keep(old));
const gotBy = byID(observed);
let diffs = 0;
for (const k of wantBy.keys()) if (!gotBy.has(k)) { diffs++; console.log(`missing: ${k}`); }
for (const [k, o] of gotBy) {
  const w = wantBy.get(k);
  if (!w) { diffs++; console.log(`unexpected: ${k}\n  ${JSON.stringify(o).slice(0, 400)}`); continue; }
  for (const f of new Set([...Object.keys(w), ...Object.keys(o)])) {
    const a = JSON.stringify(w[f]), b = JSON.stringify(o[f]);
    if (a !== b) {
      diffs++;
      console.log(`${k} / ${f}:\n  want ${String(a).slice(0, 400)}\n  got  ${String(b).slice(0, 400)}`);
    }
  }
}
console.log(`${observed.length} observations, ${diffs} differences`);
process.exit(diffs ? 1 : 0);
