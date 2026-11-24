"use strict";

// Party mode ("Zusammen spielen"), shared by the party page (party.html) and
// the puzzle page of a round (round.js uses it when the data has `party`).
// The server sends the party's state as server-sent events; actions are
// JSON POSTs. Each player's token lives in localStorage, per party code.
const Party = (() => {
  const get = (k) => {
    try { return localStorage.getItem(k) || ""; } catch { return ""; }
  };
  const put = (k, v) => {
    try { v ? localStorage.setItem(k, v) : localStorage.removeItem(k); } catch { /* private mode etc. */ }
  };
  const token = (code) => get("starbattle.party:" + code);
  const setToken = (code, t) => put("starbattle.party:" + code, t);
  const name = () => get("starbattle.name");
  const setName = (v) => put("starbattle.name", v);

  // /party (the tab "Zusammen spielen") leads back to the party this browser
  // takes part in: the server reads "CODE.TOKEN" from this cookie
  // (PartyCookie in party_server.go) and checks that the token is still a
  // player's. A state that shows us as a player puts the party there; one
  // without us, the party gone or leaving it takes it out, but only if it
  // is this party: looking at another party's lobby keeps ours.
  const PARTY_COOKIE = "starbattle.party";
  function rememberParty(code, member) {
    try {
      const now = (document.cookie.split("; ").find((c) => c.startsWith(PARTY_COOKIE + "=")) || "").slice(PARTY_COOKIE.length + 1);
      const want = member && token(code) ? code + "." + token(code) : "";
      if (want && now !== want) document.cookie = `${PARTY_COOKIE}=${want}; path=/; max-age=604800; samesite=lax`; // 7 days
      else if (!want && now.split(".")[0] === code) document.cookie = PARTY_COOKIE + "=; path=/; max-age=0; samesite=lax";
    } catch { /* cookies blocked */ }
  }

  const { el, copyText } = Dom;
  const { fmtTime } = Logic;

  // post sends an action with the player's token; it resolves to the reply
  // ({snapshot, ...}) or rejects with the server's message.
  async function post(code, action, body) {
    let res, reply;
    try {
      res = await fetch(`/party/${code}/${action}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token: token(code), ...body }),
      });
      reply = await res.json();
    } catch {
      throw Object.assign(new Error("Keine Verbindung zum Server."), { status: 0 });
    }
    if (!res.ok) throw Object.assign(new Error(reply.error || res.statusText), { status: res.status });
    return reply;
  }

  const STATUS = { solved: "gelöst", gaveup: "aufgegeben", out: "KO", playing: "spielt noch", absent: "abwesend", left: "gegangen", done: "fertig" };

  // Goes to another page of the party, once. Every state that arrives while
  // the page loads would ask for it again, and setting location anew aborts
  // the load and starts it over: with a slow connection and a busy party the
  // page would never arrive. replace: without an entry in the history.
  let leaving = null, left = false;
  function goTo(path, replace = false) {
    if (leaving === path || left) return;
    leaving = path;
    if (replace) location.replace(path);
    else location.href = path;
  }
  // Back from the history cache: whatever was under way is over.
  if (typeof window !== "undefined") window.addEventListener("pageshow", () => { leaving = null; left = false; });

  // Leaving the party for good, from its lobby or under "Mehr" in a round
  // (the lobby sends a player of the running round back to it): then /party
  // is free to create or join another one. No other page change after it.
  async function leave(code) {
    if (!confirm("Party wirklich verlassen?")) return;
    try { await post(code, "leave", {}); } catch { /* gone anyway */ }
    setToken(code, "");
    rememberParty(code, false);
    goTo("/party");
    left = true;
  }

  // The page of the running round for the viewer: against the clock, that of
  // the puzzle they are at (or the last one prepared).
  function roundPath(snap) {
    let path = `/party/${snap.code}/round/${snap.round}`;
    if (snap.clock) {
      const me = snap.ranking.find((e) => e.id === snap.you);
      path += "/" + Math.max(1, Math.min(me ? me.puzzle : 1, snap.clock.sizes.length));
    }
    return path;
  }

  // The lives of a party against each other (lives: per puzzle, 0 unlimited),
  // in a few words: the errors allowed.
  function livesRule(lives) {
    return lives === 1 ? "kein Fehler erlaubt" : `höchstens ${lives - 1} Fehler erlaubt`;
  }

  // The rules of a party against the clock, in a few words.
  function clockRules(c) {
    const skips = c.skips < 0 ? "beliebig oft überspringen" : c.skips === 0 ? "kein Überspringen" : `${c.skips}× überspringen`;
    return `${c.minutes} Minuten, ${skips}`;
  }

  // live keeps the party's state in sync and draws it into container:
  // status line, ranking of the round, players and the ready button.
  // hooks: onSnap(snap, receivedAt), onGone(). It returns {snap(), reconnect(),
  // draw(), error(text)}.
  function live(code, container, hooks = {}) {
    let snap = null, at = 0, es = null, errText = "", busy = false;

    // A state from the event stream or the reply to an action.
    function take(s) {
      snap = s;
      at = Date.now();
      rememberParty(code, !!s.you);
      errText = "";
      draw();
      if (hooks.onSnap) hooks.onSnap(snap, at);
    }

    function connect() {
      if (es) es.close();
      es = new EventSource(`/party/${code}/events?token=${encodeURIComponent(token(code))}`);
      es.onmessage = (ev) => take(JSON.parse(ev.data));
      const gone = () => {
        es.close();
        snap = null;
        rememberParty(code, false);
        errText = "Diese Party gibt es nicht mehr.";
        draw();
        if (hooks.onGone) hooks.onGone();
      };
      es.addEventListener("gone", gone);
      // A refused stream (404) ends in CLOSED; lost connections retry by themselves.
      es.onerror = () => { if (es.readyState === EventSource.CLOSED) gone(); };
    }

    // Time since the round's start as of now, from the last state.
    const elapsed = () => (snap ? snap.elapsedMs + Date.now() - at : 0);
    // Against the clock: time left as of now.
    const left = () => Math.max(0, snap.clock.minutes * 60000 - Math.max(0, elapsed()));
    const startIn = () => (snap ? snap.startInMs - (Date.now() - at) : 0);

    function statusLine() {
      const p = el("p", "party-status");
      if (snap.phase === "lobby") {
        p.textContent = "Es geht los, sobald alle bereit sind.";
      } else if (snap.clock) {
        const s = startIn();
        p.textContent = snap.phase !== "playing"
          ? `Runde ${snap.round}: Die Zeit ist um. Die nächste beginnt, sobald alle bereit sind.`
          : s > 0
            ? `Runde ${snap.round} beginnt in ${Math.ceil(s / 1000)} …`
            : `Runde ${snap.round} läuft · noch ${fmtTime(Math.ceil(left() / 1000) * 1000)}`;
      } else if (snap.phase === "playing") {
        const s = startIn();
        p.textContent = s > 0
          ? `Runde ${snap.round} (${snap.n}×${snap.n}) beginnt in ${Math.ceil(s / 1000)} …`
          : `Runde ${snap.round} (${snap.n}×${snap.n}) läuft · ${fmtTime(Math.max(0, elapsed()))}`;
        if (s <= 0 && snap.coop && snap.coop.votes.length) p.textContent += ` · ${snap.coop.votes.length} wollen aufgeben`;
      } else if (snap.coop) {
        const t = snap.ranking.length ? fmtTime(snap.ranking[0].timeMs) : "";
        p.textContent = snap.coop.solved
          ? `Runde ${snap.round} (${snap.n}×${snap.n}): gemeinsam gelöst in ${t}! Die nächste beginnt, sobald alle bereit sind.`
          : `Runde ${snap.round} (${snap.n}×${snap.n}): gemeinsam aufgegeben nach ${t}. Die nächste beginnt, sobald alle bereit sind.`;
      } else {
        p.textContent = `Runde ${snap.round} (${snap.n}×${snap.n}) ist vorbei. Die nächste beginnt, sobald alle bereit sind.`;
      }
      return p;
    }

    // Coop: one result for all; the table shows who took part and their
    // changes to the shared board.
    function coopTable() {
      const t = el("table", "ranking coop");
      const head = el("tr");
      for (const h of ["Name", "", "Beiträge"]) head.append(el("th", "", h));
      t.append(el("thead"), el("tbody"));
      t.tHead.append(head);
      const votes = snap.coop ? snap.coop.votes : [];
      for (const e of snap.ranking) {
        const tr = el("tr", e.status + (e.id === snap.you ? " you" : ""));
        const what = e.status === "playing" && votes.includes(e.id) ? "will aufgeben" : e.status === "playing" ? "spielt mit" : STATUS[e.status];
        tr.append(el("td", "name", e.name), el("td", "status", what), el("td", "num", String(e.moves || 0)));
        t.tBodies[0].append(tr);
      }
      return t;
    }

    // Against the clock: ranked by puzzles solved, then by right minus wrong
    // stars on the current puzzle.
    function clockTable() {
      const t = el("table", "ranking clock");
      const head = el("tr");
      const skips = snap.clock.skips !== 0;
      for (const h of ["#", "Name", "", "Gelöst", "★", ...(skips ? ["Übersp."] : [])]) head.append(el("th", "", h));
      head.cells[4].title = "Richtige minus falsche Sterne im aktuellen Puzzle";
      if (skips) head.cells[5].title = "Übersprungene Puzzles";
      t.append(el("thead"), el("tbody"));
      t.tHead.append(head);
      for (const e of snap.ranking) {
        const tr = el("tr", e.status + (e.id === snap.you ? " you" : ""));
        let what = STATUS[e.status];
        if (e.status === "playing") {
          const k = snap.clock.sizes[e.puzzle - 1];
          what = k ? `Puzzle ${e.puzzle} (${k}×${k})` : `wartet auf Puzzle ${e.puzzle}`;
        }
        const stars = e.stars ? (e.stars > 0 ? "+" : "−") + Math.abs(e.stars) : "";
        tr.append(
          el("td", "rank", e.rank + "."),
          el("td", "name", e.name),
          el("td", "status", what),
          el("td", "num", String(e.solved || 0)),
          el("td", "num", stars),
        );
        if (skips) tr.append(el("td", "num", e.skips ? String(e.skips) : ""));
        t.tBodies[0].append(tr);
      }
      return t;
    }

    function rankingTable() {
      if (snap.mode === "coop") return coopTable();
      if (snap.clock) return clockTable();
      const lives = snap.lives > 0;
      const t = el("table", "ranking" + (lives ? " lives" : ""));
      const head = el("tr");
      // The columns in the order they rank by.
      for (const h of ["#", "Name", "", ...(lives ? ["Fehler"] : []), "Hinweise", "Zeit"]) head.append(el("th", "", h));
      if (lives) head.cells[3].title = "Sterne in diesem Puzzle, die nicht zur Lösung passen";
      t.append(el("thead"), el("tbody"));
      t.tHead.append(head);
      for (const e of snap.ranking) {
        const tr = el("tr", e.status + (e.id === snap.you ? " you" : ""));
        const what = e.status === "playing" ? `★ ${e.stars}/${snap.n}` : STATUS[e.status];
        tr.append(
          el("td", "rank", e.rank ? e.rank + "." : ""),
          el("td", "name", e.name),
          el("td", "status", what),
          ...(lives ? [el("td", "num", String(e.errors || 0))] : []),
          el("td", "num", e.status === "solved" || e.status === "gaveup" || e.status === "out" || e.hints ? String(e.hints) : ""),
          el("td", "num", e.status === "solved" ? fmtTime(e.timeMs) : ""),
        );
        t.tBodies[0].append(tr);
      }
      return t;
    }

    function playerList() {
      const ul = el("ul", "players");
      for (const pl of snap.players) {
        const li = el("li", pl.id === snap.you ? "you" : "");
        li.append(el("span", "name", pl.name));
        let state;
        if (!pl.present) state = el("span", "muted", "abwesend");
        else if (snap.phase === "playing") state = el("span", "muted", pl.playing ? "spielt" : "wartet auf die nächste Runde");
        else state = pl.ready ? el("span", "ready", "bereit ✓") : el("span", "muted", "noch nicht bereit");
        li.append(state);
        ul.append(li);
      }
      return ul;
    }

    function readyButton() {
      const me = snap.players.find((p) => p.id === snap.you);
      if (!me || snap.phase === "playing") return null;
      const b = el("button", me.ready ? "" : "primary", me.ready ? "Doch nicht bereit" : "Bereit");
      b.disabled = busy;
      b.onclick = async () => {
        busy = true;
        draw();
        try {
          const r = await post(code, "ready", { ready: !me.ready });
          busy = false;
          take(r.snapshot); // the last one ready starts the round: on at once, not after the event
        } catch (e) {
          busy = false;
          errText = e.message;
          draw();
        }
      };
      const a = el("div", "actions");
      a.append(b);
      if (snap.preparing) a.append(el("span", "muted", "Das nächste Puzzle wird noch vorbereitet …"));
      return a;
    }

    function draw() {
      const nodes = [];
      if (errText) nodes.push(el("p", "party-error", errText));
      if (snap) {
        nodes.push(el("p", "muted party-mode", snap.mode === "coop"
          ? "Miteinander: alle auf demselben Brett, ohne Hinweise."
          : snap.clock
            ? `Gegen die Uhr (${clockRules(snap.clock)}): so viele Puzzles wie möglich, ohne Hinweise. Wertung nach gelösten Puzzles, dann richtigen minus falschen Sternen im letzten.`
            : snap.lives > 0
              ? `Gegeneinander: Wertung nach wenigsten Fehlern, dann Hinweisen, dann schnellster Zeit. ${snap.lives} Leben pro Puzzle, also ${livesRule(snap.lives)}: Jeder Stern, der nicht zur Lösung passt, kostet eins, wer keins mehr hat, ist KO.`
              : "Gegeneinander: Wertung nach wenigsten Hinweisen, dann schnellster Zeit."));
        nodes.push(statusLine());
        if (snap.prepErr) nodes.push(el("p", "party-error", snap.prepErr));
        if (snap.round) nodes.push(rankingTable());
        if (snap.phase !== "playing" || snap.players.some((p) => !p.playing)) {
          nodes.push(el("h3", "", "Dabei"), playerList());
        }
        const b = readyButton();
        if (b) nodes.push(b);
      }
      container.replaceChildren(...nodes);
    }

    // The countdown and the round's clock tick between states.
    setInterval(() => { if (snap && snap.phase === "playing") draw(); }, 500);
    connect();
    return {
      snap: () => snap,
      reconnect: connect,
      draw,
      error(text) { errText = text; draw(); },
    };
  }

  // ---------- party page: create, join, lobby ----------
  // The party's link as a QR code (qr.js, only on this page) in svg, with
  // a light margin of four modules. The short link /P/{code} (the server
  // sends it on to /party/{code}) in upper case, which takes fewer modules
  // (QR's alphanumeric mode); scheme and host don't mind. Hidden if the link
  // is too long for qr.js (see docs/development.md).
  function drawQR(svg, code) {
    let m;
    try {
      m = QR.encode((location.origin + "/P/" + code).toUpperCase());
    } catch {
      svg.setAttribute("hidden", ""); // an svg has no .hidden
      return;
    }
    const w = m.length + 8;
    svg.setAttribute("viewBox", `-4 -4 ${w} ${w}`);
    svg.innerHTML = `<rect x="-4" y="-4" width="${w}" height="${w}" fill="#fff"/><path fill="#000" d="${QR.path(m)}"/>`;
  }

  function bootPage() {
    const d = JSON.parse(document.getElementById("data").textContent);
    const $ = (id) => document.getElementById(id);
    // ⓘ beside the title: what the page is about, folded away.
    $("info-btn").onclick = () => {
      const on = $("info").hidden;
      $("info").hidden = !on;
      $("info-btn").setAttribute("aria-expanded", on);
    };
    if (d.error) {
      $("party-error").textContent = d.error; // the ways on are in the tab bar
      $("party-error").hidden = false;
    }
    if (!d.code) {
      $("start").hidden = false;
      let chosen = [6, 7, 8, 9];
      try {
        const v = JSON.parse(get("starbattle.partySizes") || "null");
        if (Array.isArray(v) && v.length) chosen = v;
      } catch { /* default */ }
      const form = $("create");
      for (let n = d.minN; n <= d.maxN; n++) {
        const lab = el("label", "size");
        const box = el("input");
        box.type = "checkbox";
        box.name = "n";
        box.value = n;
        box.checked = chosen.includes(n);
        lab.append(box, ` ${n}×${n}`);
        $("sizes").append(lab);
      }
      const chosenSizes = () => [...$("sizes").querySelectorAll("input:checked")].map((b) => +b.value);
      // The summary of the folded sizes; the mode's description and options.
      const MODE = {
        versus: "Jede Person auf ihrem eigenen Brett; Wertung: wenigste Hinweise, dann schnellste Zeit.",
        coop: "Alle markieren auf demselben Brett, gemeinsame Zeit, ohne Hinweise.",
        clock: "So viele Puzzles wie möglich in der Zeit, für alle dieselben in derselben Reihenfolge, ohne Hinweise; Wertung: gelöste Puzzles, dann richtige minus falsche Sterne im letzten.",
      };
      const summarize = () => {
        const sizes = chosenSizes();
        const run = sizes.length > 2 && sizes.every((k, i) => i === 0 || k === sizes[i - 1] + 1);
        $("sizes-sum").textContent = !sizes.length ? "keine gewählt"
          : run ? `${sizes[0]}×${sizes[0]} bis ${sizes.at(-1)}×${sizes.at(-1)}` : sizes.map((k) => `${k}×${k}`).join(", ");
        const mode = form.elements.mode.value;
        $("versus-options").hidden = mode !== "versus";
        $("clock-options").hidden = mode !== "clock";
        $("mode-desc").textContent = MODE[mode];
        $("lives-help").hidden = !(+form.elements.lives.value > 0); // only once lives are limited
      };
      form.addEventListener("change", summarize);
      summarize(); // the browser may restore the choices
      form.addEventListener("submit", (ev) => {
        const sizes = chosenSizes();
        if (!sizes.length) {
          ev.preventDefault();
          $("opt-sizes").open = true;
          $("party-error").textContent = "Bitte mindestens eine Größe wählen.";
          $("party-error").hidden = false;
          return;
        }
        put("starbattle.partySizes", JSON.stringify(sizes));
      });
      $("goto").addEventListener("submit", (ev) => {
        ev.preventDefault();
        const code = $("goto-code").value.trim().toUpperCase();
        if (code) location.href = "/party/" + encodeURIComponent(code);
      });
      return;
    }

    const code = d.code;
    $("lobby").hidden = false;
    $("party-title").textContent = "Party " + code;
    $("code").textContent = code;
    const link = location.origin + "/party/" + code;
    drawQR($("qr"), code);
    $("copy-link").onclick = async () => {
      if (await copyText(link)) {
        $("copy-link").textContent = "Kopiert ✓";
        setTimeout(() => { $("copy-link").textContent = "Link kopieren"; }, 2000);
      }
    };
    $("join-name").value = name();

    const view = live(code, $("party-panel"), {
      onSnap(snap) {
        const me = snap.players.find((p) => p.id === snap.you);
        $("join").hidden = !!me;
        $("leave").hidden = !me;
        // Taking part in the running round: off to its puzzle.
        if (me && me.playing) goTo(roundPath(snap));
      },
      onGone() {
        $("join").hidden = true;
        $("leave").hidden = true;
      },
    });

    $("join").addEventListener("submit", async (ev) => {
      ev.preventDefault();
      const nm = $("join-name").value.trim();
      try {
        const r = await post(code, "join", { name: nm });
        setToken(code, r.token);
        setName(nm);
        view.reconnect();
      } catch (e) {
        view.error(e.message);
      }
    });
    $("leave").onclick = () => leave(code);
  }

  if (typeof document !== "undefined" && document.getElementById("party-panel") && document.getElementById("lobby")) {
    bootPage();
  }

  return { token, setToken, name, setName, post, live, goTo, leave, roundPath, livesRule };
})();
