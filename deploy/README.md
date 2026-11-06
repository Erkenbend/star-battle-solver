# Hosting on a Raspberry Pi

`starbattle serve` is a single static binary (page assets are embedded), so
hosting means: cross-compile, copy, run as a systemd service.

## Requirements

- On this machine: Go (see `go.mod`), `ssh`, `scp`.
- On the Pi: any Raspberry Pi OS (or other systemd Linux), SSH enabled, a user
  with `sudo`. An SSH key (`ssh-copy-id pi@raspberrypi.local`) avoids typing the
  password several times.

## Install and update

```sh
deploy/deploy.sh pi@raspberrypi.local
```

The script asks the Pi for its CPU (`uname -m`: 64-bit `aarch64`, 32-bit
`armv7l`, or `armv6l` on Pi Zero / Pi 1), builds for it, installs
`/usr/local/bin/starbattle` and `/etc/systemd/system/starbattle.service`, and
(re)starts the service. Run it again after every change.

The page is then at `http://raspberrypi.local:8080/`.

## Configuration

Optional `/etc/default/starbattle` on the Pi:

```sh
STARBATTLE_ADDR=:8080            # listen address; :80 works too
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
  (worst case ~10 s); expect several times that on a Pi.
- There is no login and no HTTPS. Fine for the home network; to expose it to
  the internet, put a reverse proxy with TLS in front (e.g. Caddy:
  `caddy reverse-proxy --from puzzles.example.org --to localhost:8080`) and
  change `STARBATTLE_ADDR` to `localhost:8080`.
