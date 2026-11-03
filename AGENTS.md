# reworm

Self-hosted music party game: Go backend + embedded React SPA. Plan: `docs/plan.md`; UI spec: `docs/mock/index.html`.
Party mode is the product, training a side feature on the same round engine.

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
- Env prefix is `BACKEND_`, never `REWORM_` (as loom): `BACKEND_ADDR` (`:8080`), `BACKEND_DATA_DIR`
  (`/data`), `BACKEND_LOG_LEVEL`, via kit/env (empty = unset). The host is plain `PUBLIC_FQDN` only: the Traefik
  rule reads it too, one name so app and router never disagree.
- Secrets only via env / `.env.local` (import: `BACKEND_LASTFM_API_KEY`, `BACKEND_CARDS_BASE_URL`,
  `BACKEND_CARDS_FILES` = `file=Set Name;…`), never committed.
- Container runs as uid/gid 65532 (distroless nonroot), never 1000: 1000 is the host's first real user.

## Song data
- Year = the song's INITIAL release year, never a re-release, remaster or compilation date. No trustworthy year
  → song dropped. Pre-1950 without a card: earliest MusicBrainz recording.
- Original covers only: Apple artwork only when the match is the original album, else Cover Art Archive of the
  first release group, else none (UI draws a record).
- Song id is stable across re-imports (earliest recording MBID, else `artist|title` slug); progress and history
  reference it. Never edit an applied migration; next free `store/migrations/NNNN_*.sql`.
- Rebuild needing FKs off: first line `-- migrate:no-transaction`, then `PRAGMA foreign_keys = OFF; BEGIN; …
  COMMIT; PRAGMA foreign_keys = ON;` (SQLite ignores the pragma inside a transaction, kit runs each file in one).
- MusicBrainz: 1 req/s, User-Agent `Reworm/<version> ( https://github.com/trick77/reworm )`. Every import HTTP
  response cached on disk (`data/cache/`, ignored); a cold full import runs ~1 h, a rerun minutes.
- Hand fixes: `data/overrides.json`, not YAML: go-yaml has 543 stars (2026-11), under the dependency bar.
- Apple `songs?filter[isrc]` answers max 25 per ISRC, no next/offset (2026-11-03): a hit's original album drowns in
  compilations, so the import also searches the MusicBrainz album by name. Apple back-dates compilation tracks to
  the original year: the album name decides "original", not the date.
- Runtime talks to Apple Music only (previews, covers); Last.fm, MusicBrainz, chart sites are import-only.

## Apple Music
- Access is unofficial: the web player's bearer token scraped from its JS bundle (loom's tunewire approach),
  renewed before expiry. Can break any time; previews are 90 s AAC starting near the chorus.
