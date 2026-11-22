# 34. gzip for pages and JSON

- **Status:** done
- **Date:** 2026-11-22 (`12df1d7`, `d0e1626`)

Suggestion 7 of [refactorings.md](../refactorings.md). `serve` sent every
page uncompressed: about 220 KB for a 15×15 hard puzzle of the library,
170 KB for any other puzzle page, most of it the same style and scripts
(principle 6: one self-contained page). With `Cache-Control: no-store`
every page change loads all of it again, and players on the home network
reach the binary directly, without a proxy that could compress.

## What was done

- **`web.Compress`** (`internal/web/gzip.go`), a wrapper around the
  handler: it decides at a handler's first write, from the content type,
  and compresses `text/html` and `application/json` when the request
  accepts gzip (`q=0` and an explicit `gzip;q=0` next to `*` respected).
  Not the event stream (it must reach the player per event), the PNG
  icons, the manifest, SVGs or plain-text errors. It sets
  `Vary: Accept-Encoding`, leaves HEAD, 204 and 304 alone, keeps its
  writers in a `sync.Pool`, and lets `http.ResponseController` reach the
  connection (`Unwrap`, `FlushError`), which the event stream's write
  deadline needs.
- **`serve -gzip`**, on by default: `serve` wraps `web.Handler` in
  `Compress` unless started with `-gzip=false`, which `deploy/README.md`
  shows via `STARBATTLE_OPTS`. For a weak CPU in a fast network (a Pi
  Zero on Wi-Fi). A proxy in front does not need it: Caddy's `encode`
  passes an answer that already has a `Content-Encoding` through.
- **Level `gzip.BestSpeed`**, see the measurements.
- **Tests:** `TestGzip` checks that answers decode to exactly the
  uncompressed ones with the same headers, that pages above 10 KB shrink
  to at most 40 %, and that the event stream stays uncompressed and its
  first event arrives (Go's client asks for gzip and decodes it unseen,
  so `TestPartyEvents` would not notice a compressed stream).
  `BenchmarkGzipLevels` compares the levels. `starbattle stress` got the
  scenario `pages` and `-serve-args` (for `-gzip=false`).

## Measurements

Ryzen 7 5800X3D. `starbattle stress`: `serve -no-pregen` pinned to 4 cores
(`taskset -c 0-3`, `GOMAXPROCS=4`, like a Raspberry Pi 4 but each core
several times faster), the load on the other cores, one run each. "CPU"
is the server's user + system time.

**Size** (curl against `serve`, gzip at `BestSpeed`):

| answer | raw | gzip | |
|---|---|---|---|
| hard 15×15 page of the library (`15-1`, with its trace) | 219.9 KB | 71.6 KB | 33 % |
| its trace, `/bibliothek/weg` (JSON) | 50.1 KB | 9.1 KB | 18 % |
| tutorial (like any puzzle page without a long trace) | 172.9 KB | 63.2 KB | 37 % |
| library overview | 171.7 KB | 52.7 KB | 31 % |
| settings | 84.9 KB | 31.1 KB | 37 % |
| error page | 44.1 KB | 14.1 KB | 32 % |
| `/symbole.json` | 2.9 KB | 1.1 KB | 38 % |

The suggestion's 208 → 56 KB was estimated with the default level on a
smaller page; pages have grown since, and `BestSpeed` gives a third, not a
quarter.

**Levels** (`go test ./internal/web -run '^$' -bench GzipLevels`, one
core, time to compress one page; "serve" is building it without gzip):

| page | serve | `BestSpeed` | default | `BestCompression` |
|---|---|---|---|---|
| hard 15×15 | 0.67 ms | 0.97 ms, 71.6 KB | 1.81 ms, 61.0 KB | 8.92 ms, 58.6 KB |
| generated 8×8 (cached) | 0.24 ms | 0.85 ms, 62.9 KB | 1.59 ms, 53.9 KB | 7.51 ms, 51.9 KB |
| tutorial | 0.23 ms | 0.85 ms, 63.2 KB | 1.57 ms, 54.1 KB | 7.55 ms, 52.1 KB |

The default level saves another 9–11 KB per page (about 15 %) for twice
the CPU; the best one 2 KB more for nine times. Compressing costs more
than building a page: a page whose solve is done (cache of decision 33,
pool, library, tutorial) is cheap to build, so gzip is the larger part of
serving it.

**Pages** (`starbattle stress pages`): 16 players load the same kind of
page at once, 50 times (800 pages), without and with gzip; the hard page
is solved before, the 8×8 are new seeds (generated and solved per
request), the tutorial is solved once.

| page | server CPU per page | latency median / p95 | sent per page |
|---|---|---|---|
| hard 15×15, without | 1.23 ms | 4.9 / 7.1 ms | 219.9 KB |
| hard 15×15, gzip | 2.50 ms | 8.6 / 12.3 ms | 71.6 KB |
| new 8×8, without | 1.57 ms | 5.6 / 8.2 ms | 172.9 KB |
| new 8×8, gzip | 2.70 ms | 9.3 / 13.2 ms | 63.0 KB |
| tutorial, without | 0.46 ms | 1.9 / 2.9 ms | 172.9 KB |
| tutorial, gzip | 1.73 ms | 5.8 / 8.6 ms | 63.2 KB |

About 1.2 ms of CPU more per page. The latencies are over loopback, with
no limit on bandwidth, so gzip can only lose there; 16 players at once
still get every page within 14 ms on 4 cores.

**On a slow connection** the time to load a page is its size divided by
the bandwidth (an in-process test with a throttled listener, removed in
`d0e1626`, measured the same to within 1 % up to 1 MB/s, 5 % at 5 MB/s):

| bandwidth | hard 15×15 without / gzip | new 8×8 without / gzip |
|---|---|---|
| 50 KB/s (about DevTools' "Slow 3G") | 4.40 / 1.43 s | 3.46 / 1.26 s |
| 200 KB/s ("Fast 3G") | 1.10 / 0.36 s | 0.86 / 0.32 s |
| 1 MB/s (weak Wi-Fi) | 220 / 72 ms | 173 / 63 ms |
| 5 MB/s | 44 / 14 ms | 35 / 13 ms |

Below about 120 MB/s, where sending 150 KB fewer takes longer than the
1.2 ms of CPU, gzip is faster for the player; a phone in a party is far
below that. On a Pi, with cores several times slower, gzip costs a few
milliseconds per page, still well below what a slow connection saves.

**The scenarios of decision 33**, with gzip (the default) and with
`-serve-args=-gzip=false`:

| | without gzip | gzip |
|---|---|---|
| 200 × `/anleitung`, CPU | 0.13 s | 0.34 s |
| 10 reloads of a hard `/play` | 1.28 s, CPU 3.22 s | 1.25 s, CPU 3.15 s |
| 8 players open a new link at once | 3.14 s, CPU 11.1 s | 3.00 s, CPU 10.7 s |
| abandon: CPU in the 8 s after the visitors left | 0.04 s | 0.10 s |
| flood: latency median / slowest | 3.15 / 4.87 s | 2.85 / 4.71 s |
| crawl: the 193 pages of the crawler | 0.23 s | 0.64 s |
| crawl: all library solves done | 61.2 s, CPU 233 s | 58.8 s, CPU 223 s |
| crawl: new 10×10 during the crawl, slowest | 0.04 s | 0.07 s |
| crawl: peak RSS | 178 MB | 180 MB |

Where solving dominates, the difference is the noise of one run (gzip is
even ahead in some rows). It shows where only pages are served: the
tutorial (1 ms per page, as above) and the crawler's pages, which share
the four cores with the solves the crawl starts; each still came within
0.01 s.

## Alternatives

- **Compress in `writeHTML` and `writeJSON`**, as the suggestion had it:
  both lack the request, so about 30 call sites would have to pass it,
  and `/bibliothek/weg` and `/symbole.json` write JSON themselves. The
  wrapper covers every route and the handlers stay as they are.
- **A build tag instead of the option** (`-tags nogzip`): the choice
  belongs to where the server runs, which is known when deploying, not
  when building; with a tag, switching needs a new build and the tests see
  only one variant.
- **The default level or `BestCompression`**: see the table of levels;
  `BestSpeed` gets most of the saving for the least CPU, which matters
  most on the Pi.
- **Brotli or zstd**: smaller, but not in the standard library (no
  third-party dependencies, principle 2).
- **Serving scripts and style as separate, cacheable files**: rejected in
  the second review ([refactorings.md](../refactorings.md), "Considered
  and not recommended"); with gzip the pages are a third of the size.

Open: not measured on a Raspberry Pi, neither the CPU nor over Wi-Fi
(`starbattle stress -url http://raspberrypi.local:6369 -pid … pages`
from the desktop measures the real network). Keeping the cached pages of
decision 33 compressed would take the 1.2 ms off every cached page, but
decision 33 keeps the solver's result, not the HTML, on purpose; worth it
only if the Pi's numbers call for it.
