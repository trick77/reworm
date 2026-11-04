// An in-memory server behind the Api interface: rooms with the party rules (D11/D14/D15), training
// sessions, profiles, devices and pairing. Tests drive screens against it; the dev server's ?fake=1 runs
// the app on it (window.__fake poses any state). One FakeWorld is the server; each FakeApi is a device.
import {
  ApiError,
  type Api,
  type HostAction,
  type RoomHandlers,
} from "../api/client";
import type {
  Card,
  Choice,
  Clip,
  Decks,
  Device,
  Me,
  PairKind,
  Points,
  Profile,
  RoomSettings,
  RoomState,
  Song,
  Tier,
  TrainAnswer,
  TrainSettings,
  TrainStarted,
  TrainStats,
  TrainSummary,
} from "../api/types";

const AUDIO = "https://audio-ssl.itunes.apple.com/itunes-assets/";
const ART = "https://is1-ssl.mzstatic.com/image/thumb/";
const PREVIEWS = [
  "AudioPreview211/v4/0b/23/82/0b238289-52e2-2107-5531-181836267ed2/mzaf_17722571351632165824.plus.aac.ep.m4a",
  "AudioPreview211/v4/9a/c1/a4/9ac1a457-6b2d-91f2-a74a-5e2ee7241d69/mzaf_13444108823547583965.plus.aac.ep.m4a",
  "AudioPreview211/v4/59/4b/88/594b880c-4744-0510-775c-73d8b06567e8/mzaf_14770892373808610282.plus.aac.ep.m4a",
  "AudioPreview221/v4/44/10/de/4410de9b-4b10-c763-baf8-c4911c032248/mzaf_2164995177730538281.plus.aac.ep.m4a",
].map((p) => AUDIO + p);

type FakeSong = Omit<Song, "coverUrl"> & { preview: string; art?: string };

export const FAKE_SONGS: FakeSong[] = [
  {
    id: "s-prayer",
    title: "Like a Prayer",
    artist: "Madonna",
    year: 1989,
    preview: PREVIEWS[0],
    art:
      ART +
      "Music114/v4/20/3c/f5/203cf53d-689e-528f-29d7-ba33758254aa/mzi.rotbotfl.jpg/300x300bb.jpg",
  },
  {
    id: "s-queen",
    title: "Dancing Queen",
    artist: "ABBA",
    year: 1976,
    preview: PREVIEWS[1],
    art:
      ART +
      "Music112/v4/dc/f7/90/dcf790c3-7c63-f7b3-e57b-5a8747a8ebbe/14UMGIM11148.rgb.jpg/300x300bb.jpg",
  },
  {
    id: "s-super",
    title: "Superstition",
    artist: "Stevie Wonder",
    year: 1972,
    preview: PREVIEWS[2],
    art:
      ART +
      "Music114/v4/62/61/61/626161c0-f4d7-e6ff-8586-768340ef278f/00602537002382.rgb.jpg/300x300bb.jpg",
  },
  {
    id: "s-rhapsody",
    title: "Bohemian Rhapsody",
    artist: "Queen",
    year: 1975,
    preview: PREVIEWS[3],
    art:
      ART +
      "Music221/v4/62/2e/97/622e97c4-d620-2ae1-2a47-a32e16a01492/602567988847.jpg/300x300bb.jpg",
  },
  {
    id: "s-teen",
    title: "Smells Like Teen Spirit",
    artist: "Nirvana",
    year: 1991,
    preview: PREVIEWS[0],
  },
  {
    id: "s-heyya",
    title: "Hey Ya!",
    artist: "OutKast",
    year: 2003,
    preview: PREVIEWS[1],
  },
  {
    id: "s-lights",
    title: "Blinding Lights",
    artist: "The Weeknd",
    year: 2019,
    preview: PREVIEWS[2],
  },
  {
    id: "s-twist",
    title: "The Twist",
    artist: "Chubby Checker",
    year: 1960,
    preview: PREVIEWS[3],
  },
  {
    id: "s-luft",
    title: "99 Luftballons",
    artist: "Nena",
    year: 1983,
    preview: PREVIEWS[0],
  },
  {
    id: "s-lucky",
    title: "Get Lucky",
    artist: "Daft Punk",
    year: 2013,
    preview: PREVIEWS[1],
  },
];

export const FAKE_DECKS: Decks = {
  sets: [
    ["essentials", "Essentials", 308, 1908, 2021],
    ["megamix", "Mega Mix", 500, 1950, 2024],
    ["rock", "Rock Anthems", 308, 1954, 2024],
    ["schlager", "Schlager Fest", 308, 1961, 2022],
    ["sunshine", "Sunshine", 308, 1958, 2023],
    ["cheesy", "Cheesy Classics", 308, 1960, 2023],
    ["party", "Party Starters", 308, 1949, 2024],
    ["crowd", "Crowd Pleasers", 226, 1961, 2023],
    ["screen", "Screen Hits", 154, 1939, 2023],
    ["xmas", "Christmas Hits", 154, 1941, 2023],
    ["radio", "Radio Gold", 154, 1955, 2023],
  ].map(([id, name, count, from, to]) => ({
    id: id as string,
    name: name as string,
    count: count as number,
    from: from as number,
    to: to as number,
  })),
  categories: [
    ["smash", "Smash Hits", 500],
    ["ones", "Number ones", 320],
    ["ever", "Evergreens", 40],
    ["xmascat", "Christmas", 120],
    ["movie", "Movie & TV", 140],
    ["summer", "Summer", 200],
    ["partycat", "Party", 180],
    ["swiss", "Swiss charts", 200],
    ["esc", "Eurovision", 120],
  ].map(([id, name, count]) => ({
    id: id as string,
    name: name as string,
    count: count as number,
  })),
  genres: [
    ["general", "General", 4200],
    ["pop", "Pop", 1400],
    ["rockg", "Rock", 900],
    ["schlagerg", "Schlager & Deutschpop", 380],
    ["italo", "Italo-Schlager", 120],
    ["hiphop", "Hip-Hop / Rap", 300],
    ["soul", "R&B / Soul", 420],
    ["dance", "Dance / Electronic", 360],
    ["chanson", "French chanson", 90],
  ].map(([id, name, count]) => ({
    id: id as string,
    name: name as string,
    count: count as number,
  })),
};

const ROUND_MS = 15_000;
const ANSWER_MS = 10_000;
// proposed-year offsets the fake cycles through: a mix of the four tiers (exact, tight, clear, easy)
const OFFSETS = [6, -2, 0, 14, -5, 3, -11, 1, 0, -8];

const fail = (status: number, code: string, message = code) =>
  Promise.reject(new ApiError(status, code, message));
const clone = <T>(v: T): T => structuredClone(v);
const ok = <T>(v: T): Promise<T> => Promise.resolve(clone(v));

type FakeDevice = {
  id: string;
  label: string;
  lastSeen: number;
  profileId: string | null;
  players: Record<string, string>; // room code → player id
};

type FakeRoom = {
  state: RoomState;
  hostDevice: string;
  songs: FakeSong[];
  song: FakeSong | null;
  pausedFrom: string | null;
  pausedAt: number;
  kicked: Set<string>;
  subs: Set<RoomHandlers>;
  timer?: ReturnType<typeof setTimeout>;
};

type FakeSession = {
  id: string;
  settings: TrainSettings;
  queue: { card: Card; song: FakeSong }[];
  index: number;
  results: {
    song: FakeSong;
    proposedYear?: number;
    choice: Choice | number;
    ok: boolean;
  }[];
  streak: number;
  best: number;
  xp: number;
};

let seq = 0;
const nextId = (p: string) => `${p}${++seq}`;

function clipOf(s: FakeSong, length: number): Clip {
  return { previewUrl: s.preview, from: 0, length };
}

export class FakeWorld {
  profiles = new Map<string, Profile & { admin: boolean }>();
  devices = new Map<string, FakeDevice>();
  pairs = new Map<
    string,
    { kind: PairKind; profileId: string; expiresAt: number }
  >();
  setupToken: string | null = "fake-setup";
  rooms = new Map<string, FakeRoom>();
  sessions = new Map<string, FakeSession>();
  stats: TrainStats = {
    award: "gold",
    xp: 1380,
    nextAt: 1500,
    mastered: 124,
    learning: 76,
    new: 200,
    streakDays: 12,
    decades: [
      [1960, 0.81],
      [1970, 0.52],
      [1980, 0.74],
      [1990, 0.68],
      [2000, 0.61],
      [2010, 0.79],
      [2020, 0.88],
    ].map(([decade, accuracy]) => ({ decade, accuracy })),
  };
  songs = FAKE_SONGS;
  // timers: rounds end by themselves (15 s, 10 s to say it); off in most tests
  timers: boolean;
  // latency: ms every call takes (the dev server's feel); 0 in tests
  latency: number;
  codes: string[] = [];

  constructor(
    opts: { timers?: boolean; latency?: number; seed?: boolean } = {},
  ) {
    this.timers = opts.timers ?? false;
    this.latency = opts.latency ?? 0;
    if (opts.seed !== false) {
      for (const [id, name, avatar, admin] of [
        ["p-jan", "Jan", "vinyl", true],
        ["p-mia", "Mia", "mic", false],
        ["p-luca", "Luca", "guitar", false],
        ["p-sarah", "Sarah", "disco", false],
      ] as const)
        this.profiles.set(id, { id, name, avatar, admin });
      this.devices.set("d-ipad", {
        id: "d-ipad",
        label: "iPad · Home-screen app",
        lastSeen: Date.now() - 2 * 3600_000,
        profileId: "p-jan",
        players: {},
      });
      this.devices.set("d-mac", {
        id: "d-mac",
        label: "MacBook · Safari",
        lastSeen: Date.now() - 3 * 86400_000,
        profileId: "p-jan",
        players: {},
      });
    }
  }

  // device is a new browser: unpaired unless a profile is given.
  device(profileId: string | null = null, label = "iPhone · Safari"): FakeApi {
    const id = nextId("d-");
    this.devices.set(id, {
      id,
      label,
      lastSeen: Date.now(),
      profileId,
      players: {},
    });
    return new FakeApi(this, id);
  }

  // ---- posing and bots (tests, the dev ?fake=1 console) ----

  room(code: string): FakeRoom {
    const r = this.rooms.get(code);
    if (!r) throw new Error(`no room ${code}`);
    return r;
  }

  // addPlayer joins a bot (or a test's other phone) and returns its player id.
  addPlayer(code: string, name: string, avatar: string): string {
    const r = this.room(code);
    const id = nextId("pl-");
    r.state.players.push({
      id,
      name,
      avatar,
      score: 0,
      connected: true,
      // joining mid-round sits that round out
      state: r.state.round && !r.state.round.result ? "late" : "thinking",
    });
    this.emit(code);
    return id;
  }

  // demo opens a room hosted by another device with the mock's six players; returns their ids.
  demo(code: string, mode: RoomSettings["mode"] = "years"): string[] {
    const host = this.device("p-jan", "Stage");
    this.codes.unshift(code);
    this.createRoom(host.deviceId, {
      mode,
      deck: {
        sets: ["essentials"],
        categories: [],
        genres: [],
        from: 1900,
        to: 2025,
      },
      toWin: 10,
      clip: { length: 20, start: "chorus" },
      difficulty: "medium",
    });
    return [
      ["Jan", "vinyl"],
      ["Mia", "mic"],
      ["Luca", "guitar"],
      ["Sarah", "disco"],
      ["Tom", "drum"],
      ["Lea", "note"],
    ].map(([n, a]) => this.addPlayer(code, n, a));
  }

  // seat makes `api`'s device the given player (the pad of a demo room).
  seat(api: FakeApi, code: string, playerId: string) {
    this.devices.get(api.deviceId)!.players[code] = playerId;
  }

  // set merges a patch into the room state (round merged too) and pushes it.
  set(code: string, patch: Partial<RoomState>) {
    const r = this.room(code);
    const round =
      patch.round && r.state.round
        ? { ...r.state.round, ...patch.round }
        : patch.round === undefined
          ? r.state.round
          : patch.round;
    r.state = { ...r.state, ...patch, round };
    this.emit(code);
  }

  song(code: string): FakeSong | null {
    return this.room(code).song;
  }

  // correct is the right years-mode answer for the round now.
  correct(code: string): Choice {
    const r = this.room(code);
    const y = r.song!.year;
    const p = r.state.round!.proposedYear!;
    return y === p ? "exact" : y < p ? "earlier" : "later";
  }

  // close drops a room: every subscriber hears the 404 a reconnect would get.
  close(code: string) {
    const r = this.room(code);
    this.rooms.delete(code);
    for (const h of r.subs)
      h.onError?.(new ApiError(404, "no_room", "room not found"));
  }

  emit(code: string) {
    const r = this.rooms.get(code);
    if (!r) return;
    r.state.seq++;
    r.state.now = Date.now();
    for (const h of r.subs) h.onState(clone(r.state));
  }

  // ---- the room engine ----

  createRoom(deviceId: string, s: RoomSettings): string {
    const dev = this.devices.get(deviceId)!;
    let code = this.codes.shift();
    while (!code || this.rooms.has(code)) {
      const A = "ABCDEFGHJKMNPQRSTUVWXYZ23456789";
      code = Array.from(
        { length: 4 },
        () => A[Math.floor(Math.random() * A.length)],
      ).join("");
    }
    const from = s.deck.from;
    const to = s.deck.to;
    const songs = this.songs.filter((x) => x.year >= from && x.year <= to);
    this.rooms.set(code, {
      hostDevice: deviceId,
      songs: songs.length ? songs : this.songs,
      song: null,
      pausedFrom: null,
      pausedAt: 0,
      kicked: new Set(),
      subs: new Set(),
      state: {
        code,
        mode: s.mode,
        phase: "lobby",
        hostId: dev.profileId!,
        toWin: s.toWin,
        settings: { deck: s.deck, clip: s.clip, difficulty: s.difficulty },
        players: [],
        round: null,
        nextClip: null,
        seq: 1,
        now: Date.now(),
      },
    });
    return code;
  }

  private schedule(code: string, at: number, f: () => void) {
    const r = this.room(code);
    clearTimeout(r.timer);
    if (!this.timers) return;
    r.timer = setTimeout(f, Math.max(0, at - Date.now()));
  }

  newRound(code: string) {
    const r = this.room(code);
    const st = r.state;
    const n = (st.round?.n ?? 0) + 1;
    const song = r.songs[(n - 1) % r.songs.length];
    const next = r.songs[n % r.songs.length];
    r.song = song;
    const now = Date.now();
    const len = st.settings.clip.length;
    st.players.forEach((p) => (p.state = "thinking"));
    st.round = {
      n,
      startedAt: now,
      endsAt: now + ROUND_MS,
      proposedYear:
        st.mode === "years"
          ? song.year + OFFSETS[(n - 1) % OFFSETS.length]
          : undefined,
      clip: clipOf(song, len),
      buzz: null,
      revealVotes: [],
      judgeEndsAt: null,
      judgeVotes: {},
      result: null,
    };
    st.nextClip = clipOf(next, len);
    st.phase = "playing";
    this.schedule(code, st.round.endsAt, () => this.endRound(code, null, 0));
  }

  endRound(code: string, winnerId: string | null, points: number) {
    const r = this.room(code);
    const st = r.state;
    if (!st.round || !r.song) return;
    clearTimeout(r.timer);
    const w = st.players.find((p) => p.id === winnerId);
    if (w) w.score += points;
    st.round.result = {
      song: {
        id: r.song.id,
        artist: r.song.artist,
        title: r.song.title,
        year: r.song.year,
        coverUrl: r.song.art ?? null,
      },
      winnerId: w ? w.id : null,
      points: w ? points : 0,
      photoFinish: st.round.result?.photoFinish ?? [],
      winnerMs: w ? Date.now() - st.round.startedAt : null,
    };
    st.phase = w && w.score >= st.toWin ? "over" : "result";
    this.emit(code);
  }

  answerAs(code: string, playerId: string, round: number, choice: Choice) {
    const r = this.room(code);
    const st = r.state;
    const p = st.players.find((x) => x.id === playerId);
    if (
      !p ||
      st.mode !== "years" ||
      st.phase !== "playing" ||
      st.round?.n !== round
    )
      return fail(409, "not_now");
    if (p.state !== "thinking") return fail(409, "already_answered");
    const y = r.song!.year;
    const shown = st.round.proposedYear!;
    const right =
      choice === "exact"
        ? y === shown
        : choice === "earlier"
          ? y < shown
          : y > shown;
    if (!right) {
      p.state = "out";
      if (st.players.every((x) => x.state === "out"))
        this.endRound(code, null, 0);
      else this.emit(code);
    } else {
      p.state = "answered";
      this.endRound(code, p.id, choice === "exact" ? 2 : 1);
    }
    return Promise.resolve();
  }

  buzzAs(code: string, playerId: string, round: number) {
    const st = this.room(code).state;
    const p = st.players.find((x) => x.id === playerId);
    if (
      !p ||
      st.mode !== "buzzer" ||
      st.phase !== "playing" ||
      st.round?.n !== round
    )
      return fail(409, "not_now");
    const now = Date.now();
    p.state = "buzzed";
    st.round.buzz = { playerId, at: now, answerEndsAt: now + ANSWER_MS };
    st.phase = "buzzed";
    clearTimeout(this.room(code).timer);
    this.emit(code);
    return Promise.resolve();
  }

  revealAs(code: string, playerId: string, round: number) {
    const r = this.room(code);
    const st = r.state;
    const rd = st.round;
    if (st.phase !== "buzzed" || rd?.n !== round || !rd.buzz)
      return fail(409, "not_now");
    if (!rd.revealVotes.includes(playerId)) rd.revealVotes.push(playerId);
    const buzzer = rd.buzz.playerId;
    if (
      rd.revealVotes.includes(buzzer) &&
      rd.revealVotes.some((v) => v !== buzzer)
    ) {
      st.phase = "judging";
      rd.judgeEndsAt = Date.now() + 20_000;
      const s = r.song!;
      // judging needs the song in view: the fake hands it out with the result still open
      rd.result = {
        song: {
          id: s.id,
          artist: s.artist,
          title: s.title,
          year: s.year,
          coverUrl: s.art ?? null,
        },
        winnerId: null,
        points: 0,
        photoFinish: [],
        winnerMs: null,
      };
    }
    this.emit(code);
    return Promise.resolve();
  }

  judgeAs(code: string, playerId: string, round: number, points: Points) {
    const st = this.room(code).state;
    const rd = st.round;
    if (st.phase !== "judging" || rd?.n !== round || !rd.buzz)
      return fail(409, "not_now");
    rd.judgeVotes[playerId] = points;
    const buzzer = rd.buzz.playerId;
    const mine = rd.judgeVotes[buzzer];
    const others = Object.entries(rd.judgeVotes).filter(
      ([id]) => id !== buzzer,
    );
    let decided: number | null = null;
    if (mine != null && others.some(([, v]) => v === mine)) decided = mine;
    else if (Object.keys(rd.judgeVotes).length >= st.players.length) {
      // the lower median of all votes
      const v = Object.values(rd.judgeVotes).sort((a, b) => a - b);
      decided = v[Math.floor((v.length - 1) / 2)];
    }
    if (decided == null) {
      this.emit(code);
      return Promise.resolve();
    }
    this.endRound(code, decided > 0 ? buzzer : null, decided);
    return Promise.resolve();
  }

  hostAction(code: string, action: HostAction) {
    const r = this.room(code);
    const st = r.state;
    switch (action) {
      case "start":
        if (st.phase !== "lobby") return fail(409, "not_in_lobby");
        this.newRound(code);
        break;
      case "next":
        if (st.phase !== "result") return fail(409, "not_now");
        this.newRound(code);
        break;
      case "pause":
        if (
          st.phase === "paused" ||
          st.phase === "lobby" ||
          st.phase === "over"
        )
          return fail(409, "not_now");
        clearTimeout(r.timer);
        r.pausedFrom = st.phase;
        r.pausedAt = Date.now();
        st.phase = "paused";
        break;
      case "resume": {
        if (st.phase !== "paused") return fail(409, "not_paused");
        const gap = Date.now() - r.pausedAt;
        st.phase = (r.pausedFrom ?? "playing") as RoomState["phase"];
        if (st.round && st.phase === "playing") {
          st.round.endsAt += gap;
          this.schedule(code, st.round.endsAt, () =>
            this.endRound(code, null, 0),
          );
        }
        if (st.round?.buzz) st.round.buzz.answerEndsAt += gap;
        break;
      }
      case "rematch":
        st.players.forEach((p) => (p.score = 0));
        st.round = null;
        this.newRound(code);
        break;
    }
    this.emit(code);
    return Promise.resolve();
  }

  // ---- training ----

  startTraining(s: TrainSettings): TrainStarted {
    const id = nextId("t-");
    const songs = s.songIds
      ? this.songs.filter((x) => s.songIds!.includes(x.id))
      : this.songs.filter((x) => x.year >= s.deck.from && x.year <= s.deck.to);
    const pool = (songs.length ? songs : this.songs).slice(0, s.length);
    const queue = pool.map((song, i) => ({
      song,
      card: {
        id: nextId("c-"),
        retry: false,
        proposedYear:
          s.mode === "years"
            ? song.year + OFFSETS[i % OFFSETS.length]
            : undefined,
        clip: clipOf(song, s.clip.length),
      },
    }));
    this.sessions.set(id, {
      id,
      settings: s,
      queue,
      index: 0,
      results: [],
      streak: 0,
      best: 0,
      xp: 0,
    });
    return { id, card: queue[0].card, now: Date.now() };
  }

  trainAnswer(
    id: string,
    b: { cardId: string; choice?: Choice; year?: number },
  ): TrainAnswer {
    const ses = this.sessions.get(id)!;
    const cur = ses.queue[ses.index];
    if (!cur || cur.card.id !== b.cardId)
      throw new ApiError(409, "stale_card", "stale card");
    const y = cur.song.year;
    const shown = cur.card.proposedYear;
    let correct: boolean;
    let diff: number;
    if (ses.settings.mode === "pin") {
      diff = Math.abs((b.year ?? 0) - y);
      correct = diff <= 1;
    } else {
      diff = Math.abs(shown! - y);
      const c = b.choice;
      correct =
        c === "exact" ? y === shown : c === "earlier" ? y < shown! : y > shown!;
    }
    const tier: Tier =
      diff === 0 ? "exact" : diff <= 3 ? "tight" : diff <= 9 ? "clear" : "easy";
    const xp = correct ? (diff === 0 ? 30 : diff <= 2 ? 20 : 10) : 0;
    ses.streak = correct ? ses.streak + 1 : 0;
    ses.best = Math.max(ses.best, ses.streak);
    ses.xp += xp;
    this.stats.xp += xp;
    ses.results.push({
      song: cur.song,
      proposedYear: shown,
      choice: ses.settings.mode === "pin" ? b.year! : b.choice!,
      ok: correct,
    });
    if (!correct && !cur.card.retry) {
      const at = Math.min(ses.index + 4, ses.queue.length);
      ses.queue.splice(at, 0, {
        song: cur.song,
        card: {
          id: nextId("c-"),
          retry: true,
          proposedYear:
            ses.settings.mode === "years"
              ? y + (shown! > y ? -3 : 3)
              : undefined,
          clip: cur.card.clip,
        },
      });
    }
    ses.index++;
    const s = cur.song;
    return {
      correct,
      tier,
      xp,
      song: {
        id: s.id,
        artist: s.artist,
        title: s.title,
        year: s.year,
        coverUrl: s.art ?? null,
      },
      next: ses.queue[ses.index]?.card ?? null,
      nextError: null,
      now: Date.now(),
    };
  }

  trainSummary(id: string): TrainSummary {
    const ses = this.sessions.get(id)!;
    const by = new Map<number, { ok: number; n: number }>();
    for (const r of ses.results) {
      const d = Math.floor(r.song.year / 10) * 10;
      const v = by.get(d) ?? { ok: 0, n: 0 };
      v.n++;
      if (r.ok) v.ok++;
      by.set(d, v);
    }
    const plain = (s: FakeSong): Song => ({
      id: s.id,
      artist: s.artist,
      title: s.title,
      year: s.year,
      coverUrl: s.art ?? null,
    });
    return {
      correct: ses.results.filter((r) => r.ok).length,
      total: ses.results.length,
      xp: ses.xp,
      bestStreak: ses.best,
      byDecade: [...by.entries()]
        .sort(([a], [b]) => a - b)
        .map(([decade, v]) => ({ decade, ...v })),
      misses: ses.results
        .filter((r) => !r.ok)
        .map((r) => ({
          song: plain(r.song),
          proposedYear: r.proposedYear ?? null,
          choice: r.choice,
        })),
    };
  }
}

// FakeApi is one device talking to the FakeWorld.
export class FakeApi implements Api {
  readonly world: FakeWorld;
  readonly deviceId: string;
  calls: string[] = [];
  constructor(world: FakeWorld, deviceId: string) {
    this.world = world;
    this.deviceId = deviceId;
  }

  private get dev(): FakeDevice {
    return this.world.devices.get(this.deviceId)!;
  }

  private async wait() {
    if (this.world.latency)
      await new Promise((r) => setTimeout(r, this.world.latency));
  }

  private async log<T>(name: string, f: () => Promise<T>): Promise<T> {
    this.calls.push(name);
    await this.wait();
    return f();
  }

  private profile() {
    const id = this.dev?.profileId;
    return id ? this.world.profiles.get(id) : undefined;
  }

  private paired<T>(
    name: string,
    f: (p: Profile & { admin: boolean }) => Promise<T>,
  ) {
    return this.log(name, () => {
      const p = this.profile();
      return p ? f(p) : fail(401, "unpaired", "this device is not paired");
    });
  }

  // playerId is this device's player in a room, if it joined.
  playerId(code: string): string | undefined {
    return this.dev.players[code];
  }

  version = () => this.log("version", () => ok("0.0.0-fake"));
  me = () =>
    this.paired<Me>("me", (p) =>
      ok({ profile: p, device: { id: this.deviceId, label: this.dev.label } }),
    );
  setup = (b: { token: string; name: string; avatar: string }) =>
    this.log("setup", () => {
      const w = this.world;
      if (w.setupToken == null)
        return fail(410, "used", "setup link already used");
      if (b.token !== w.setupToken) return fail(403, "bad_token");
      w.setupToken = null;
      const id = nextId("p-");
      w.profiles.set(id, { id, name: b.name, avatar: b.avatar, admin: true });
      this.dev.profileId = id;
      return Promise.resolve();
    });
  profiles = () =>
    this.paired("profiles", () =>
      ok(
        [...this.world.profiles.values()].map(({ id, name, avatar }) => ({
          id,
          name,
          avatar,
        })),
      ),
    );
  switchProfile = (profileId: string) =>
    this.paired("switch", (me) => {
      const to = this.world.profiles.get(profileId);
      if (!to) return fail(404, "no_profile");
      if (to.admin && me.id !== to.id)
        return fail(
          403,
          "admin_profile",
          "pair this device from an admin device instead",
        );
      this.dev.profileId = profileId;
      return Promise.resolve();
    });
  updateMe = (b: { name?: string; avatar?: string }) =>
    this.paired("updateMe", (p) => {
      if (b.name != null) p.name = b.name;
      if (b.avatar != null) p.avatar = b.avatar;
      return Promise.resolve();
    });
  devices = () =>
    this.paired("devices", (p) =>
      ok(
        [...this.world.devices.values()]
          .filter((d) => d.profileId === p.id)
          .map((d): Device => ({
            id: d.id,
            label: d.label,
            lastSeen: d.lastSeen,
            current: d.id === this.deviceId,
          })),
      ),
    );
  removeDevice = (id: string) =>
    this.paired("removeDevice", () => {
      this.world.devices.delete(id);
      return Promise.resolve();
    });
  pair = (kind: PairKind) =>
    this.paired("pair", (p) => {
      const code = Math.random()
        .toString(36)
        .slice(2, 8)
        .toUpperCase()
        .padEnd(6, "X");
      const expiresAt = Date.now() + 5 * 60_000;
      this.world.pairs.set(code, { kind, profileId: p.id, expiresAt });
      return ok({ code, url: `${location.origin}/pair/${code}`, expiresAt });
    });
  claimPair = (b: { code: string; name?: string; avatar?: string }) =>
    this.log("claim", () => {
      const pc = this.world.pairs.get(b.code.toUpperCase());
      if (!pc || pc.expiresAt < Date.now())
        return fail(404, "bad_code", "code unknown or expired");
      if (pc.kind === "invite") {
        if (!b.name || !b.avatar)
          return fail(400, "name_required", "name and avatar needed");
        const id = nextId("p-");
        this.world.profiles.set(id, {
          id,
          name: b.name,
          avatar: b.avatar,
          admin: false,
        });
        this.dev.profileId = id;
      } else {
        this.dev.profileId = pc.profileId;
      }
      this.world.pairs.delete(b.code.toUpperCase());
      return Promise.resolve();
    });
  decks = () => this.log("decks", () => ok(FAKE_DECKS));
  coverUrl = (songId: string) =>
    this.world.songs.find((s) => s.id === songId)?.art ??
    `/api/covers/${songId}`;

  createRoom = (s: RoomSettings) =>
    this.paired("createRoom", () =>
      ok({ code: this.world.createRoom(this.deviceId, s) }),
    );
  room = (code: string) =>
    this.log("room", () => {
      const r = this.world.rooms.get(code);
      return r ? ok(r.state) : fail(404, "no_room", "room not found");
    });
  subscribeRoom(code: string, h: RoomHandlers) {
    let live = true;
    void this.room(code).then(
      (s) => {
        if (!live) return;
        h.onState(s);
        h.onLive?.(true);
        this.world.rooms.get(code)?.subs.add(h);
      },
      (e: ApiError) => live && h.onError?.(e),
    );
    return () => {
      live = false;
      this.world.rooms.get(code)?.subs.delete(h);
    };
  }
  joinRoom = (code: string, b: { name: string; avatar: string }) =>
    this.log("join", () => {
      const r = this.world.rooms.get(code);
      if (!r) return fail(404, "no_room", "room not found");
      const had = this.dev.players[code];
      if (had && r.kicked.has(had)) return fail(403, "kicked", "kicked");
      if (had && r.state.players.some((p) => p.id === had))
        return ok({ playerId: had });
      if (r.state.players.length >= 8)
        return fail(409, "room_full", "room is full");
      const p = this.profile();
      const id = this.world.addPlayer(
        code,
        p?.name ?? b.name,
        p?.avatar ?? b.avatar,
      );
      this.dev.players[code] = id;
      return ok({ playerId: id });
    });
  roomMe = (code: string) =>
    this.log("roomMe", () => {
      const r = this.world.rooms.get(code);
      const id = this.dev.players[code];
      if (r && id && r.kicked.has(id))
        return fail(403, "kicked", "removed from this game");
      return ok(
        r && id && r.state.players.some((p) => p.id === id) ? id : null,
      );
    });
  private hostOnly(name: string, code: string, f: () => Promise<void>) {
    return this.log(name, () => {
      const r = this.world.rooms.get(code);
      if (!r) return fail(404, "no_room");
      if (r.hostDevice !== this.deviceId) return fail(403, "not_host");
      return f();
    });
  }
  host = (code: string, action: HostAction) =>
    this.hostOnly(action, code, () => this.world.hostAction(code, action));
  kick = (code: string, playerId: string) =>
    this.hostOnly("kick", code, () => {
      const st = this.world.room(code).state;
      st.players = st.players.filter((p) => p.id !== playerId);
      this.world.room(code).kicked.add(playerId);
      this.world.emit(code);
      return Promise.resolve();
    });
  roomSettings = (code: string, s: RoomSettings) =>
    this.hostOnly("settings", code, () => {
      const r = this.world.room(code);
      if (r.state.phase !== "lobby") return fail(409, "not_in_lobby");
      r.state.mode = s.mode;
      r.state.toWin = s.toWin;
      r.state.settings = {
        deck: s.deck,
        clip: s.clip,
        difficulty: s.difficulty,
      };
      this.world.emit(code);
      return Promise.resolve();
    });
  private me_(code: string) {
    return this.dev.players[code] ?? "";
  }
  answer = (code: string, round: number, choice: Choice) =>
    this.log("answer", () =>
      this.world.answerAs(code, this.me_(code), round, choice),
    );
  buzz = (code: string, round: number) =>
    this.log("buzz", () => this.world.buzzAs(code, this.me_(code), round));
  reveal = (code: string, round: number) =>
    this.log("reveal", () => this.world.revealAs(code, this.me_(code), round));
  judge = (code: string, round: number, points: Points) =>
    this.log("judge", () =>
      this.world.judgeAs(code, this.me_(code), round, points),
    );

  startTraining = (s: TrainSettings) =>
    this.paired("startTraining", () => ok(this.world.startTraining(s)));
  trainSession = (id: string) =>
    this.paired("trainSession", () => {
      const ses = this.world.sessions.get(id);
      if (!ses) return fail(404, "no_session");
      return ok({
        id,
        index: ses.index,
        total: ses.queue.length,
        card: ses.queue[ses.index]?.card ?? null,
        stats: {
          correct: ses.results.filter((r) => r.ok).length,
          answered: ses.results.length,
          streak: ses.streak,
          bestStreak: ses.best,
          learning: ses.queue.slice(ses.index).filter((q) => q.card.retry)
            .length,
          xp: ses.xp,
        },
        now: Date.now(),
      });
    });
  trainAnswer = (
    id: string,
    b: { cardId: string; choice?: Choice; year?: number },
  ) =>
    this.paired("trainAnswer", () => {
      try {
        return ok(this.world.trainAnswer(id, b));
      } catch (e) {
        return Promise.reject(e);
      }
    });
  trainSummary = (id: string) =>
    this.paired("trainSummary", () => ok(this.world.trainSummary(id)));
  trainStats = () => this.paired("trainStats", () => ok(this.world.stats));
}
