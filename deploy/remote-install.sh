#!/usr/bin/env bash
# Runs on the target, started by deploy.sh with the upload directory and the
# port: installs the uploaded binary and service file and (re)starts the
# service, then checks that it answers.
set -euo pipefail

dir=$1
port=$2
info() { printf '    %s\n' "$*"; }

if systemctl is-active --quiet starbattle; then
	info "service is running (installed $(date -r /usr/local/bin/starbattle '+%Y-%m-%d %H:%M' 2>/dev/null || echo '?')), will be replaced"
else
	info "first installation (or service stopped)"
fi

info "installing /usr/local/bin/starbattle"
sudo install -m 755 "$dir/starbattle" /usr/local/bin/starbattle
info "installing /etc/systemd/system/starbattle.service"
sudo install -m 644 "$dir/starbattle.service" /etc/systemd/system/starbattle.service
rm -rf "$dir"

info "reloading systemd, enabling the service at boot"
sudo systemctl daemon-reload
sudo systemctl enable --quiet starbattle
info "restarting the service"
sudo systemctl restart starbattle

# Wait up to 10 s until the server answers (a tiny puzzle, so it is quick).
info "waiting for the server on port $port"
for _ in $(seq 20); do
	if ! systemctl is-active --quiet starbattle; then
		break
	fi
	if ! command -v curl >/dev/null; then
		info "running (curl not installed, not checking the page)"
		exit 0
	fi
	if curl -fsS -o /dev/null "http://localhost:$port/?n=4&seed=1" 2>/dev/null; then
		info "server answers"
		exit 0
	fi
	sleep 0.5
done

printf '\033[31m    the service did not come up; last log lines:\033[0m\n'
sudo journalctl -u starbattle -n 20 --no-pager
exit 1
