// Behaviour check of the pages (UI check): plays every mode of the puzzle
// page and the designer in a headless Chromium via the DevTools protocol
// (node >= 22, no packages), records what the page shows after each action
// and compares that with expected/<group>.jsonl. Normally started by run.sh,
// which builds starbattle and starts the servers and Chromium first.
//
// Usage: node uicheck.mjs <base-url> <designer-url> <devtools-host:port> <html-file> <out-dir> [-update] [group...]
//
// The scenarios come in groups, one module each in groups/ and one file of
// observations each in expected/: a page or the way between pages (GROUPS
// below). Without groups, all run, in that order. A group's scenarios
// always run together and in their order: one may build on the one before.
//
// Every line of an expected file is one observation: the board's cell
// classes, banner, messages, buttons, clock, steps of the solving path and
// so on, or the server's view of a party ("server: …"). Values that differ
// between runs of the same build are normalized when recorded: times, party
// codes, the random puzzles of parties (their boards as the classes that
// occur, no rule violations, steps or problems), the random puzzle after the
// tutorial and the designer's completions. A failed step, a JavaScript error
// or a page load the page aborted itself is an observation too, so it shows
// as a difference.
//
// With -update, the observations replace the expected files of the groups
// run. Do that after an intended change of the page, and check its diff.
import fs from "node:fs";
import path from "node:path";
import { connect, disconnect, send, state, record, observed, errors, aborted, moves, DESKTOP, PHONE } from "./lib.mjs";

// Page by page, roughly in the order a visitor meets them.
const GROUPS = ["tutorial", "solo", "library", "party", "site", "designer"];

const args = process.argv.slice(2);
const UPDATE = args.includes("-update");
const [base, designer, devtools, htmlFile, outDir, ...only] = args.filter((a) => a !== "-update");
const unknown = only.filter((g) => !GROUPS.includes(g));
if (unknown.length) {
  console.error(`unknown group ${unknown.join(", ")}; the groups are ${GROUPS.join(", ")}`);
  process.exit(2);
}
const ran = GROUPS.filter((g) => !only.length || only.includes(g));
const here = path.dirname(new URL(import.meta.url).pathname);
const expectedFile = (g) => path.join(here, "expected", g + ".jsonl");

await connect({ base, designer, devtools, htmlFile }).catch((e) => {
  console.error(e.message);
  process.exit(2);
});

const scenarioGroup = new Map();
for (const g of ran) {
  const { scenarios, PHONE_SCENARIOS } = await import(`./groups/${g}.mjs`);
  for (const [name, f] of Object.entries(scenarios)) {
    scenarioGroup.set(name, g);
    state.scenario = name;
    state.party = false;
    state.phone = PHONE_SCENARIOS.has(name);
    await send("Emulation.setDeviceMetricsOverride", state.phone ? PHONE : DESKTOP);
    try {
      await f();
    } catch (e) {
      record({ label: "FAILED", error: String(e.message || e).slice(0, 300) });
    }
    if (errors.length) record({ label: "JS ERRORS", errors: errors.splice(0) });
    if (aborted.length) record({ label: "ABORTED PAGE LOADS", count: aborted.splice(0).length });
    if (moves.length) record({ label: "BOARD MOVED", moves: moves.splice(0) });
  }
}
disconnect();

// ---------- compare with the expected files ----------
const got = new Map(ran.map((g) => [g, []]));
for (const l of observed) got.get(scenarioGroup.get(JSON.parse(l).scenario)).push(l);
fs.mkdirSync(outDir, { recursive: true });
for (const [g, lines] of got) fs.writeFileSync(path.join(outDir, g + ".jsonl"), lines.join("\n") + "\n");

if (UPDATE) {
  fs.mkdirSync(path.join(here, "expected"), { recursive: true });
  for (const [g, lines] of got) fs.writeFileSync(expectedFile(g), lines.join("\n") + "\n");
  console.log(`${observed.length} observations of ${ran.join(", ")} written to expected/`);
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
let diffs = 0;
for (const [g, lines] of got) {
  const f = expectedFile(g);
  const wantBy = byID(fs.existsSync(f) ? fs.readFileSync(f, "utf8").split("\n").filter(Boolean) : []);
  const gotBy = byID(lines);
  for (const k of wantBy.keys()) if (!gotBy.has(k)) { diffs++; console.log(`${g}: missing: ${k}`); }
  for (const [k, o] of gotBy) {
    const w = wantBy.get(k);
    if (!w) { diffs++; console.log(`${g}: unexpected: ${k}\n  ${JSON.stringify(o).slice(0, 400)}`); continue; }
    for (const field of new Set([...Object.keys(w), ...Object.keys(o)])) {
      const a = JSON.stringify(w[field]), b = JSON.stringify(o[field]);
      if (a !== b) {
        diffs++;
        console.log(`${g}: ${k} / ${field}:\n  want ${String(a).slice(0, 400)}\n  got  ${String(b).slice(0, 400)}`);
      }
    }
  }
}
console.log(`${observed.length} observations of ${ran.join(", ")}, ${diffs} differences`);
process.exit(diffs ? 1 : 0);
