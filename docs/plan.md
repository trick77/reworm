# Build plan — Reworm

Reworm is a **party game**: one screen (iPad, laptop on the TV) plays a song clip and shows a year, everyone answers
on their phone — earlier, spot on, later — and the fastest right answer takes the round. **Training** (solo, adaptive,
progress per song) is a side feature on the same engine.

Status: agreed. Mock: `docs/mock/index.html` (Notes tab = decisions + spike facts).

## Scope and order

1. **Song database** (offline import) — everything else stands on it.
2. **Round engine** — the server owns every round (song, clip, proposed year, scoring); used by both modes.
3. **Party mode** — rooms, QR/code join, phones as answer pads, live scoreboard. The product.
4. **Training mode** — solo sessions with adaptive years and progress. Cheap once 2 exists.
- Later: "Library" decks from your own Apple Music playlists (media-user-token) — D6.

## Built from ship and kit — nothing of this is written again

| Need | Comes from |
|---|---|
| CI, release, image signing/scanning, dependabot, Makefile, .gitignore/.dockerignore, ESLint + tsconfig, pre-push hook | `ship init` / `ship conform` (canon templates) |
| Gate: lint, tests, coverage floors, conform, commit-subject rule | `ship check` (`make check`) |
| Config from env, every fault reported | `kit/env` |
| Logging (slog, level from env) | `kit/logging` |
| Graceful shutdown on SIGTERM | `kit/run` |
| HTTP middleware: access log, body limits, bearer parsing | `kit/httpx` |
| Serving the embedded Vite build (hashed assets cached, app routes → shell, placeholder when unbuilt) | `kit/spa` |
| SQLite DSN (busy timeout, WAL, foreign keys) + numbered migrations | `kit/sqlitex`, `kit/migrate` |
| Signed session cookies (TTL, groups, `Require`/`Guard` middleware) | `kit/auth` Sessions |
| Rate limits (pairing codes, join, answers) | `kit/ratelimit` |
| Constant-time compare of setup token / admin secret | `kit/secret` |
| Live updates to host screen and phones (fan-out per room) | `kit/sse` Hub — no WebSocket library |
| Atomic writes of the import output | `kit/fsx` |
| Version stamp in the binary and UI | `kit/version` |

Written here (nothing in ship/kit covers it): import pipeline, Apple Music client (D5), round engine + year proposal,
rooms, pairing, the UI. A kit gap found on the way → raise it for kit rather than build a local copy.

## Architecture (same family as loom)

- Go backend (stdlib `net/http`) + React SPA (Vite + TS) embedded in the binary. Containerfile, `compose.yaml`,
  uid/gid 65532, pure-Go SQLite (loom's pinned stack) on a data volume, behind Traefik.
- Layout: `backend/` (`cmd/server`, `cmd/import`, `internal/...`), `ui/`, `data/` (song dataset), `ship.yaml`,
  `AGENTS.md`.
- Runtime talks to Apple Music only (preview URLs, covers). No runtime calls to Last.fm, MusicBrainz or chart sites.

## Milestone 0 — device check (before anything else)

The mock's audio only ran in headless desktop Chromium. Serve `docs/mock/` on the LAN, test on a real iPhone and iPad:
- clips audible with the **silent switch on and off** (Web Audio may follow it; candidate fix
  `navigator.audioSession.type = "playback"`, Safari 17+ — verify);
- the next clip starts by itself after the first tap (party host plays dozens of rounds unattended);
- fades, swipe, iPad landscape/portrait layouts, in a tab and installed to the home screen;
- whether the installed home-screen app keeps its own cookie jar (affects pairing).

## 1 — Song database (offline import, `cmd/import`)

Run by hand on the dev machine; output committed. Songs and years don't change: re-run to add genres, sets, a new
year or to apply fixes. Every HTTP response cached on disk, so re-runs are cheap and survive a blocked source.

**Song identity** (stable across re-imports): MusicBrainz recording MBID of the earliest recording, else a normalized
`artist|title` slug. `isrc`, `apple_id`, `storefront`, `cover_url` are mutable attributes. `data/songs.jsonl` is
embedded and upserted into SQLite at boot; progress, history and flags reference the id; songs that leave the
dataset keep their rows (hidden from decks).

1. **Candidates**: card sets (community CSV snapshot, German editions, ~3,300 cards: Essentials, Platinum, Rock, …);
   charts (Wikipedia first: US Year-End Hot 100, CH/DE/AT number-one lists — verify; chart archives only as fallback;
   never a blocker); Last.fm `tag.getTopTracks` for genres (italo, chanson, schlager, christmas, …).
2. **Well-known filter**: Last.fm listeners ≥ per-genre threshold, calibrated on the card sets (10th percentile of
   card songs in that genre; test: ≥ 90 % of card songs pass). Card-set songs bypass it. Count the **song, not the
   recording**: sum (or max) listeners over all recordings of the MusicBrainz work. Measured: Joseíto Fernández's
   Guantanamera has 324 Last.fm listeners, Dancing Queen 2.66 M — a famous song's original recording can be obscure.
   Key: `BACKEND_LASTFM_API_KEY` in the uncommitted `.env.local` (tested 2026-11-03).
3. **Initial year**: card year for card sets; else MusicBrainz min `first-release-date` over all recordings of the
   ISRC. Apple's date only as a cross-check (report if off > 1). No trustworthy year → dropped.
4. **Apple match**: `songs?filter[isrc]` (paginate past 25) → prefer the original-album version (MusicBrainz
   release-group title + date → Apple album search → same ISRC); fallback artist+title. Storefronts ch → de → us,
   stored with the apple_id. Must have `previews[]` (90 s AAC, starts ~10 s before the chorus — no intros).
5. **Cover**: Apple artwork if the match is the original album; else Cover Art Archive front of the first release
   group; else none (UI draws a record).
6. **Genres**: Last.fm top tags → the app's genre list (mapping table, tested).
7. **Categories**, rule each; v1 = Gen Alpha (2016 on, no upper bound: what kids born 2010+ know), Smash Hits,
   Number ones, Evergreens (< 1950), Christmas, Movie & TV, Summer, Party, Swiss charts, Eurovision. Later: One-hit
   wonders, Guilty pleasures, Love songs (fuzzy).
8. **Output**: `data/songs.jsonl` (diffable) + `data/overrides.json` (hand fixes by song id, applied last) + import
   report (unmatched, no preview, year conflicts, low-confidence matches, per-deck counts).
9. **Politeness**: MusicBrainz 1 req/s with app name + contact in the User-Agent (D8); retries for truncated Apple
   bodies.

Verify: per-deck numbers; 50 random songs checked by ear (year, clip, cover); spike songs pass (Dancing Queen 1976 on
*Arrival*, Bohemian Rhapsody 1975, Guantanamera card 1929).

## 2 — Round engine (shared)

- **Apple client**: token from the web player bundle (tunewire's approach), renewed before expiry. `apple_id +
  storefront → preview URL`, cached ~24 h; gone → re-lookup by ISRC → else skip + log.
- **Round** = song, clip start/length, proposed year, answer window. Clients get the preview URL and the proposed
  year, never the answer before the reveal. The next round's clip is handed out early so the stage prefetches it.
- **Year proposal**: tiers easy ±10–25 / clear ±4–9 / tight ±1–3 / exact, base mix per difficulty. `fair` (party):
  same year for everyone, no per-player shift. Adaptive (training): shift per card (warm-up, streak, last miss, song
  state, re-ask). Direction 50/50 unless one side passes 1900 / now. Go tests on the distributions.
- **Song picking**: from the chosen decks + year range; no repeats within a game; songs played in the host's recent
  games go to the back.

## 3 — Party mode (the product)

**Devices and roles**
- **Stage** (host): iPad or laptop (on the TV). Plays audio, shows the year, countdown, who's locked in, the reveal
  and the scoreboard. Started from a paired device (D3).
- **Answer pad** (players): phones. Join by QR (`https://<domain>/join/CODE`) or the 4-letter code, pick name +
  avatar. No account: a room-scoped guest cookie (`kit/auth` Sessions, short TTL, `guest` group). No audio on phones.

**A round**
1. Stage plays the clip (fade in) and shows the proposed year; pads unlock.
2. Players tap Earlier / Spot on / Later. The server stamps each answer on arrival.
3. First **correct** answer takes the round; a wrong answer locks that player out for the round. "Spot on" when it is
   the exact year scores a bonus (D11).
4. Round ends on the first correct answer, when all are locked out, or at the timer (D11). Reveal on the stage (cover,
   year, who won, how far off) and on every pad (your result).
5. First to N points wins the game; final scoreboard on the stage, rematch button.

**Live updates**: `kit/sse` hub per room pushes state (lobby, round start, locks, result, scores) to stage and pads;
answers go up as `POST`s. On reconnect a client fetches the room snapshot, so a phone that slept rejoins mid-game.
Fairness = server arrival order; latency compensation only if testing shows it matters.

**Room lifecycle**: lobby (players join, host picks decks/rounds/clip/difficulty), playing, finished; host can kick a
player and pause; late join allowed between rounds; rooms expire after inactivity; max 8 players (D11).

Mocks still missing for this (next design step): stage during a round, pad during a round (unlocked / locked in /
locked out), round result on both, scoreboard, game over.

## 4 — Training mode (side feature)

Solo session on one device (stage and pad combined, as in the current mock): adaptive year proposal, missed songs
re-asked a few cards later, per-song progress (new/learning/mastered, spaced repetition), decade accuracy, XP,
streak. Session state on the server, so a reload or a backgrounded tab resumes at the current card.

## UI (both modes)

- Components split **Stage** and **Answer pad**: separate devices in party, combined in training.
- Audio: per milestone 0. Baseline `<audio crossOrigin>` → Web Audio GainNode; fade in 1.5 s, out 0.8 s (end, answer,
  pause, skip); AudioContext unlocked by the host's first tap; next clip prefetched.
- Layout: container queries — iPhone single column; iPad landscape two columns; iPad portrait stacked; stage scales
  to a TV across the room; installable PWA.
- Home: Game night on top (Host / Join), training below; tabs Home · Party · Train · Settings.
- Profiles: "Who's playing?" on shared devices, profile page with avatar picker, devices, "Add a device" / "Invite
  someone". Avatars: own set of 12 comic music-object SVGs (mock), shipped as static files.
- Covers hidden until the reveal. Brand-free everywhere (D10).

Verify: on a real iPhone + iPad (and a laptop as stage) through a full game with 3+ phones and a training session;
Playwright (Chromium + WebKit; phone, iPad, multi-context party game), vitest, Go tests; `ship check`.

## Milestones
0. Device check of the mock (audio, silent switch, autoplay chain, layouts, cookie jar).
1. ~~Party gameplay mocks~~ — done in the mock (stage: round live, result, game over; pad: live, locked out, won,
   someone faster).
2. Repo bootstrap: `trick77/reworm` (public, description + topics per D1), `ship init` + conform green, AGENTS.md (incl. D10), Containerfile, compose.
3. Import v1: card sets + MusicBrainz + Apple + covers → `songs.jsonl` + report.
4. Import v2: charts + Last.fm filter (calibrated) + genres + v1 categories.
5. Round engine + Apple client + pairing/profiles, tests.
6. Party mode: rooms, join, live updates, rounds, scoreboard — stage on iPad/laptop, pads on phones.
7. Training mode on the same engine.
8. Deploy on the home server.

## Decisions

All decided.

- **D1** Name Reworm: game, repo `trick77/reworm`, module, image.
  - GitHub description (set with `gh repo create`/`gh repo edit`, brand-free):
    "Music party game: a song clip plays on the big screen, a year appears, everyone taps Earlier, Spot on or Later
    on their phone, fastest right answer wins. Solo training mode included. Self-hosted, Apple Music previews."
  - Topics: `party-game`, `music-quiz`, `apple-music`, `self-hosted`, `go`, `react`, `pwa`.
- **D2** Public repo.
- **D3 Auth**: no passwords, paired devices.
  - Cookie via `kit/auth` Sessions (TTL ~1 year, `admin` group for you). Signed cookies can't be revoked on their
    own, so the claim carries a device id and a `devices` table is checked on each request → a lost phone can be
    removed.
  - "Add a device": a paired device shows a QR (`/pair/<token>`) + 6-character code; 5 min, single use,
    `kit/ratelimit`. "Invite someone": same, creates a new profile.
  - First device: one-time setup link in the server log on first boot (compared with `kit/secret`).
  - Party guests: room-scoped guest sessions, no profile needed.
- **D4 Covers**: all cached on our server (fetched by the import or on first use, ~25 KB × ~10k ≈ 250 MB on the data
  volume, not in the repo). No outside image host during a game.
- **D5 Apple client**: copied from tunewire first; both copies drifted within days, so it is github.com/trick77/toolkit's
  applemusic now (2026-11-04), shared with loom.
- **D6** Library decks later.
- **D7 Last.fm API key**: registered ("Reworm"), in `.env.local` as `BACKEND_LASTFM_API_KEY`, tested. Shared secret unused.
- **D8 MusicBrainz contact**: `https://github.com/trick77/reworm` in the User-Agent.
- **D9 Progress theme** (training): record awards, Bronze → Silver → Gold → Platinum → Diamond.
- **D10** Brand-free everywhere (source URL in uncommitted local config, neutral file names, rule in AGENTS.md).
- **D11 Party rules**: first correct answer = 1 point, +1 if it's a correct "Spot on"; wrong answer = locked out for
  the round; 15 s answer window; first to 10 points wins; max **8** players; latency compensation only if testing
  shows it's needed. The stage shows who is locked out, never what they answered (that would give the direction
  away).
- **D12 Pre-1950 songs without a card**: initial year = earliest MusicBrainz recording (Guantanamera → 1940; the
  card keeps 1929).
- **D13 Set names** (own names, confirmed): Essentials, Mega Mix, Rock Anthems, Schlager Fest, Sunshine, Cheesy
  Classics, Party Starters, Crowd Pleasers, Screen Hits, Christmas Hits, Radio Gold. The import maps each source
  snapshot file to one of these (mapping in the uncommitted local config, D10).
- **D14 Buzzer mode**: second party mode, picked in the lobby next to Earlier/Later. No year shown: the first buzz
  pauses clip and timer, the buzzer has 10 s to say artist and title out loud. Buzzer + one other press Reveal; buzzer
  + one other judge 2 (both) / 1 (one) / 0, disagreement → the rest of the room breaks the tie (lower median of all
  votes). With 2 players "one other" is the opponent. One buzz per round; nobody in 15 s → reveal, 0 points.
- **D15 Photo finish**: after the decisive press (first correct answer, the buzz) the server keeps a 1 s window and
  records later presses with their delta (years: other correct answers only; buzzer: every buzz). Order = server
  arrival time; < 50 ms is shown as "photo finish". Latency compensation only if testing shows it's needed.
- Also: initial year only · original covers only.
