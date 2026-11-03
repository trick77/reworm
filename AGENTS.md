# reworm

Self-hosted music party game: Go backend + embedded React SPA. Plan: `docs/plan.md`; UI spec: `docs/mock/index.html`;
PR stack: `docs/progress.md`. Party mode is the product, training a side feature on the same round engine.

## Working conventions
- Docs, specs, code comments English only. TDD. YAML is `.yaml`, container builds `Containerfile`.
- One branch per change, never commit to `master`.
- Commit subjects + PR titles: Go style `area: what` (`import: …`, `engine: …`, `party: …`, `train: …`, `api: …`, `ui: …`, `store: …`, `apple: …`, `auth: …`, `docs: …`), gated by ship; release notes list them by area.
- Fresh clone: `ship hooks` before the first push (check, conform and init install it too); without the pre-push hook a push skips the gate.
- Gate: `make check` (`ship check`): conform, lint, tests, floors 75 (Go, UI), patch 75. Raise tests, never a floor.
- kit first (`github.com/trick77/ship/kit`): env, logging, run, httpx (probes, middleware), spa, sqlitex, migrate,
  auth, ratelimit, secret, sse, fsx, version. Never hand-roll what kit or ship provide; a gap → raise it for kit, no
  local copy. Lint config is ship's.

## Brand-free (hard rule)
- The commercial card game this is inspired by is never named anywhere: code, data, comments, docs, file names,
  commits, PRs. Own set names only (plan D13).
- The card-list source URL and the snapshot → set mapping live only in the uncommitted `.env.local`.
- Before every commit: case-insensitive grep of the tree and the commit message for the name.

## Config and secrets
- `REWORM_*` env via kit/env (empty = unset): `ADDR` (`:8080`), `DATA_DIR` (`/data`), `PUBLIC_FQDN`, `LOG_LEVEL`.
  Compose maps `PUBLIC_FQDN` (also the Traefik rule) to `REWORM_PUBLIC_FQDN`.
- Secrets only via env / `.env.local` (import: `LASTFM_API_KEY`), never committed.
- Container runs as uid/gid 65532 (distroless nonroot), never 1000: 1000 is the host's first real user.

## Song data
- Year = the song's INITIAL release year, never a re-release, remaster or compilation date. No trustworthy year
  → song dropped. Pre-1950 without a card: earliest MusicBrainz recording.
- Original covers only: Apple artwork only when the match is the original album, else Cover Art Archive of the
  first release group, else none (UI draws a record).
- Song id is stable across re-imports (earliest recording MBID, else `artist|title` slug); progress and history
  reference it. Never edit an applied migration; next free `store/migrations/NNNN_*.sql`.
- MusicBrainz: 1 req/s, User-Agent `Reworm/<version> ( https://github.com/trick77/reworm )`. Every import HTTP
  response cached on disk (`data/cache/`, ignored).
- Runtime talks to Apple Music only (previews, covers); Last.fm, MusicBrainz, chart sites are import-only.

## Apple Music
- Access is unofficial: the web player's bearer token scraped from its JS bundle (loom's tunewire approach),
  renewed before expiry. Can break any time; previews are 90 s AAC starting near the chorus.
