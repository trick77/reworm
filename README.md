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
```

## Run

`compose.yaml` runs a released image behind Traefik, so it works from the first release on (`0.1.0`): set
`PUBLIC_FQDN` in `.env`, `mkdir -p data && sudo chown -R 65532:65532 data`, then `docker compose up -d`. Settings are `BACKEND_*`
variables (`BACKEND_ADDR`, `BACKEND_DATA_DIR`, `BACKEND_LOG_LEVEL`) plus `PUBLIC_FQDN`.

## Licence

MIT
