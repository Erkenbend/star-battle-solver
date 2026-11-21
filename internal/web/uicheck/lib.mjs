// Helpers of the UI check (see uicheck.mjs): the DevTools protocol, input,
// waiting, and the observations of the puzzle page. The groups in groups/
// import what they need; uicheck.mjs connects first and runs them.
export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// Set by connect: the server (serve), the designer (design) and the page of
// a puzzle saved with `starbattle html`.
export let BASE = "", DESIGNER = "", HTMLFILE = "";

// What the running scenario is: its name, whether it shows a random party
// puzzle (party, set by the scenario) and whether it runs on a phone.
export const state = { scenario: "", party: false, phone: false };

// ---------- DevTools protocol ----------
let ws, seq = 0;
const pending = new Map();
const listeners = [];
// JavaScript errors of the page, and page loads it aborted itself (see
// visited below): uicheck.mjs records them after each scenario.
export const errors = [];

// A new tab, retried for a while: on a busy machine the browser may answer
// its port before it is ready (ECONNREFUSED, an empty reply).
async function newTab(devtools) {
  for (const t0 = Date.now(); ; await sleep(500)) {
    try {
      const r = await fetch(`http://${devtools}/json/new?about:blank`, { method: "PUT" });
      if (r.ok) return await r.json();
      throw new Error(`HTTP ${r.status}`);
    } catch (e) {
      if (Date.now() - t0 > 30000) throw new Error(`no browser at ${devtools}: ${e.cause?.code || e.message}`);
    }
  }
}

export const send = (method, params = {}) => new Promise((res, rej) => { pending.set(++seq, { res, rej }); ws.send(JSON.stringify({ id: seq, method, params })); });
// Waits for the next event of this kind, at most ms: a load that never comes
// must not hang the run.
export const once = (method, ms = 15000) => new Promise((r, rej) => {
  const l = (d) => { if (d.method === method) { listeners.splice(listeners.indexOf(l), 1); clearTimeout(t); r(d.params); } };
  const t = setTimeout(() => { listeners.splice(listeners.indexOf(l), 1); rej(new Error("no " + method)); }, ms);
  listeners.push(l);
});

export const DESKTOP = { width: 1200, height: 900, deviceScaleFactor: 1, mobile: false };
export const PHONE = { width: 393, height: 851, deviceScaleFactor: 2.75, mobile: true };
// The pages the tab went to (requested, also those aborted by a new one).
// A load the page started itself and aborted is an error in every
// scenario: setting location again while a load is under way starts it
// over, and with a slow connection the page may never arrive.
export const visited = [], aborted = [];
const byScript = new Set();

// Opens a tab in the browser at devtools and gets it ready.
export async function connect({ base, designer, devtools, htmlFile }) {
  BASE = base;
  DESIGNER = designer;
  HTMLFILE = htmlFile;
  const target = await newTab(devtools);
  ws = new WebSocket(target.webSocketDebuggerUrl);
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
  await send("Page.enable");
  await send("Runtime.enable");
  await send("Emulation.setDeviceMetricsOverride", DESKTOP);
  listeners.push((d) => {
    if (d.method === "Network.requestWillBeSent" && d.params.type === "Document") {
      visited.push(d.params.request.url.replace(BASE, ""));
      if (d.params.initiator.type === "script") byScript.add(d.params.requestId);
    }
    if (d.method === "Network.loadingFailed" && d.params.canceled && byScript.has(d.params.requestId)) aborted.push(d.params.requestId);
  });
  await send("Network.enable");
}

export function disconnect() {
  ws.onclose = null;
  ws.close();
}


export async function js(expr) {
  const r = await send("Runtime.evaluate", { expression: expr, awaitPromise: true, returnByValue: true });
  if (r.exceptionDetails) throw new Error(expr + ": " + JSON.stringify(r.exceptionDetails));
  return r.result.value;
}
// Loads url afresh. A URL that differs from the current one only in its #
// would just scroll, so it goes through about:blank.
export async function go(url) {
  const full = /^(file|https?):/.test(url) ? url : BASE + url;
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
export async function reload() {
  const loaded = once("Page.loadEventFired");
  await send("Page.reload");
  await loaded;
  await sleep(250);
}
// Waits until expr holds in the page; fails the scenario after ms.
export async function waitFor(expr, ms = 20000) {
  const t0 = Date.now();
  while (Date.now() - t0 < ms) {
    if (await js(`!!(${expr})`)) return;
    await sleep(50);
  }
  throw new Error("timeout: " + expr);
}
// Waits until expr holds in the page, at most ms, and goes on either way: the
// observation that follows shows whether it happened.
export async function settle(expr, ms = 8000) {
  const t0 = Date.now();
  while (Date.now() - t0 < ms && !(await js(`!!(${expr})`))) await sleep(50);
}
export async function pos(sel) {
  const r = await js(`(() => { const e = document.querySelector(${JSON.stringify(sel)}); if (!e) return null; e.scrollIntoView({block: "nearest"}); const b = e.getBoundingClientRect(); return [b.x + b.width / 2, b.y + b.height / 2]; })()`);
  if (!r) throw new Error("no element " + sel);
  return r;
}
// On a phone (the scenarios in a group's PHONE_SCENARIOS), the board must
// stay where it is on screen when the player acts on it: what changed above
// it (a banner, the tutorial's card, the tabs appearing) would make the next
// tap land elsewhere, and on the solving tap the page would jump. The same
// holds for the designer (a desktop tool) everywhere: its drawing (#canvas)
// and the picture a color is painted on (#big). Every move is an error of
// the scenario.
export const moves = [];
const stillBoard = (sel) => {
  const id = String(sel).match(/^#(board|canvas|big)\b/)?.[1];
  return id === "board" && !state.phone ? null : id;
};
export const boardAt = (id = "board") => js(`(() => { const b = document.getElementById(${JSON.stringify(id)}); return b ? location.href + " " + Math.round(b.getBoundingClientRect().top) : ""; })()`);
export async function boardStill(sel, act) {
  const id = stillBoard(sel);
  if (!id) return act();
  await pos(sel); // scrolled to first
  const before = await boardAt(id);
  await act();
  await tapDone();
  const after = await boardAt(id);
  const [u0, t0] = before.split(" "), [u1, t1] = after.split(" ");
  if (before && after && u0 === u1 && Math.abs(t1 - t0) > 1) moves.push(`${sel}: ${t1 - t0} px`); // 1 px: rounding at the phone's pixel ratio
}
export async function click(sel, opts) {
  await boardStill(sel, () => press(sel, opts));
}
// What is used now and then lies in the sheet "Mehr": opened first, as a
// player does.
export async function more(sel, opts) {
  if (await js(`document.getElementById("more-sheet").hidden`)) await click("#more");
  await click(sel, opts);
}
// Closes "Mehr" (it opens in place) with another tap on it.
export async function moreClose() {
  if (!(await js(`document.getElementById("more-sheet").hidden`))) await click("#more");
}
// A click on sel: with shift, or with the right button (right).
export async function press(sel, { shift = false, right = false } = {}) {
  const [x, y] = await pos(sel);
  const modifiers = shift ? 8 : 0, button = right ? "right" : "left";
  await send("Input.dispatchMouseEvent", { type: "mouseMoved", x, y, modifiers });
  await send("Input.dispatchMouseEvent", { type: "mousePressed", x, y, button, buttons: right ? 2 : 1, clickCount: 1, modifiers });
  await send("Input.dispatchMouseEvent", { type: "mouseReleased", x, y, button, buttons: 0, clickCount: 1, modifiers });
  await sleep(60);
}
export const tapDone = () => settle(`!document.querySelector("#board[data-pending]")`, 2000); // a click waiting for its double
export async function dblclick(sel) {
  await boardStill(sel, async () => {
    await press(sel);
    await press(sel);
  });
}
export async function drag(sels) {
  await boardStill(sels[0], () => pull(sels));
}
export async function pull(sels) {
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
export async function key(k, mods = 0) {
  const codes = { ArrowRight: 39, ArrowLeft: 37, Home: 36, End: 35, " ": 32, z: 90, y: 89, Escape: 27 };
  await send("Input.dispatchKeyEvent", { type: "keyDown", key: k, windowsVirtualKeyCode: codes[k], modifiers: mods });
  await send("Input.dispatchKeyEvent", { type: "keyUp", key: k, windowsVirtualKeyCode: codes[k], modifiers: mods });
  await sleep(60);
}
export const cell = (i) => `#board .cell[data-i="${i}"]`;
export const hasClass = (i, c) => `document.querySelector('${cell(i)}').classList.contains("${c}")`;
export const bannerHas = (t) => `document.getElementById("banner").textContent.includes(${JSON.stringify(t)})`;
export async function fresh() {
  await go("/einstellungen"); // any page of the origin that leads nowhere by itself
  await js(`localStorage.clear(); localStorage.setItem("starbattle.tutorial", "1")`); // no tutorial first
  await js(`document.cookie = "starbattle.solo=; path=/; max-age=0"`); // "/" gives a new puzzle
  await js(`document.cookie = "starbattle.party=; path=/; max-age=0"`); // "/party" is not the last party's
}
// Taps sel, a link or submit button, and waits for the page it leads to.
export async function follow(sel, how = click) {
  const loaded = once("Page.loadEventFired");
  await how(sel);
  await loaded;
  await sleep(250);
}
export async function api(p, body) {
  const r = await fetch(BASE + p, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  const v = await r.json();
  if (!r.ok) throw new Error(p + ": " + JSON.stringify(v));
  return v;
}
export const data = () => js(`JSON.parse(document.getElementById("data").textContent)`);

// ---------- observations ----------
export const observed = [];
export function record(o) {
  observed.push(JSON.stringify({ scenario: state.scenario, ...o }));
}

// What the page shows, normalized (see the top).
// Pages change through the site's tab bar only (decision 30): a link or form
// shown on a puzzle page that leads elsewhere is an error of the scenario,
// except the way to the next puzzle (#next-go, and the form "Neues Puzzle"
// it submits). obs collects them; uicheck.mjs records them after the scenario.
export const strayLinks = new Set();
const ALLOWED_WAYS = ["#next-go", "#newgame"];

export async function obs(label, { random = false } = {}) {
  await sleep(120);
  await tapDone();
  const stray = await js(`[...document.querySelectorAll("a[href], form[action]")]
    .filter((e) => e.getClientRects().length && !e.closest(".site-nav") && !${JSON.stringify(ALLOWED_WAYS)}.some((s) => e.matches(s)))
    .map((e) => (e.id ? "#" + e.id : e.tagName.toLowerCase()) + " " + JSON.stringify(e.textContent.trim().slice(0, 40)) + " -> " + (e.getAttribute("href") || e.getAttribute("action")))`);
  for (const s of stray) strayLinks.add(s.replace(/\/party\/[A-Z]{4}/g, "/party/CODE"));
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
      strip: [...document.querySelectorAll("#colors .color")].map((e) => e.classList.contains("has-star") ? "•" : "■").join(""), // a dot: the color has a star
      stripWant: (() => { // the strip as the board's stars make it
        if (!q("#colors .color")) return "";
        const d = JSON.parse(q("#data").textContent), out = new Array(d.n).fill("■");
        document.querySelectorAll("#board .cell").forEach((e, i) => { if (e.classList.contains("star")) out[d.regions[Math.floor(i / d.n)][i % d.n]] = "•"; });
        return out.join("");
      })(),
      tabs: !!q(".top .tabs") && getComputedStyle(q(".top .tabs")).display !== "none", // hidden with only one tab
      banner: txt("banner"), msg: txt("play-msg"), problems: txt("problems"), summary: txt("summary"),
      clock: txt("clock"), legend: txt("legend"), current: txt("current"),
      steps: document.querySelectorAll("#steps .row").length, active: t(q("#steps .row.active .num")),
      tutorial: txt("tutorial"), tour: t(q(".tour-tip")), tourTarget: q(".tour-target") && q(".tour-target").id,
      buttons: ["undo", "reset", "hint", "check", "hyp", "hyp-contra", "hyp-drop", "giveup", "more", "star-tool", "tab-play", "tab-watch", "first", "prev", "next", "last", "newgame", "next-go", "party-leave", "site-nav", "party-box", "play-options"].map(btn).join(" | "),
      pressed: [...document.querySelectorAll("[aria-pressed=true], [aria-expanded=true]")].map((e) => e.id).join(","),
      info: t(q("#info")) && (q("#info").hidden ? "[hidden]" : "shown"),
      selected: [...document.querySelectorAll("[aria-selected=true], [aria-current=page]")].map((e) => e.id).join(","),
      party: t(q("#party-panel")),
    };
  })()`);
  // On a phone the site's tab bar lies over the bottom of the screen: the
  // buttons of the shown tab must not end up under it.
  if (state.phone) o.bar = await js(`(() => {
    const b = document.querySelector(".panel:not([hidden]) .actionbar"), nav = document.getElementById("site-nav");
    if (!b) return "none";
    const r = b.getBoundingClientRect(), top = nav && !nav.hidden ? nav.getBoundingClientRect().top : innerHeight;
    return r.bottom <= top + 1 || r.top >= innerHeight ? "not covered" : "under the tab bar";
  })()`);
  // The banner and the tutorial's card come with a tap on the board (solved,
  // a hypothesis, a party's news, the next step), and with "solved" the way
  // to the next puzzle: they must be on screen then, not above it, where
  // only scrolling back up finds them. Only recorded while one is shown.
  if (state.phone) {
    const seen = await js(`(() => {
      const nav = document.getElementById("site-nav"), bottom = nav && !nav.hidden ? nav.getBoundingClientRect().top : innerHeight;
      return ["banner", "next-puzzle", "tutorial"].map((id) => document.getElementById(id)).filter((e) => e && !e.hidden && e.offsetParent).map((e) => {
        const r = e.getBoundingClientRect();
        return e.id + ": " + (r.top < -1 ? "above the screen" : r.bottom > bottom + 1 ? "below the screen" : "on screen");
      }).join(", ");
    })()`);
    if (seen) o.seen = seen;
  }
  // Words of the action bar cut off ("Aufgeb…"): only recorded when there are any.
  if (state.phone) {
    const cut = await js(`[...document.querySelectorAll(".actionbar button span")].filter((e) => e.offsetParent && e.scrollWidth > e.clientWidth + 1).map((e) => e.textContent).join(",")`);
    if (cut) o.cut = cut;
  }
  if (state.party) { // a random puzzle: which classes occur, not where, and no rule violations
    o.cells = [...new Set(o.cells.flatMap((c) => c.split("+")))].filter((c) => c !== "." && c !== "bad").sort();
    // The strip of colors: whether its dots are the colors with a star on
    // the board, not which (a wrong star may share a color with another).
    o.strip = o.strip === o.stripWant ? "dots on the colors with a star" : "differs from the board: " + o.strip;
    delete o.steps;
    delete o.problems;
  } else {
    o.cells = o.cells.join(" ");
  }
  delete o.stripWant;
  if (random) { // the random puzzle after the tutorial: only where it is
    for (const k of Object.keys(o)) if (k !== "path") delete o[k];
    o.path = o.path.replace(/seed=\d+/, "seed=RANDOM");
  }
  const norm = (s) => {
    if (typeof s !== "string") return s;
    s = s.replace(/\b\d+:\d\d(:\d\d)?\b/g, "T").replace(/\/party\/[A-Z]{4}/g, "/party/CODE").replace(/Party [A-Z]{4}/g, "Party CODE");
    s = s.replace(/^\/.*\/(p\.html)$/, "$1");
    if (state.party) s = s.replace(/\d+/g, "#");
    return s;
  };
  for (const k of Object.keys(o)) o[k] = norm(o[k]);
  record({ label, ...o });
}

// The server's view of Alice (the browser) in party p, as Bob's client gets
// it. With until, waits (at most a few seconds) until it holds. With board,
// her stars are compared with those on the page (without a hypothesis), for
// steps whose effect depends on the random puzzle (a hint).
export async function server(p, label, until = () => true, { board = false } = {}) {
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

// Runs act while Bob's connections come and go every 100 ms: each is a
// state for everybody, as when players change pages or their phones lose
// the connection. With latency (ms), on a slow connection too. For the
// moments that must hold up under a stream of states: tabs, page changes.
export async function underFire(p, act, { latency = 0 } = {}) {
  if (latency) await send("Network.emulateNetworkConditions", { offline: false, latency, downloadThroughput: -1, uploadThroughput: -1 });
  let on = true;
  const fire = (async () => {
    while (on) {
      const ac = new AbortController();
      fetch(`${BASE}/party/${p.code}/events?token=${p.token}`, { signal: ac.signal }).catch(() => {});
      await sleep(100);
      ac.abort();
      await sleep(100);
    }
  })();
  try {
    return await act();
  } finally {
    on = false;
    await fire;
    if (latency) await send("Network.emulateNetworkConditions", { offline: false, latency: 0, downloadThroughput: -1, uploadThroughput: -1 });
  }
}
// What tells the solving path from the play tab on a random puzzle (the
// steps differ): the play tab's banner and solution must not come back.
export async function watchShown(label, n) {
  record({
    label,
    banner: await js(`document.getElementById("banner").hidden ? "[hidden]" : document.getElementById("banner").textContent`),
    solutionOnBoard: await js(`document.querySelectorAll("#board .cell.star").length === ${n}`),
  });
}

export async function solveBy(sol, n, skip = new Set()) {
  for (let r = 0; r < n; r++) if (!skip.has(r)) await click(cell(r * n + sol[r]), { shift: true });
}

// A party in mode with Alice in the browser and Bob via the API, up to the
// start of round 1 (5×5 puzzles); options are more fields of the form.
export async function partyStart(mode, options = {}) {
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
export async function solutionOf(text) {
  const html = await (await fetch(BASE + "/play?p=" + encodeURIComponent(text.trim().split("\n").join("/")))).text();
  const m = html.match(/<script id="data" type="application\/json">(.*?)<\/script>/s);
  return JSON.parse(m[1]).solution;
}
