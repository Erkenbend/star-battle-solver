# 26. The site can be installed as a web app

- **Status:** done
- **Date:** 2026-11-20

Takes up the open point of [decision 25](25-mobile-first-page.md): with
variant B's tab bar at the bottom, the site already works like an app on a
phone. Now it can also be installed as one ("Zum Startbildschirm
hinzufügen", "App installieren"), with its own icon, opening in a window of
its own without the browser's address bar.

## Decision

- **Manifest** at `/manifest.webmanifest` (`internal/web/install.go`):
  name "Star Battle", `start_url` and `scope` `/`, `display: standalone`,
  the light `--bg` as theme and background colour, German description.
- **Icons** drawn in Go with `image/png`, so there are no binary files in
  the repo and no tool to make them: a 5×5 puzzle with its solution, in the
  colours the page gives a 5×5 board (`Logic.regionHSL`), the walls and the
  star symbol of the page. It is a real puzzle: `TestIconPuzzle` checks that
  the stars are its only solution. The first layout had two solutions, and
  that test found it. Renderings:
  - `icon-192.png` and `icon-512.png` (purpose `any`): rounded square,
    board at 80 %;
  - `icon-maskable-512.png`: square, full bleed, board at 56 %, so it stays
    in the safe zone (a circle of 80 %) whatever shape the launcher cuts;
  - `apple-touch-icon.png` (180 px): square, because iOS rounds it itself
    and fills transparent pixels with black;
  - `icon-32.png`, the favicon: board only, walls at least 2 px, no thin
    lines and stars without outline, drawn over the walls (with the large
    icon's proportions the stars disappeared at this size).

  Each is rendered on its first request (4×4 samples per pixel) and kept;
  both the icons and the manifest are cached for a day.
- **Head tags** on every page the server sends (puzzle, tutorial, library,
  settings, party, error page) through the placeholder `<!--{{APP}}-->`:
  the manifest link, the favicon, the apple-touch-icon and `theme-color`
  for light and dark. A page saved with `starbattle html` gets none of
  them, because there is no server to fetch them from and it should stay
  one file.
- The manifest link has **`crossorigin="use-credentials"`**. Browsers fetch
  the manifest without credentials by default. Behind basic auth (the
  public instance) that request gets a 401, and the page cannot be
  installed.
- **No service worker, so no offline play.** Chrome no longer requires one
  for installing. New puzzles, the library and parties come from the
  server anyway, and parties need a live connection. An installed app
  without a network shows the browser's error page.

## Checks

- `go test ./internal/web`: every kind of served page links the manifest,
  the `html` page does not. The manifest is `standalone` with an `any` and
  a `maskable` icon, and every icon it or the head names is a PNG of the
  stated size. The icon's puzzle has one solution.
- UI check, scenario `install`: Chrome's own verdict
  (`Page.getInstallabilityErrors`, which fetches the manifest and an icon)
  on the puzzle page, tutorial, library, settings, party and error page
  says installable, and Chrome reads name, start URL, display and icons
  from the manifest. The `html` page reports
  `not-from-secure-origin,no-manifest`, which shows the check can fail.
  The UI check runs on `127.0.0.1`, which counts as a secure origin.

## Measurements

Rendering on first request (desktop, `curl` against `serve`), size, later
requests below 1 ms:

| URL | first request | size |
|---|---|---|
| `/icon-32.png` | 3 ms | 0.9 kB |
| `/apple-touch-icon.png` | 20 ms | 4.4 kB |
| `/icon-192.png` | 32 ms | 5.9 kB |
| `/icon-maskable-512.png` | 101 ms | 12.3 kB |
| `/icon-512.png` | 222 ms | 18.5 kB |
| `/manifest.webmanifest` | – | 0.5 kB |

Expect several times that once on a Pi. The 512 px icons are requested only
when the app is installed.

## Alternatives

- **PNG files in the repo**, drawn once with a tool or rewritten by a test
  with `-update` (like the sprite sheets): adds binary files and a step to
  remember after a change. Drawing in Go keeps the icon next to the
  palette it copies.
- **An SVG icon only:** iOS needs a PNG for the apple-touch-icon, and
  Android's support of SVG manifest icons is uncertain. The favicon could
  be SVG, but one drawing for all sizes is simpler.
- **A service worker caching the pages:** offline play is not planned. A
  cached puzzle page without the server would show a puzzle, but no new
  puzzle, library, party or settings, and a stale cache brings its own
  bugs (old scripts after an update).
- **Manifest only on the puzzle page:** the tutorial, library or a party
  link is just as likely the page someone installs from.

## Open

- **Back navigation in standalone mode.** On iOS an installed app has no
  back button. The tab bar reaches every page, but "Solo" always starts a
  new puzzle (`/`), so going from a puzzle to the settings and back loses
  your place (the progress stays in local storage, under the puzzle's URL).
  The fix: "Solo" goes back to the last unfinished solo puzzle.
- **Not tried on a device yet:** installing on Android and iOS, the icon
  under the launcher's mask, and basic auth in an installed app on iOS
  (which may ask for the password again).
- The Pi at home (`http://raspberrypi.local:6369/`) is plain HTTP: browsers
  offer only a bookmark there, not an app. Installing needs the reverse
  proxy with TLS (see `deploy/README.md`).
