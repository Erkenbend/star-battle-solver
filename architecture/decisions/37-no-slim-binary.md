# 37. No separate serve-only binary for the Pi

- **Status:** rejected
- **Date:** 2026-11-24 (measured at `144fbc2`, no code change)

The idea: a second, smaller binary ("starbattle-slim") for the Raspberry
Pi that can only run `serve`, without `generate`, `show`, `solve`, `html`,
`design`, `hardsearch` and `stress`. The Pi only ever runs `serve`
(`deploy/starbattle.service`), so the developer tools seemed to be
dead weight there.

## Measurements

A serve-only variant was built without touching the repository: `go build
-overlay` replaced `cmd/starbattle/main.go` by one that keeps only `main`
(accepting `serve` alone), `cmdServe`, `solverFlags`, `serveUntilDone` and
`newServer`. Both were built with the flags of `deploy/deploy.sh`
(`CGO_ENABLED=0 -trimpath -ldflags="-s -w"`).

| | full | serve-only | difference |
|---|---|---|---|
| linux/arm64 binary | 8.52 MB | 7.80 MB | −0.72 MB (−8.5 %) |
| same, `gzip -9` (what `deploy.sh` copies over the network is the plain binary; this is for the CI archives) | 3.38 MB | 3.05 MB | −0.32 MB |
| RSS after start and two pages (amd64, `serve -no-pregen`) | 16.5 MB | 15.3 MB | −1.2 MB, of it 0.66 MB file-backed |

Why so little: most of the binary is what `serve` needs itself. The Go
runtime, `net/http` (with http2, and TLS and x509 that it pulls in),
`encoding/json`, the embedded library, motifs, symbols and page assets
(about 1.1 MB), and `web` already depends on `generator`, `solver`,
`bruteforce` and `party`. Leaving out the dev commands drops only
`designer` (and its two assets), `hardsearch`, `stress` (and with it the
client half of `net/http`), `render` and the command code. Memory barely
changes because code that never runs is never paged in, and the
file-backed part can be reclaimed by the kernel anyway. Even a Pi Zero
(512 MB RAM) and any SD card do not notice either difference.

## Why rejected

The gain is under 1 MB. The cost is a second build configuration: CI must
vet and build it, the tests of `cmd/starbattle` cover the two variants
differently, `deploy/README.md`, `docs/development.md` and
[architecture/README.md](../README.md) need to describe it, and it bends
principle 6 ("one binary"). It also takes away `starbattle solve` and
`html` on the Pi itself, handy for a quick check of the installed build.

## How it would be built

If it is ever wanted (e.g. the binary grows a much bigger dev-only part):

- **A build tag, not a second `cmd/`.** Split `cmd/starbattle/main.go`
  into `serve.go` (always built: `main`, a map from command name to
  `func([]string) error` holding `serve`, plus `cmdServe`,
  `solverFlags`, `serveUntilDone`, `newServer`) and `dev.go` with
  `//go:build !slim`, whose `init` registers the other commands.
  `usage()` lists the registered commands.
- `deploy/deploy.sh` builds with `-tags slim`, maybe behind an option;
  the service file stays as it is (`ExecStart=… serve`).
- CI: `go vet -tags slim ./cmd/starbattle` and a slim build next to the
  release builds.
- Rejected alternative: a separate `cmd/starbattle-serve` would need
  `solverFlags` and the server helpers moved into an internal package
  (a main package cannot be imported), plus a second binary name in
  `deploy/`, CI and the docs.
