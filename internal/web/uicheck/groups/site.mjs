// UI check, group site: the site around the puzzles, the way back to the
// solo puzzle through the tab bar, the ends of the other pages and
// installing the site as an app.
import { BASE, HTMLFILE, cell, click, data, follow, fresh, go, js, obs, once, record, send, sleep, solveBy, tapDone } from "../lib.mjs";

// Those that run on a phone; the others run on a desktop screen.
export const PHONE_SCENARIOS = new Set(["resume", "pages"]);

export const scenarios = {
  // "/" (Solo, the start page, the installed app's start) leads back to
  // the unsolved solo puzzle, with its marks; a new one comes from "Neues
  // Puzzle", or from "/" once the last one is solved.
  async resume() {
    await fresh();
    await go("/?n=6&seed=3");
    await click(cell(0));
    await tapDone();
    await obs("started");
    await go("/einstellungen");
    await follow(`.site-nav a[href="/"]:not(.home)`);
    await obs("Solo from the settings");
    await go("/");
    await obs("start page");
    await follow("#newgame-go");
    const b = await js("location.pathname + location.search");
    record({ label: "Neues Puzzle", newPuzzle: b !== "/?n=6&seed=3" });
    await go("/bibliothek");
    await follow(`.site-nav a[href="/"]:not(.home)`);
    record({ label: "Solo from the library", samePuzzle: (await js("location.pathname + location.search")) === b });
    const d = await data();
    await solveBy(d.solution, d.n);
    await obs("solved", { random: true });
    await go("/einstellungen");
    await follow(`.site-nav a[href="/"]:not(.home)`);
    const c = await js("location.pathname + location.search");
    record({ label: "Solo after solving", newPuzzle: c !== b && c !== "/?n=6&seed=3" });
  },

  // The pages besides the puzzle, scrolled to their end: on a phone their
  // last part must end above the site's tab bar, not under it.
  async pages() {
    await fresh();
    for (const url of ["/bibliothek", "/bibliothek#schwer", "/einstellungen", "/party"]) {
      await go(url);
      await js(`window.scrollTo(0, document.documentElement.scrollHeight)`);
      await sleep(150);
      const end = await js(`(() => {
        const main = document.querySelector("main"), nav = document.querySelector(".site-nav");
        const last = [...main.querySelectorAll("*")].filter((e) => e.getClientRects().length).map((e) => e.getBoundingClientRect().bottom);
        const top = nav ? nav.getBoundingClientRect().top : innerHeight;
        return Math.max(...last) <= top + 1 ? "above the tab bar" : "under the tab bar";
      })()`);
      record({ label: url, end });
    }
    // The settings close with GitHub and the version. run.sh builds without
    // a build time, so the line shows the commit's time; whether the tree
    // had changes depends on where the check runs.
    await go("/einstellungen");
    record({ label: "settings footer", ...(await js(`(() => {
      const gh = document.getElementById("github"), a = document.querySelector("#build a");
      return {
        github: gh.href + " " + gh.target + " " + gh.getAttribute("aria-label"),
        build: document.getElementById("build").textContent.replace(/[0-9a-f]{7}/, "HASH").replace(" mit Änderungen", "").replace(/\\d/g, "9"),
        commitLink: a && /^https:\\/\\/github\\.com\\/Erkenbend\\/star-battle-solver\\/commit\\/[0-9a-f]{40}$/.test(a.href),
      };
    })()`)) });
  },

  // Installing the site as an app (decision 26): Chrome's own verdict on
  // each kind of page (it fetches the manifest and an icon for that), and
  // what it reads from the manifest. A page saved with `starbattle html`
  // has no manifest.
  async install() {
    await fresh();
    for (const url of ["/?n=6&seed=3", "/anleitung", "/bibliothek", "/einstellungen", "/party", "/?n=99"]) {
      await go(url);
      const { installabilityErrors } = await send("Page.getInstallabilityErrors");
      const { url: manifest, errors: manifestErrors } = await send("Page.getAppManifest");
      record({
        label: url, manifest: manifest.replace(BASE, ""),
        installable: installabilityErrors.length ? installabilityErrors.map((e) => e.errorId).join(",") : "yes",
        manifestErrors: manifestErrors.map((e) => e.message).join(" | "),
      });
    }
    const { data } = await send("Page.getAppManifest");
    const m = JSON.parse(data);
    record({ label: "manifest", name: m.name, start: m.start_url, display: m.display, icons: m.icons.map((i) => `${i.src} ${i.sizes} ${i.purpose}`).join(" | ") });
    await go("file://" + HTMLFILE);
    const { installabilityErrors } = await send("Page.getInstallabilityErrors");
    record({ label: "file page", manifest: (await send("Page.getAppManifest")).url || "none", installable: installabilityErrors.map((e) => e.errorId).join(",") });
  },
};
