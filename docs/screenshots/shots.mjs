// Takes the README screenshots via the Chrome DevTools protocol (node >= 22,
// no packages). Normally started by take.sh, which runs the server and a
// headless Chromium first.
//
// Usage: node shots.mjs <base-url> <devtools-port> <out-dir> <puzzle-11x11-file> [name...]
//
// Every picture is 1200×800 CSS pixels at device scale 1.5. Party pictures
// play Nina in the browser and Undine via the API; their party codes and
// puzzles are random.
import fs from "node:fs";

const [, , BASE, PORT, OUT, P11, ...ONLY] = process.argv;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const target = await (await fetch(`http://localhost:${PORT}/json/new?about:blank`, { method: "PUT" })).json();
const ws = new WebSocket(target.webSocketDebuggerUrl);
let seq = 0;
const pending = new Map();
const listeners = [];
ws.onmessage = (m) => {
  const d = JSON.parse(m.data);
  if (d.id && pending.has(d.id)) {
    const { res, rej } = pending.get(d.id);
    pending.delete(d.id);
    d.error ? rej(new Error(JSON.stringify(d.error))) : res(d.result);
  } else if (d.method) {
    for (const l of [...listeners]) l(d);
  }
};
await new Promise((r) => (ws.onopen = r));
const send = (method, params = {}) => new Promise((res, rej) => { pending.set(++seq, { res, rej }); ws.send(JSON.stringify({ id: seq, method, params })); });
const once = (method) => new Promise((r) => { const l = (d) => { if (d.method === method) { listeners.splice(listeners.indexOf(l), 1); r(d.params); } }; listeners.push(l); });

await send("Page.enable");
await send("Runtime.enable");
await send("Emulation.setDeviceMetricsOverride", { width: 1200, height: 800, deviceScaleFactor: 1.5, mobile: false });

async function js(expr) {
  const r = await send("Runtime.evaluate", { expression: expr, awaitPromise: true, returnByValue: true });
  if (r.exceptionDetails) throw new Error(expr + ": " + JSON.stringify(r.exceptionDetails));
  return r.result.value;
}
async function go(url) {
  const loaded = once("Page.loadEventFired");
  await send("Page.navigate", { url: BASE + url });
  await loaded;
  await sleep(300);
}
async function waitFor(expr, ms = 15000) {
  const t0 = Date.now();
  while (Date.now() - t0 < ms) {
    if (await js(`!!(${expr})`)) return;
    await sleep(50);
  }
  throw new Error("timeout: " + expr);
}
async function click(sel, { shift = false } = {}) {
  const r = await js(`(() => { const e = document.querySelector(${JSON.stringify(sel)}); if (!e) return null; const b = e.getBoundingClientRect(); return [b.x + b.width / 2, b.y + b.height / 2]; })()`);
  if (!r) throw new Error("no element " + sel);
  const [x, y] = r;
  const modifiers = shift ? 8 : 0;
  await send("Input.dispatchMouseEvent", { type: "mouseMoved", x, y, modifiers });
  await send("Input.dispatchMouseEvent", { type: "mousePressed", x, y, button: "left", buttons: 1, clickCount: 1, modifiers });
  await send("Input.dispatchMouseEvent", { type: "mouseReleased", x, y, button: "left", buttons: 0, clickCount: 1, modifiers });
  await sleep(80);
}
const cell = (i) => `#board .cell[data-i="${i}"]`;
async function shot(name) {
  await sleep(250);
  const r = await send("Page.captureScreenshot", { format: "png" });
  fs.writeFileSync(`${OUT}/${name}.png`, Buffer.from(r.data, "base64"));
  console.log("wrote", name);
}
async function fresh() {
  await go("/party"); // any page of the origin
  await js("localStorage.clear()");
}
async function api(path, body) {
  const r = await fetch(BASE + path, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  const v = await r.json();
  if (!r.ok) throw new Error(path + ": " + JSON.stringify(v));
  return v;
}
async function newParty(mode) {
  const r = await fetch(BASE + "/party", { method: "POST", body: new URLSearchParams({ n: "7", mode }), redirect: "manual" });
  return r.headers.get("location").split("/").pop();
}
const clockIs = (t) => `document.getElementById("clock").textContent.includes(${JSON.stringify(t)})`;

// A party with Nina in the browser and Undine via the API, up to the start of round 1.
async function partyStart(mode) {
  await fresh();
  const code = await newParty(mode);
  const undine = await api(`/party/${code}/join`, { name: "Undine" });
  await js(`localStorage.setItem("starbattle.name", "Nina")`);
  await go("/party/" + code);
  await click("#join button[type=submit]");
  await waitFor(`[...document.querySelectorAll("#party-panel button")].some((b) => b.textContent === "Bereit") && !document.querySelector("#party-panel .actions .muted")`);
  await api(`/party/${code}/ready`, { token: undine.token, ready: true });
  const loaded = once("Page.loadEventFired");
  await js(`[...document.querySelectorAll("#party-panel button")].find((b) => b.textContent === "Bereit").click()`);
  await loaded;
  await waitFor(`!document.body.classList.contains("party-wait") && document.querySelector("#board .cell")`);
  const d = await js(`JSON.parse(document.getElementById("data").textContent)`);
  return { code, token: undine.token, sol: d.solution, n: d.n };
}

const shots = {
  async spielen() {
    await fresh();
    await go("/?n=8&seed=42");
    for (const i of [0, 1, 8, 9, 62, 63]) await click(cell(i));
    await click(cell(16), { shift: true });
    await click("#hint");
    await waitFor(clockIs("0:04"));
    await shot("spielen");
  },
  async loesungsweg() {
    await fresh();
    await go("/?n=8&seed=42");
    await click("#tab-watch");
    await click("#steps > li:nth-child(7) > .row");
    await js(`document.getElementById("steps").scrollTop = 0`);
    await shot("loesungsweg");
  },
  async annahme() {
    await fresh();
    const rows = fs.readFileSync(P11, "utf8").split("\n").filter((l) => l && !l.startsWith("#"));
    await go("/play?p=" + encodeURIComponent(rows.join("/")));
    await click("#tab-watch");
    // Rows far down the list are out of view: click them by script.
    // Each toggle rebuilds the list, so look the rows up afresh.
    for (const k of [28, 29]) {
      const t = `document.querySelector("#steps > li:nth-child(${k + 1}) .toggle-chain")`;
      if (await js(`${t}.textContent === "Gedankengang zeigen"`)) await js(`${t}.click()`);
      await sleep(100);
    }
    await js(`document.querySelector("#steps > li:nth-child(29) .chain > li:nth-child(2) > .row").click()`);
    await sleep(200);
    // Step 27 at the top of the list.
    await js(`(() => { const ol = document.getElementById("steps"); ol.scrollTop += ol.children[27].getBoundingClientRect().top - ol.getBoundingClientRect().top; })()`);
    await shot("annahme");
  },
  async "party-anlegen"() {
    await fresh();
    await go("/party");
    await click('input[name=mode][value=coop]');
    await shot("party-anlegen");
  },
  async "party-runde"() {
    const p = await partyStart("versus");
    await api(`/party/${p.code}/progress`, { token: p.token, round: 1, count: 3, hints: 0 });
    for (const r of [1, 4]) await click(cell(r * p.n + p.sol[r]), { shift: true });
    await waitFor(clockIs("0:05"));
    await shot("party-runde");
  },
  async "party-gemeinsam"() {
    const p = await partyStart("coop");
    const n = p.n, last = n - 1;
    const crosses = [...Array(n).keys()].filter((c) => c !== p.sol[last]).slice(0, 3).map((c) => last * n + c);
    const ops = [0, 1, 2, 3].map((r, k) => [k + 1, r * n + p.sol[r], 0, 2]);
    ops.push([5, crosses[0], 0, 1], [6, crosses[1], 0, 1]);
    await api(`/party/${p.code}/mark`, { token: p.token, round: 1, ops });
    await click(cell(4 * n + p.sol[4]), { shift: true });
    await click(cell(crosses[2]));
    await waitFor(clockIs("0:04"));
    await shot("party-gemeinsam");
  },
  async bibliothek() {
    await fresh();
    await go("/bibliothek");
    const entries = await js(`JSON.parse(document.getElementById("data").textContent).entries`);
    const times = { 1: 151, 2: 263, 3: 418, 4: 372, 6: 545, 7: 690 };
    const solved = {};
    for (const e of entries) if (times[e.nr]) solved[e.id] = { time: times[e.nr] * 1000 + 400, at: "2026-11-08" };
    await js(`localStorage.setItem("starbattle:library:solved", ${JSON.stringify(JSON.stringify(solved))})`);
    await go("/bibliothek");
    await shot("bibliothek");
  },
  async "bibliothek-geloest"() {
    await go("/bibliothek");
    const e = (await js(`JSON.parse(document.getElementById("data").textContent).entries`)).find((e) => e.nr === 9);
    const d = await (await fetch(`${BASE}/bibliothek/weg?id=${encodeURIComponent(e.id)}`)).json();
    const marks = new Array(d.n * d.n).fill(0);
    d.solution.forEach((c, r) => { marks[r * d.n + c] = 2; });
    const v = { marks, auto: true, time: (18 * 60 + 43) * 1000 + 300, hints: 0, done: true, paid: [] };
    await js(`localStorage.setItem(${JSON.stringify("starbattle:library:" + e.id)}, ${JSON.stringify(JSON.stringify(v))})`);
    await go("/bibliothek?id=" + encodeURIComponent(e.id));
    await shot("bibliothek-geloest");
  },
};

for (const [name, f] of Object.entries(shots)) {
  if (ONLY.length && !ONLY.includes(name)) continue;
  await f();
}
process.exit(0);
