# Hosting on a Raspberry Pi

`starbattle serve` is a single static binary (page assets are embedded), so
hosting means: cross-compile, copy, run as a systemd service.

## Requirements

- On this machine: Go (see `go.mod`), `ssh`, `scp`.
- On the Pi: any Raspberry Pi OS (or other systemd Linux), SSH enabled, a user
  with `sudo`. The script asks for the SSH password once (all steps share one
  connection), and for the sudo password only if sudo on the Pi wants one. With
  an SSH key (`ssh-copy-id pi@raspberrypi.local`) there is no prompt at all.

## Install and update

```sh
deploy/deploy.sh pi@raspberrypi.local
```

The script reports five steps: connect; build for the Pi's CPU (`uname -m`:
64-bit `aarch64`, 32-bit `armv7l`, or `armv6l` on Pi Zero / Pi 1); upload;
install (`deploy/remote-install.sh` on the Pi puts the binary in
`/usr/local/bin/starbattle` and the unit in
`/etc/systemd/system/starbattle.service`, restarts the service and waits until
the page answers, otherwise it shows the last log lines and fails); done. Run it
again after every change.

The page is then at `http://raspberrypi.local:6369/`.

## Configuration

Optional `/etc/default/starbattle` on the Pi:

```sh
STARBATTLE_ADDR=:6369            # listen address; :80 works too
STARBATTLE_OPTS=-search lookahead   # any solver flags of `starbattle serve`
```

Then `sudo systemctl restart starbattle`.

## Operating

```sh
systemctl status starbattle      # running?
journalctl -u starbattle -f      # log
sudo systemctl disable --now starbattle   # stop for good
sudo rm /usr/local/bin/starbattle /etc/systemd/system/starbattle.service   # uninstall
```

## Notes

- The service runs as a throwaway user (`DynamicUser`) with no write access to
  the system and at `Nice=5`, so the Pi stays responsive while a puzzle is
  generated.
- Generating 13×13 and 14×14 puzzles takes a few seconds on a desktop CPU
  (worst case ~10 s); expect several times that on a Pi. So the server keeps
  one puzzle of every size ready and generates the next one in the background
  (`journalctl` shows "14×14 bereit …"). Only someone asking for the same size
  again before that is done, or right after a restart, has to wait.
  `STARBATTLE_OPTS=-no-pregen` turns this off.
- "Zusammen spielen" (`/party`) keeps parties in memory only: a restart
  (e.g. by `deploy.sh`) ends all running parties. Its live updates are
  server-sent events; a reverse proxy in front must not buffer them (nginx:
  the server already sends `X-Accel-Buffering: no`).
- There is no login and no HTTPS. Fine for the home network; to expose it to
  the internet, put a reverse proxy with TLS in front (e.g. Caddy:
  `caddy reverse-proxy --from puzzles.example.org --to localhost:6369`) and
  change `STARBATTLE_ADDR` to `localhost:6369`.
