# Reworm HTTP API (v1) — shared by backend and UI

JSON over HTTP. Live party state over SSE (kit/sse). Cookies: device session `reworm_device` (kit/auth Sessions,
1 year, claim = device id, checked against `devices` on every request, so a removed device is signed out at once) and
room guest session `reworm_guest` (12 h, path `/api/rooms/`, claim = player id + room code). All times are server
unix milliseconds. Errors: `{ "error": "<code>", "message": "<text>" }` with a proper status; kit's own refusals carry
`error` only: 400 `malformed request` (body does not parse), 413 `request body too large`, 429 `rate limited` (with
`Retry-After`).

## Profiles, devices, pairing (D3)
- `GET  /api/me` → `{ profile: {id,name,avatar,admin}, device: {id,label} }` | 401 `unauthorized` (not paired)
- `POST /api/setup` `{token, name, avatar}` → 201 + `/api/me` body. First device of a fresh server: the one-time
  token is printed in the server log on boot (`/setup/<token>` link) while no profile exists. Sets the cookie; that
  profile is the admin. 403 `invalid_token`, 410 `setup_done` once a profile exists.
- `GET  /api/profiles` → `[{id,name,avatar}]` (paired devices only; "Who's playing?")
- `POST /api/profiles/switch` `{profileId}` → `/api/me` body; moves this device to that profile, re-issues the cookie.
  Never into the admin profile (403 `admin_profile`) unless the device is on it already: admin only by pairing from
  an admin device ("Add a device").
- `PATCH /api/me` `{name?, avatar?}` → `/api/me` body. Names: 1–24 printable characters (400 `invalid_name`).
- `GET  /api/devices` → `[{id,label,lastSeen,current}]` (this profile's devices, last seen first; `lastSeen` is
  refreshed at most every 5 min); `DELETE /api/devices/{id}` → 204 (own profile's devices; the admin: any). Removing
  the current device clears its cookie.
- `POST /api/pair` `{kind: "device"|"invite"}` → 201 `{code: "H4K9QZ", url: "https://<host>/pair/H4K9QZ", expiresAt}`
  (5 min, single use; alphabet without 0/O/1/I; rate-limited per client address). `url` is absolute only when the
  server has `PUBLIC_FQDN`; otherwise it is the path `/pair/<code>` and the client prefixes its own origin (never
  built from the request's Host header).
- The admin profile's only device cannot leave it: switching away, deleting it or claiming a code over it → 409
  `last_admin` (pair a second device first).
- `POST /api/pair/claim` `{code, name?, avatar?}` → 201 + `/api/me` body, sets the cookie. `device` adds this device to
  the code's profile, `invite` creates a new profile (name+avatar required, a bad one leaves the code usable). A
  browser already paired drops its old device row. 404 `invalid_code` (wrong, used or expired). Rate-limited.
- avatar = one of: vinyl cassette boombox mic worm disco guitar drum note keys speaker jukebox (400 `invalid_avatar`)
- Device label is guessed from the User-Agent: iPhone, iPad, Android, Mac, Windows, Chromebook, Linux, Browser.

## Decks
- `GET /api/decks` → `{ sets: [{id,name,count,from,to}], categories: [{id,name,count}], genres: [{id,name,count}] }`
  (active songs only). Ids are the import's slugs (`essentials`, `mega-mix`, `number-ones`, `pop`, …); names and
  order come from the backend's list (sets per D13), unknown slugs title-cased after them.
- deck selector object used below: `{ sets: [ids], categories: [ids], genres: [ids], from: 1900, to: 2025 }` — songs in
  any listed set ∪ category ∪ genre (all three empty = every song), with a year in [from, to] (defaults 1900 and the
  current year; at most 100 ids)
- `GET /api/covers/{songId}` → image (server fetches the stored cover_url on first use — Apple `{w}x{h}` at
  600x600, or the CAA URL, redirects followed — caches it under `BACKEND_DATA_DIR/covers/`, D4;
  `Cache-Control: public, max-age=31536000, immutable`); 404 → UI draws the placeholder (no cover, or the fetch failed)

## Party rooms (main feature)
- `POST /api/rooms` (paired device = host) `{mode: "years"|"buzzer", deck, toWin: 10, clip: {length: 20, start:
  "chorus"|"random"}, difficulty: "easy"|"medium"|"hard"}` → 201 `{code: "K7Q4"}`. Every field optional (defaults as
  shown; toWin 1–50, clip length 5–60 s). 400 `invalid_settings`.
- Codes are 4 symbols, case-insensitive. A room expires after 2 h without any action. Every room route meters
  unknown codes per client address (20, then 10 a minute; 429), so codes cannot be guessed cheaply; a known room
  costs nothing there. Reads (`GET` state, events, me) are limited to 5/s per address (burst 60).
- `GET  /api/rooms/{code}` → `RoomState` (snapshot; also used after SSE reconnect) | 404 `unknown_room`
- `GET  /api/rooms/{code}/events` → SSE, event `state` with the full `RoomState` on every change, the current one
  first (small payload; simplest correct). `: keepalive` comments every 25 s. 503 `too_many_streams` past 64 per
  room or 12 per client address (a party shares one router). A browser that is neither paired nor holds a guest
  cookie for the room gets one with the stream; joining later on that cookie makes the open stream count as the
  player's connection (`connected`). `nextClip` is resolved in the background after start/next and arrives with a
  later `state` event.
- `GET  /api/rooms/{code}/me` → `{playerId}` the guest cookie or paired profile plays as | 401 `not_in_room` (no
  session for this room) | 403 `kicked`. The pad asks this instead of trusting its own storage.
- `POST /api/rooms/{code}/join` `{name, avatar}` → guest cookie, `{playerId}` (paired profiles join with their
  profile, body optional; max 8 players, 409 `room_full`). The same cookie or profile joining again gets the same
  player (reconnect). A player joining during a round is `late` and plays from the next round. A kicked player or
  profile cannot rejoin the room (403 `kicked`). Rate-limited.
- host only (the profile that created the room; 403 `not_host`, 401 unpaired), each answers the new `RoomState`:
  `POST …/start` (lobby; 409 `no_players`, 422 `deck_empty`), `…/next` (after a result; no song left → `over`),
  `…/pause` / `…/resume` (during a round; paused time extends every deadline), `…/rematch` (over → lobby, scores 0),
  `…/kick {playerId}` (for the room's lifetime; during a pause the round is settled on resume), `…/settings {…same
  as create}` (lobby only). Wrong phase → 409 `wrong_phase`.
- years mode: `POST …/answer {round, choice: "earlier"|"exact"|"later"}` → `{accepted: true}`; correctness only via state
- buzzer mode: `POST …/buzz {round}`, `POST …/reveal {round}`, `POST …/judge {round, points: 0|1|2}` → `{accepted: true}`
- Player actions are rate-limited per client address (generous: a party shares one router). Refusals: 401
  `not_in_room` (no guest cookie / profile player for this room — never silent), 403 `kicked`, 409 `wrong_mode`,
  `wrong_round` (not the running round, or over), `already_answered`, `already_buzzed`, `late` (joined mid-round),
  `wrong_phase` (paused).
- `start`, `next` and training need preview URLs: 503 `previews_unavailable` while no preview source is configured
  or when it fails (retry); a song the source has no preview for is skipped.

### RoomState
```
{ code, seq, now, mode, phase: "lobby"|"playing"|"buzzed"|"judging"|"result"|"paused"|"over", hostId, toWin,
  settings: {deck, clip, difficulty},
  players: [{ id, name, avatar, score, connected, state: "thinking"|"out"|"late"|"buzzed"|"answered" }],
  round: null | {
    n, startedAt, endsAt,                    // 15 s answer window; paused time extends endsAt
    proposedYear,                            // years mode only
    clip: { previewUrl, from, length },      // the stage plays this; pads never play audio
    buzz: null | { playerId, at, answerEndsAt },        // buzzer mode: clip+timer paused, 10 s to say it
    revealVotes: [playerIds],                // buzzer: buzzer + one other must press Reveal
    judgeVotes: { [playerId]: 0|1|2 },       // buzzer: buzzer + one other must agree; tie → rest of room
    judgeEndsAt: null | ms,                  // buzzer: from the reveal on, when the votes so far decide (20 s)
    result: null | { song: {id, artist, title, year, coverUrl}, winnerId, winnerMs, points,
                     photoFinish: [{ playerId, deltaMs }] }   // D15: later presses within 1 s
  },
  nextClip: null | { previewUrl, from, length } }  // prefetch for the stage
```
- `seq` grows by one with every change of the room: keep the state with the highest, drop older snapshots/events.
  `now` is the server clock (unix ms) when the state was written: clients derive their clock offset for countdowns.
- `winnerMs` is the winning press (years: the answer, buzzer: the buzz) in ms after `startedAt`, paused time left
  out; null without a winner.
- `hostId` is the host's profile id. `connected` = the player has an event stream open. `clip.from` is seconds into
  the 90 s preview (0 for `chorus`). `coverUrl` is `/api/covers/{id}` or null. `winnerId` is null when nobody scored.
  `nextClip` is null when the deck has nothing left. `round` stays after the result (and in `over`) until the next one.
- `answered` = gave a correct answer (years); `out` = wrong answer this round (years); `late` = joined mid-round,
  plays from the next round.

Rules (D11/D14/D15): years mode — first CORRECT answer scores 1 (+1 if "exact" and right), wrong → that player "out"
for the round (stage shows ✕, never the choice), round ends on first correct / all out / timer. Buzzer — first buzz
pauses; buzzer + one other press Reveal (a room of one: the buzzer alone), or the 10 s answer window ends → phase
`judging`, and from here `round.result` already carries the song with `points: 0`, `winnerId: null`; buzzer + one other
judge 2/1/0 alike → that; else once everyone voted (or 20 s after the reveal) the lower median of the votes decides, so
the others break the tie (2 players disagreeing → the lower vote). 0 points → no winner. One buzz per round; nobody in
15 s → reveal, 0 points. Photo finish: server keeps a 1 s window after the decisive press, by arrival time on the
server; years: other correct answers only; buzzer: all buzzes; < 50 ms shown as "photo finish". First to `toWin` →
"over" (the last result stays in `round`). The song (artist/title/year) is never sent before `result` exists.

## Training (side feature)
- `POST /api/train/sessions` `{deck, mode: "years"|"pin", difficulty, clip, length, songIds?}` → 201 `{id, card,
  now}` (paired device; length 1–50, default 10). Filled to `length` with songs that have a preview; 422
  `deck_empty` only when none has. `songIds` (≤ 50, e.g. a summary's misses) drills those songs instead of the deck;
  length defaults to their count, unknown ids are skipped.
- `GET  /api/train/sessions/{id}` → `{id, index, total, card, stats, now}` (resume; `card` null when done; `stats` =
  `{correct, answered, streak, bestStreak, learning, xp}`; 404 `unknown_session`, also for another profile's)
- `now` (server unix ms) in create, resume and answer responses, for the client's clock offset.
- Training requests should carry `X-Timezone: <IANA zone>` (e.g. `Europe/Zurich`); the daily streak counts days in
  that zone, remembered on the profile (no or an invalid header: the stored zone, else UTC).
- Create tries at most 3 × length songs for previews (4 at a time).
- card = `{ id, retry, proposedYear?, clip: {previewUrl, from, length} }` (`proposedYear` in years mode only)
- `POST /api/train/sessions/{id}/answer` `{cardId, choice? | year?}` → `{correct, tier: "easy"|"clear"|"tight"|"exact",
  xp, song, next: card|null, nextError: null|"previews_unavailable"|"internal", now}`. The answer is saved before
  the next card is readied: when that fails, `next` is null with `nextError` set — GET the session to retry. 409 `wrong_card` (not the current card), 409 `conflict` (answered twice at once).
  Pin mode: right within ±1 year; `tier` is how far the pin was. XP for a right answer: 30 exact, 20 within 2
  years (years mode: of the proposed year; pin: ±1), else 10. A first miss comes back 3 cards later with another year
  (`retry: true`, `total` grows).
- `GET  /api/train/sessions/{id}/summary` → `{correct, total, xp, bestStreak, byDecade: [{decade, ok, n}], misses:
  [{song, proposedYear, choice}]}` (`choice` the string in years mode, the pinned year (number) in pin mode, where
  `proposedYear` is null)
- `GET  /api/train/stats` → `{award: "bronze"|"silver"|"gold"|"platinum"|"diamond", xp, nextAt, mastered, learning,
  new, streakDays, decades: [{decade, accuracy}]}`. Awards at 0 / 250 / 750 / 1500 / 3000 XP; `nextAt` is the next
  one's XP, null at diamond. Mastered = three first-try hits in a row, each when the song was due (rests of 1, 3, 7,
  16, 35 days); a hit before it is due moves nothing; a miss sets a song back to learning, due at once. `streakDays` = consecutive days with an answer, 0 once a day was skipped. `accuracy` 0–1 over all answers.
