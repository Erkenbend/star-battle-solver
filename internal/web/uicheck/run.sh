#!/usr/bin/env bash
# UI check: builds starbattle, starts the server and a headless Chromium, and
# lets uicheck.mjs play every mode of the puzzle page and compare what it
# shows with expected.jsonl. Fails (exit 1) on a difference and prints it.
#
#   internal/web/uicheck/run.sh                  all scenarios
#   internal/web/uicheck/run.sh coop race        only some
#   internal/web/uicheck/run.sh -update          write expected.jsonl anew
#
# Environment:
#   CHROME       Chromium/Chrome (default: chromium, google-chrome or Playwright's)
#   UICHECK_OUT  file for the observations of this run (default: a temp file,
#                removed afterwards)
# Needs go, node >= 22 and curl. Takes about a minute.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../.." && pwd)
port=${UICHECK_PORT:-6397}
devtools=${UICHECK_DEVTOOLS:-9337}

chrome=${CHROME:-}
if [[ -z $chrome ]]; then
  chrome=$(command -v chromium || command -v chromium-browser || command -v google-chrome || command -v google-chrome-stable || true)
fi
if [[ -z $chrome ]]; then
  chrome=$(ls -d "$HOME"/.cache/ms-playwright/chromium-*/chrome-linux/chrome 2>/dev/null | tail -n 1 || true)
fi
[[ -x $chrome ]] || { echo "no Chromium found, please set CHROME" >&2; exit 1; }

tmp=$(mktemp -d)
pids=()
cleanup() {
  for p in "${pids[@]}"; do kill "$p" 2>/dev/null || true; done
  wait 2>/dev/null || true
  rm -rf "$tmp"
}
trap cleanup EXIT

(cd "$root" && go build -o "$tmp/starbattle" ./cmd/starbattle)
# The page of a puzzle as a file, as `starbattle html` writes it.
"$tmp/starbattle" generate -n 6 -seed 3 | "$tmp/starbattle" html -o "$tmp/p.html"
"$tmp/starbattle" serve -addr "localhost:$port" -no-pregen > "$tmp/serve.log" 2>&1 &
pids+=($!)
"$chrome" --headless --no-sandbox --disable-gpu --no-first-run --user-data-dir="$tmp/profile" \
  --remote-debugging-port=$devtools about:blank > "$tmp/chrome.log" 2>&1 &
pids+=($!)

for _ in $(seq 100); do
  curl -sf "localhost:$port/party" > /dev/null && curl -sf "localhost:$devtools/json/version" > /dev/null && break
  sleep 0.1
done

out=${UICHECK_OUT:-$tmp/observed.jsonl}
status=0
node "$here/uicheck.mjs" "http://localhost:$port" "$devtools" "$tmp/p.html" "$out" "$@" || status=$?
if [[ $status -ne 0 ]]; then
  echo "--- server log" >&2
  tail -n 20 "$tmp/serve.log" >&2
fi
exit $status
