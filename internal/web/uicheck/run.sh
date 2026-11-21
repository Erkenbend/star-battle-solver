#!/usr/bin/env bash
# UI check: builds starbattle, starts the server, the designer and a
# headless Chromium, and lets uicheck.mjs play every mode of the puzzle page
# and the designer and compare what they show with expected/. Fails (exit 1)
# on a difference and prints it.
#
#   internal/web/uicheck/run.sh                  all groups
#   internal/web/uicheck/run.sh solo designer    only these groups
#   internal/web/uicheck/run.sh -update [group…] write their expected files anew
#
# The groups (see GROUPS in uicheck.mjs, one module each in groups/):
# tutorial, solo, library, party, site, designer.
#
# Environment:
#   CHROME       Chromium/Chrome (default: chromium, google-chrome or Playwright's)
#   UICHECK_OUT  directory for the observations of this run, one file per
#                group (default: a temp directory, removed afterwards)
# Needs go, node >= 22 and curl. All groups take about two minutes.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../.." && pwd)
port=${UICHECK_PORT:-6397}
designport=${UICHECK_DESIGN_PORT:-6398}
devtools=${UICHECK_DEVTOOLS:-9337}

# Google Chrome first: a chromium from snap cannot use paths in /tmp.
chrome=${CHROME:-}
if [[ -z $chrome ]]; then
  chrome=$(command -v google-chrome || command -v google-chrome-stable || command -v chromium || command -v chromium-browser || true)
fi
if [[ -z $chrome ]]; then
  chrome=$(ls -d "$HOME"/.cache/ms-playwright/chromium-*/chrome-linux/chrome 2>/dev/null | tail -n 1 || true)
fi
[[ -x $chrome ]] || { echo "no Chromium found, please set CHROME" >&2; exit 1; }
echo "browser: $chrome ($("$chrome" --version 2>/dev/null || echo "version unknown"))"

tmp=$(mktemp -d)
pids=()
# Keeps the script's exit code. The browser's helper processes may still
# write to its profile for a moment after it is stopped, so removing the
# temp directory is retried, and a leftover never fails the run.
cleanup() {
  local code=$?
  for p in "${pids[@]}"; do kill "$p" 2>/dev/null || true; done
  wait 2>/dev/null || true
  for _ in $(seq 20); do
    rm -rf "$tmp" 2>/dev/null && break
    sleep 0.25
  done
  exit "$code"
}
trap cleanup EXIT

(cd "$root" && go build -o "$tmp/starbattle" ./cmd/starbattle)
# The page of a puzzle as a file, as `starbattle html` writes it.
"$tmp/starbattle" generate -n 6 -seed 3 | "$tmp/starbattle" html -o "$tmp/p.html"
# 127.0.0.1, not localhost: that may resolve to ::1 first, where neither
# Chrome's debugging port nor the server listens (GitHub's runners).
host=127.0.0.1
"$tmp/starbattle" serve -addr "$host:$port" -no-pregen > "$tmp/serve.log" 2>&1 &
pids+=($!)
# The designer saves to designs/ in the temp directory: the page names the
# directory as given, the same on every machine.
(cd "$tmp" && exec ./starbattle design -addr "$host:$designport" -dir designs > design.log 2>&1) &
pids+=($!)
# --disable-dev-shm-usage: CI machines and containers often have a small
# /dev/shm, on which the browser crashes. No throttling of timers in tabs
# that look hidden: the page's clock and party reports run on timers.
"$chrome" --headless --no-sandbox --disable-gpu --no-first-run --no-default-browser-check \
  --disable-dev-shm-usage --disable-extensions \
  --disable-background-timer-throttling --disable-renderer-backgrounding --disable-backgrounding-occluded-windows \
  --user-data-dir="$tmp/profile" \
  --remote-debugging-address=$host --remote-debugging-port=$devtools about:blank > "$tmp/chrome.log" 2>&1 &
pids+=($!)

ready=
for _ in $(seq 600); do
  if curl -sf "http://$host:$port/party" > /dev/null && curl -sf "http://$host:$designport/" > /dev/null &&
    curl -sf "http://$host:$devtools/json/version" > /dev/null; then
    ready=1
    break
  fi
  sleep 0.1
done
if [[ -z $ready ]]; then
  echo "server, designer or browser did not start within 60 s" >&2
  echo "--- server log" >&2
  tail -n 20 "$tmp/serve.log" >&2
  echo "--- designer log" >&2
  tail -n 20 "$tmp/design.log" >&2
  echo "--- browser log" >&2
  tail -n 40 "$tmp/chrome.log" >&2
  exit 1
fi

out=${UICHECK_OUT:-$tmp/observed}
status=0
node "$here/uicheck.mjs" "http://$host:$port" "http://$host:$designport" "$host:$devtools" "$tmp/p.html" "$out" "$@" || status=$?
if [[ $status -ne 0 ]]; then
  echo "--- server log" >&2
  tail -n 20 "$tmp/serve.log" >&2
  echo "--- designer log" >&2
  tail -n 20 "$tmp/design.log" >&2
  echo "--- browser log" >&2
  tail -n 20 "$tmp/chrome.log" >&2
fi
exit $status
