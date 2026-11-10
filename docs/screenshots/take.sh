#!/usr/bin/env bash
# Takes the README screenshots anew: builds starbattle, starts the server and
# a headless Chromium, and lets shots.mjs drive the pages.
#
#   docs/screenshots/take.sh            all pictures
#   docs/screenshots/take.sh annahme    only some (names as the PNG files)
#
# Environment:
#   CHROME  Chromium/Chrome (default: chromium, google-chrome or Playwright's)
#   FONTS   directory with a font for ⏱ (e.g. Noto Emoji) if the system has
#           none; otherwise the clock shows an empty box.
# Needs go, node >= 22 and curl.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
port=6398
devtools=9338

chrome=${CHROME:-}
if [[ -z $chrome ]]; then
  chrome=$(command -v chromium || command -v chromium-browser || command -v google-chrome || true)
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

if [[ -n ${FONTS:-} ]]; then
  cat > "$tmp/fonts.conf" <<EOF
<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "fonts.dtd">
<fontconfig>
  <include ignore_missing="yes">/etc/fonts/fonts.conf</include>
  <dir>$(cd "$FONTS" && pwd)</dir>
  <cachedir>$tmp/fontcache</cachedir>
</fontconfig>
EOF
  export FONTCONFIG_FILE=$tmp/fonts.conf
fi

(cd "$root" && go build -o "$tmp/starbattle" ./cmd/starbattle)
"$tmp/starbattle" serve -addr "localhost:$port" -no-pregen > "$tmp/serve.log" 2>&1 &
pids+=($!)
"$chrome" --headless --no-sandbox --disable-gpu --hide-scrollbars --no-first-run \
  --user-data-dir="$tmp/profile" --remote-debugging-port=$devtools about:blank > "$tmp/chrome.log" 2>&1 &
pids+=($!)

for _ in $(seq 100); do
  curl -sf "localhost:$port/party" > /dev/null && curl -sf "localhost:$devtools/json/version" > /dev/null && break
  sleep 0.1
done

node "$here/shots.mjs" "http://localhost:$port" "$devtools" "$here" "$root/testdata/hard/n11_s095.txt" "$@"
