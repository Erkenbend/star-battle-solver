"use strict";

// Party mode ("Zusammen spielen"), shared by the party page (party.html) and
// the puzzle page of a round (app.js uses it when the data has `party`).
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

  function el(tag, cls, text) {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }

  function fmtTime(ms) {
    const s = Math.floor(ms / 1000), h = Math.floor(s / 3600), m = Math.floor(s / 60) % 60;
    const ss = String(s % 60).padStart(2, "0");
    return h ? `${h}:${String(m).padStart(2, "0")}:${ss}` : `${m}:${ss}`;
  }
  const hintsText = (k) => (k === 1 ? "1 Hinweis" : `${k} Hinweise`);

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

  // Copies text to the clipboard, with the old execCommand route for plain
  // http (a home server is not a secure context).
  async function copyText(text) {
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
        return true;
      }
    } catch { /* fall through */ }
    const ta = el("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.append(ta);
    ta.select();
    let ok = false;
    try { ok = document.execCommand("copy"); } catch { ok = false; }
    ta.remove();
    if (!ok) window.prompt("Kopieren nicht möglich, bitte selbst kopieren:", text);
    return ok;
  }

  const STATUS = { solved: "gelöst", gaveup: "aufgegeben", playing: "spielt noch", absent: "abwesend", left: "gegangen", done: "fertig" };

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
    let snap = null, at = 0, es = null, errText = "", busy = false, isGone = false;

    function connect() {
      if (es) es.close();
      es = new EventSource(`/party/${code}/events?token=${encodeURIComponent(token(code))}`);
      es.onmessage = (ev) => {
        snap = JSON.parse(ev.data);
        at = Date.now();
        errText = "";
        draw();
        if (hooks.onSnap) hooks.onSnap(snap, at);
      };
      const gone = () => {
        es.close();
        snap = null;
        isGone = true;
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
      const t = el("table", "ranking");
      const head = el("tr");
      for (const h of ["#", "Name", "", "Hinweise", "Zeit"]) head.append(el("th", "", h));
      t.append(el("thead"), el("tbody"));
      t.tHead.append(head);
      for (const e of snap.ranking) {
        const tr = el("tr", e.status + (e.id === snap.you ? " you" : ""));
        const what = e.status === "playing" ? `★ ${e.stars}/${snap.n}` : STATUS[e.status];
        tr.append(
          el("td", "rank", e.rank ? e.rank + "." : ""),
          el("td", "name", e.name),
          el("td", "status", what),
          el("td", "num", e.status === "solved" || e.status === "gaveup" || e.hints ? String(e.hints) : ""),
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
          snap = r.snapshot;
          at = Date.now();
        } catch (e) {
          errText = e.message;
        }
        busy = false;
        draw();
      };
      const a = el("div", "actions");
      a.append(b);
      if (snap.preparing) a.append(el("span", "muted", "Das nächste Puzzle wird noch vorbereitet …"));
      return a;
    }

    // The ways on once the party is gone.
    function wayOut() {
      const a = el("div", "actions");
      const home = el("a", "button primary", "Zur Startseite");
      home.href = "/";
      const fresh = el("a", "button", "Neue Party");
      fresh.href = "/party";
      a.append(home, fresh);
      return a;
    }

    function draw() {
      const nodes = [];
      if (errText) nodes.push(el("p", "party-error", errText));
      if (isGone) nodes.push(wayOut());
      if (snap) {
        nodes.push(el("p", "muted party-mode", snap.mode === "coop"
          ? "Miteinander: alle auf demselben Brett, ohne Hinweise."
          : snap.clock
            ? `Gegen die Uhr (${clockRules(snap.clock)}): so viele Puzzles wie möglich, ohne Hinweise (Prüfen geht). Wertung nach gelösten Puzzles, dann richtigen minus falschen Sternen im letzten.`
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
  function bootPage() {
    const d = JSON.parse(document.getElementById("data").textContent);
    const $ = (id) => document.getElementById(id);
    if (d.error) {
      $("party-error").textContent = d.error + " ";
      const home = el("a", "next", "Zur Startseite");
      home.href = "/";
      $("party-error").append(home);
      $("party-error").hidden = false;
    }
    if (!d.code) {
      $("start").hidden = false;
      let chosen = [6, 7, 8, 9];
      try {
        const v = JSON.parse(get("starbattle.partySizes") || "null");
        if (Array.isArray(v) && v.length) chosen = v;
      } catch { /* default */ }
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
      const clockOpts = () => { $("clock-options").hidden = !$("create").querySelector('input[name="mode"][value="clock"]').checked; };
      for (const r of $("create").querySelectorAll('input[name="mode"]')) r.addEventListener("change", clockOpts);
      clockOpts(); // the browser may restore the choice
      $("create").addEventListener("submit", (ev) => {
        const sizes = [...$("sizes").querySelectorAll("input:checked")].map((b) => +b.value);
        if (!sizes.length) {
          ev.preventDefault();
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
    $("copy-link").onclick = async () => {
      if (await copyText(location.origin + "/party/" + code)) {
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
        if (me && me.playing) location.href = roundPath(snap);
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
    $("leave").onclick = async () => {
      if (!confirm("Party wirklich verlassen?")) return;
      try { await post(code, "leave", {}); } catch { /* gone anyway */ }
      setToken(code, "");
      location.href = "/party";
    };
  }

  if (typeof document !== "undefined" && document.getElementById("party-panel") && document.getElementById("lobby")) {
    bootPage();
  }

  return { token, setToken, name, setName, post, live, fmtTime, hintsText, copyText, el, roundPath };
})();
