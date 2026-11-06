#!/usr/bin/env bash
# Builds starbattle for the target machine (e.g. a Raspberry Pi) and installs
# it there as the systemd service "starbattle".
#
#   deploy/deploy.sh [user@]host
#
# Needs SSH access to host and sudo rights there. Run it again to update.
set -euo pipefail

host=${1:?usage: deploy/deploy.sh [user@]host}
cd "$(dirname "$0")/.."

arch=$(ssh "$host" uname -m)
case $arch in
	aarch64 | arm64) export GOARCH=arm64 ;;
	armv7l) export GOARCH=arm GOARM=7 ;;
	armv6l) export GOARCH=arm GOARM=6 ;; # Pi Zero / Pi 1
	x86_64) export GOARCH=amd64 ;;
	*)
		echo "unsupported architecture: $arch" >&2
		exit 1
		;;
esac

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "building for linux/$GOARCH${GOARM:+ (ARMv$GOARM)}"
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o "$tmp/starbattle" ./cmd/starbattle

remote=$(ssh "$host" mktemp -d)
scp -q "$tmp/starbattle" deploy/starbattle.service "$host:$remote/"
# -t so that sudo can ask for a password.
ssh -t "$host" "
	set -e
	sudo install -m 755 $remote/starbattle /usr/local/bin/starbattle
	sudo install -m 644 $remote/starbattle.service /etc/systemd/system/starbattle.service
	rm -rf $remote
	sudo systemctl daemon-reload
	sudo systemctl enable --quiet starbattle
	sudo systemctl restart starbattle
	sleep 1
	systemctl is-active starbattle
"

addr=$(ssh "$host" "sed -n 's/^STARBATTLE_ADDR=//p' /etc/default/starbattle 2>/dev/null" || true)
port=${addr##*:}
echo "running on http://${host#*@}:${port:-8080}/"
