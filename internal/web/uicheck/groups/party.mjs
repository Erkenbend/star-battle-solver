// UI check, group party: parties against each other (also with lives),
// together and against the clock, Alice in the browser and Bob via the API.
import { api, bannerHas, cell, click, copied, data, dblclick, follow, go, hasClass, js, key, more, obs, once, partyStart, record, reload, server, settle, sleep, solveBy, state, tapDone, underFire, visited, waitFor, watchShown } from "../lib.mjs";

// Those that run on a phone; the others run on a desktop screen.
export const PHONE_SCENARIOS = new Set(["versus", "lives", "coop", "race"]);

export const scenarios = {
  // A party round against each other: progress, hints, hypothesis (not
  // reported), solving, the end of the round; then giving up.
  async versus() {
    state.party = true;
    const p = await partyStart("versus", {}, { inLobby: () => copied("lobby link copied", "#copy-link") });
    await obs("started");
    // The way to the lobby is the tab bar's "Zusammen spielen", not a link in the page.
    record({ label: "Zusammen spielen in a round", href: (await js(`document.getElementById("nav-party").getAttribute("href")`)).replace(/[A-Z]{4}$/, "CODE") });
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
    await more("#reset");
    await obs("reset in party");
    await solveBy(p.sol, p.n);
    await waitFor(`!document.getElementById("tab-watch").disabled`);
    await obs("solved");
    await server(p, "solved", (me) => me.status === "solved");
    // The solving path while Bob plays on: his progress must not bring back
    // the board and banner of the play tab.
    await click("#tab-watch");
    await underFire(p, async () => {
      await click("#next");
      await api(`/party/${p.code}/progress`, { token: p.token, round: 1, count: 2, hints: 0 });
      await settle(`document.getElementById("party-panel").textContent.includes("★ 2/")`);
      await sleep(800);
    });
    await watchShown("watch while Bob plays", p.n);
    await click("#first"); // what the next observations see of the hidden tab
    await click("#tab-play");
    await api(`/party/${p.code}/giveup`, { token: p.token, round: 1, hints: 0 });
    await settle(bannerHas("vorbei") + " || document.querySelector('#party-panel').textContent.includes('Ergebnis')");
    await obs("round over");
    await reload();
    await obs("reloaded after round");
    // The next round, from this page: straight to its puzzle, not through
    // the lobby. Bob's connections coming and going while the page loads
    // must not start the load over.
    await waitFor(`[...document.querySelectorAll("#party-panel button")].some((b) => b.textContent === "Bereit") && !document.querySelector("#party-panel .actions .muted")`);
    await api(`/party/${p.code}/ready`, { token: p.token, ready: true });
    visited.length = 0;
    await underFire(p, async () => {
      const next = once("Page.loadEventFired");
      await js(`[...document.querySelectorAll("#party-panel button")].find((b) => b.textContent === "Bereit").click()`);
      await next.catch(() => {});
    }, { latency: 600 });
    record({ label: "next round: pages loaded", pages: visited.map((u) => u.replace(/\/party\/[A-Z]{4}/, "/party/CODE")) });

    const q = await partyStart("versus");
    await js(`window.confirm = () => true`);
    await click(cell(q.sol[0]), { shift: true });
    await click("#giveup");
    await settle(bannerHas("Aufgegeben"));
    await obs("gave up");
    await server(q, "gave up", (me) => me.status === "gaveup");
    // "Zusammen spielen" (/party) leads back to the party: its lobby, which
    // sends a player of the running round on to it. Leaving it under "Mehr"
    // frees /party for a new one (decision 31).
    const here = async () => (await js("location.pathname")).replace(/\/party\/[A-Z]{4}/, "/party/CODE");
    await go("/party");
    await waitFor(`location.pathname.includes("/round/")`);
    await sleep(250);
    record({ label: "/party while in a party", path: await here() });
    await js(`window.confirm = () => true`);
    await follow("#party-leave", more);
    record({ label: "left under Mehr", path: await here(), start: await js(`!document.getElementById("start").hidden`) });
    await go("/party");
    record({ label: "/party after leaving", path: await here() });
  },

  // Against each other with 2 lives: a star where the solution has none
  // costs one, once per cell, even if it breaks no rule; a cross on a star of
  // the solution and a star in a hypothesis cost none; the second wrong star
  // ends the round for Alice.
  async lives() {
    state.party = true;
    const p = await partyStart("versus", { lives: "2" });
    await obs("started");
    const c0 = p.sol[0];
    const wrongIn = (r) => r * p.n + [...Array(p.n).keys()].find((c) => c !== p.sol[r] && c !== c0);
    await click(cell(c0), { shift: true }); // right
    await click(cell(wrongIn(2)), { shift: true }); // two rows further: no rule against it; its banner must not move the board
    await obs("wrong star");
    await server(p, "one error", (me) => me.errors === 1);
    await click("#undo");
    await obs("undone");
    await key("y", 2); // redo has no button
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
    // That star, not the rule violations it may add (a color of the random
    // puzzle with two stars), so not a count.
    record({ label: "out of lives: wrong star in red", bad: await js(hasClass(wrongIn(3), "bad")) });
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
    state.party = true;
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
    await key("y", 2); // redo has no button
    await settle(hasClass(p.sol[0], "star"));
    await obs("redo own");
    await server(p, "after redo", (me, s) => s.coop.board.includes("2"));
    await click("#undo");
    await key("y", 2); // redo has no button
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
    await settle(`document.getElementById("giveup").getAttribute("aria-pressed") === "true"`);
    await obs("voted");
    await api(`/party/${p.code}/vote`, { token: p.token, round: 1, giveUp: true });
    await settle(bannerHas("aufgegeben"));
    await obs("given up together");
    // Reloaded, the board comes from the server (a coop round keeps nothing
    // on the device).
    await reload();
    await waitFor(`document.querySelector("#board .cell")`);
    await obs("coop reloaded");

    const q = await partyStart("coop");
    await solveBy(q.sol, q.n);
    await settle(bannerHas("Gemeinsam gelöst"));
    await obs("solved together");
    await click("#tab-watch");
    await underFire(q, async () => {
      await click("#next");
      await sleep(1000);
    });
    await watchShown("watch after solving together", q.n);
  },

  // Against the clock: progress, solving (on to puzzle 2), reload, skipping.
  async race() {
    state.party = true;
    const p = await partyStart("clock");
    await obs("started");
    await click(cell(p.sol[0]), { shift: true });
    await click(cell(p.n + p.sol[1]), { shift: true });
    await server(p, "progress", (me) => me.stars === 2);
    // On to the next puzzle on a slow connection while states keep coming:
    // one load (aborted ones are an error of the scenario).
    await underFire(p, async () => {
      const loaded = once("Page.loadEventFired");
      await solveBy(p.sol, p.n, new Set([0, 1]));
      await loaded;
    }, { latency: 600 });
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
