// The HTTP API contract (v1), shared with the backend. All times are server unix milliseconds.

export type Profile = { id: string; name: string; avatar: string };
export type Me = {
  profile: Profile & { admin: boolean };
  device: { id: string; label: string };
};
export type Device = {
  id: string;
  label: string;
  lastSeen: number;
  current: boolean;
};
export type PairKind = "device" | "invite";
export type PairCode = { code: string; url: string; expiresAt: number };

export type DeckSet = {
  id: string;
  name: string;
  count: number;
  from: number;
  to: number;
};
export type DeckGroup = { id: string; name: string; count: number };
export type Decks = {
  sets: DeckSet[];
  categories: DeckGroup[];
  genres: DeckGroup[];
};
// Deck is the selector sent with a room or a training session.
export type Deck = {
  sets: string[];
  categories: string[];
  genres: string[];
  from: number;
  to: number;
};

export type Difficulty = "easy" | "medium" | "hard";
export type ClipStart = "chorus" | "random";
export type ClipSettings = { length: number; start: ClipStart };
export type Clip = { previewUrl: string; from: number; length: number };
export type Choice = "earlier" | "exact" | "later";
export type Song = {
  id: string;
  artist: string;
  title: string;
  year: number;
  coverUrl: string | null; // null: no cover, the UI draws a record
};

// ---- party rooms ----
export type RoomMode = "years" | "buzzer";
export type RoomSettings = {
  mode: RoomMode;
  deck: Deck;
  toWin: number;
  clip: ClipSettings;
  difficulty: Difficulty;
};
export type Phase =
  "lobby" | "playing" | "buzzed" | "judging" | "result" | "paused" | "over";
// out: answered wrong this round (years); late: joined mid-round, plays from the next one
export type PlayerState = "thinking" | "out" | "late" | "buzzed" | "answered";
export type RoomPlayer = {
  id: string;
  name: string;
  avatar: string;
  score: number;
  connected: boolean;
  state: PlayerState;
};
export type Buzz = { playerId: string; at: number; answerEndsAt: number };
export type Points = 0 | 1 | 2;
export type RoundResult = {
  song: Song;
  winnerId: string | null;
  points: number;
  photoFinish: { playerId: string; deltaMs: number }[];
  winnerMs: number | null; // the winning press after startedAt, pauses excluded; null without a winner
};
export type Round = {
  n: number;
  startedAt: number;
  endsAt: number;
  judgeEndsAt: number | null; // buzzer: the judging window's end, null before the reveal
  proposedYear?: number;
  clip: Clip;
  buzz: Buzz | null;
  revealVotes: string[];
  judgeVotes: Record<string, Points>;
  result: RoundResult | null;
};
export type RoomState = {
  code: string;
  mode: RoomMode;
  phase: Phase;
  // only while paused: the phase the pause froze
  pausedFrom?: "playing" | "buzzed" | "judging";
  hostId: string;
  toWin: number;
  settings: { deck: Deck; clip: ClipSettings; difficulty: Difficulty };
  players: RoomPlayer[];
  round: Round | null;
  nextClip: Clip | null;
  seq: number; // grows with every change of the room: an older state is dropped
  now: number; // server ms when written: the countdowns' clock
};

// ---- training ----
export type TrainMode = "years" | "pin";
export type TrainSettings = {
  deck: Deck;
  mode: TrainMode;
  difficulty: Difficulty;
  clip: ClipSettings;
  length: number;
  songIds?: string[]; // drill exactly these songs (optional)
};
export type Card = {
  id: string;
  retry: boolean;
  proposedYear?: number;
  clip: Clip;
};
export type TrainStatsLine = { ok: number; n: number; decade: number };
export type TrainStarted = { id: string; card: Card; now: number };
export type TrainSession = {
  id: string;
  index: number;
  total: number;
  card: Card | null; // null: the session is done
  stats: {
    correct: number;
    answered: number;
    streak: number;
    bestStreak: number;
    learning: number;
    xp: number;
  };
  now: number;
};
export type Tier = "easy" | "clear" | "tight" | "exact";
export type TrainAnswer = {
  correct: boolean;
  tier: Tier;
  xp: number;
  song: Song;
  next: Card | null;
  // set when next is null because the next card failed to load (the answer is saved): GET the session to retry
  nextError: null | "previews_unavailable" | "internal";
  now: number;
};
export type TrainSummary = {
  correct: number;
  total: number;
  xp: number;
  bestStreak: number;
  byDecade: TrainStatsLine[];
  // proposedYear: null in pin mode, where choice is the pinned year
  misses: {
    song: Song;
    proposedYear: number | null;
    choice: Choice | number;
  }[];
};
export type Award = "bronze" | "silver" | "gold" | "platinum" | "diamond";
export type TrainStats = {
  award: Award;
  xp: number;
  nextAt: number | null; // null at diamond
  mastered: number;
  learning: number;
  new: number;
  streakDays: number;
  decades: { decade: number; accuracy: number }[];
};
