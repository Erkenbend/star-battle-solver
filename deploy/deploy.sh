#!/usr/bin/env bash
# Builds starbattle for the target machine (e.g. a Raspberry Pi) and installs
# it there as the systemd service "starbattle".
#
#   deploy/deploy.sh [user@]host
#
# Needs SSH access to host and sudo rights there. Run it again to update.
# All SSH and SCP calls share one connection, so a password is asked for at
# most once (plus once for sudo, if sudo on the host wants one).
set -euo pipefail

host=${1:?usage: deploy/deploy.sh [user@]host}
cd "$(dirname "$0")/.."

total=5
step() { printf '\n\033[1m==> [%d/%d] %s\033[0m\n' "$1" "$total" "$2"; }
info() { printf '    %s\n' "$*"; }
die() { printf '\033[31merror: %s\033[0m\n' "$*" >&2; exit 1; }

# Short directory: it holds the SSH control socket, whose path may not be
# longer than about 100 characters (TMPDIR on macOS is too long).
tmp=$(mktemp -d /tmp/starbattle-deploy.XXXXXX)
ssh_opts=(-o ControlPath="$tmp/ssh-%C")
cleanup() {
	ssh "${ssh_opts[@]}" -O exit "$host" 2>/dev/null || true
	rm -rf "$tmp"
}
trap cleanup EXIT
run() { ssh "${ssh_opts[@]}" "$host" "$@"; }

step 1 "Connecting to $host"
info "(asks for the SSH password unless you use an SSH key; only this once)"
ssh "${ssh_opts[@]}" -o ControlMaster=yes -o ControlPersist=yes -fN "$host" ||
	die "cannot connect to $host"
info "connected"

# One round trip for everything we need to know about the host.
read -r arch remote port < <(run '
	addr=$(sed -n "s/^STARBATTLE_ADDR=//p" /etc/default/starbattle 2>/dev/null || true)
	port=${addr##*:}
	echo "$(uname -m) $(mktemp -d) ${port:-6369}"
')
[[ -n ${arch:-} && -n ${remote:-} ]] || die "could not query $host"
case $arch in
	aarch64 | arm64) export GOARCH=arm64 ;;
	armv7l) export GOARCH=arm GOARM=7 ;;
	armv6l) export GOARCH=arm GOARM=6 ;; # Pi Zero / Pi 1
	x86_64) export GOARCH=amd64 ;;
	*) die "unsupported architecture: $arch" ;;
esac
info "CPU: $arch → building for linux/$GOARCH${GOARM:+ (ARMv$GOARM)}"

step 2 "Building"
info "$(go version)"
start=$SECONDS
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o "$tmp/starbattle" ./cmd/starbattle
info "starbattle: $(du -h "$tmp/starbattle" | cut -f1) in $((SECONDS - start)) s, commit $(git rev-parse --short HEAD)$(git diff --quiet HEAD 2>/dev/null || echo ' + uncommitted changes')"

step 3 "Uploading to $host:$remote"
scp -q "${ssh_opts[@]}" "$tmp/starbattle" deploy/starbattle.service deploy/remote-install.sh "$host:$remote/"
info "binary, service file and install script uploaded"

step 4 "Installing on $host"
if run sudo -n true 2>/dev/null; then
	run bash "$remote/remote-install.sh" "$remote" "$port"
else
	info "(sudo on $host asks for your password)"
	ssh -t "${ssh_opts[@]}" "$host" bash "$remote/remote-install.sh" "$remote" "$port"
fi

step 5 "Done"
info "running on http://${host#*@}:$port/ (deployed in $SECONDS s)"
info "log: ssh $host journalctl -u starbattle -f"
