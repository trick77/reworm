# reworm

Music party game: a song clip plays on the big screen, a year appears, everyone taps Earlier, Spot on or Later on
their phone, fastest right answer wins. Solo training mode included. Self-hosted, Apple Music previews.

Go backend (stdlib `net/http`, pure-Go SQLite) with the React + TypeScript UI embedded in one binary, shipped as a
container image. Built, gated and released with [ship](https://github.com/trick77/ship) and its plumbing library
kit.

## Develop

Needs Go 1.27 and Node 26.

```sh
make hooks     # once per clone: the pre-push gate
make run       # server on :8080, database in data/dev
make ui-dev    # vite dev server, /api proxied to :8080
make build     # UI + binary into bin/reworm
make check     # the gate: lint, tests, coverage floors, conform
make e2e       # Playwright, chromium + webkit, against the real binary (not part of check)
```

`make e2e` starts the binary in dev mode (`BACKEND_DEV=1`) on a free port with a fresh data dir, clips generated
into a local directory (`BACKEND_DEV_PREVIEWS`) and the fixture songs in `ui/e2e/fixtures` (`BACKEND_DEV_SONGS`), and
pairs its first device through the setup link from the log. Dev mode is for local runs only: refused with
`PUBLIC_FQDN`, on a `BACKEND_ADDR` other than 127.0.0.1, ::1 or localhost, and on a data dir that holds real songs.

## Run

`compose.yaml` runs a released image behind Traefik, from its own directory on the server holding just
`compose.yaml` and `.env` (not a checkout: the checkout's `data/` holds the tracked dataset). It expects the host's
Traefik on the external `traefik` network with a `websecure` entrypoint, an ACME resolver named `letsencrypt` and
the http → https redirect on `web`. There:
`cp .env.example .env` and set `PUBLIC_FQDN`, `mkdir -p data && sudo chown -R 65532:65532 data`, then
`docker compose up -d`. The first device pairs with the one-time setup link from `docker compose logs`. Settings
are `BACKEND_*` variables (`BACKEND_ADDR`, `BACKEND_DATA_DIR`, `BACKEND_LOG_LEVEL`) plus `PUBLIC_FQDN`.

`compose.dev.yaml` builds the image from the checkout and serves it on port 8081 (beside `make run` on 8080; phones
on the LAN reach it at `http://<this machine>:8081`) without Traefik and without `.env`:
`mkdir -p data/compose-dev` (on Linux also `sudo chown 65532:65532 data/compose-dev`), then
`docker compose -f compose.dev.yaml up --build`. The setup link in its log is path-only: open
`http://localhost:8081/setup/<token>`.

## Licence

MIT
