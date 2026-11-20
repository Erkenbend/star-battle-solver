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
- **Icons** drawn in Go (`internal/icon`), no outside tool: a 5×5 puzzle
  with its solution, in the colours the page gives a 5×5 board
  (`Logic.regionHSL`), the walls and the star symbol of the page. It is a
  real puzzle: `TestPuzzle` in `internal/icon` checks that the stars are its
  only solution. The first layout had two solutions, and that test found
  it. Renderings:
  - `icon-192.png` and `icon-512.png` (purpose `any`): rounded square,
    board at 80 %;
  - `icon-maskable-512.png`: square, full bleed, board at 56 %, so it stays
    in the safe zone (a circle of 80 %) whatever shape the launcher cuts;
  - `apple-touch-icon.png` (180 px): square, because iOS rounds it itself
    and fills transparent pixels with black;
  - `icon-32.png`, the favicon: board only, walls at least 2 px, no thin
    lines and stars without outline, drawn over the walls (with the large
    icon's proportions the stars disappeared at this size).

  They are golden files, as the sprite sheets in `symbols/` are:
  `go test ./internal/icon -update` draws them (4×4 samples per pixel) into
  `internal/web/assets/icons`, without `-update` the test fails when a file
  is not what the code draws (pixels may differ by 2 of 255, for floating
  point rounding on other CPUs). `web` embeds the files and serves them
  as they are. Both the icons and the manifest are cached for a day.
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
  stated size. `go test ./internal/icon`: the files are what the code
  draws, and the icon's puzzle has one solution.
- UI check, scenario `install`: Chrome's own verdict
  (`Page.getInstallabilityErrors`, which fetches the manifest and an icon)
  on the puzzle page, tutorial, library, settings, party and error page
  says installable, and Chrome reads name, start URL, display and icons
  from the manifest. The `html` page reports
  `not-from-secure-origin,no-manifest`, which shows the check can fail.
  The UI check runs on `127.0.0.1`, which counts as a secure origin.

## Measurements

First request of each icon (desktop, `curl` against `serve`): drawn by the
server on first request (the first version) and embedded files (now).

| URL | drawn | file | size |
|---|---|---|---|
| `/icon-32.png` | 3 ms | 1.2 ms | 0.9 kB |
| `/apple-touch-icon.png` | 20 ms | 0.7 ms | 4.4 kB |
| `/icon-192.png` | 32 ms | 0.9 ms | 5.9 kB |
| `/icon-maskable-512.png` | 101 ms | 0.7 ms | 12.3 kB |
| `/icon-512.png` | 222 ms | 0.9 ms | 18.5 kB |
| `/manifest.webmanifest` | – | – | 0.5 kB |

On a Pi the drawn ones would take several times as long. With the files the
binary got 0.3 MB smaller (12.33 → 12.00 MB): the server no longer links
the PNG encoder, the five files add 42 kB.

## Alternatives

- **Drawing on the first request** (the first version): no binary files in
  the repo, but up to 222 ms for the first request of an icon on a desktop,
  more on a Pi, and the encoder in the binary. The golden files cost a step
  after a change, which the test enforces.
- **PNG files drawn with an outside tool:** nothing keeps them in step with
  the page's palette and the puzzle check.
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
  The fix: "Solo" goes back to the last unfinished solo puzzle. Done in
  [decision 27](27-solo-resumes.md).
- **Not tried on a device yet:** installing on Android and iOS, the icon
  under the launcher's mask, and basic auth in an installed app on iOS
  (which may ask for the password again).
- The Pi at home (`http://raspberrypi.local:6369/`) is plain HTTP: browsers
  offer only a bookmark there, not an app. Installing needs the reverse
  proxy with TLS (see `deploy/README.md`).
