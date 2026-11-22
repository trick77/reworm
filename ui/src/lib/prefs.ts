// Small things this device remembers (localStorage): last room and training settings, the last guest
// name and avatar, the player id per joined room, the room this device hosts. Storage may be blocked (private mode): then nothing sticks.
import type { Deck, RoomSettings, TrainSettings } from "../api/types";

const P = "reworm.";

export function load<T>(key: string): T | null {
  try {
    const v = window.localStorage.getItem(P + key);
    return v == null ? null : (JSON.parse(v) as T);
  } catch {
    return null;
  }
}

export function save(key: string, v: unknown) {
  try {
    if (v == null) window.localStorage.removeItem(P + key);
    else window.localStorage.setItem(P + key, JSON.stringify(v));
  } catch {
    // not saved
  }
}

export const YEAR_MIN = 1900;
// the newest songs are from this year
export const YEAR_MAX = new Date().getFullYear();

export const allYears = (deck: Partial<Deck> = {}): Deck => ({
  sets: [],
  categories: [],
  genres: [],
  from: YEAR_MIN,
  to: YEAR_MAX,
  ...deck,
});

// oneKind keeps one kind of a deck, the one the picker opens on: sets, else categories, else genres. The server
// takes a song in any of them, so a deck saved before the picker kept to one kind played a set's oldies beside
// Gen Alpha.
export const oneKind = (d: Deck): Deck =>
  d.sets.length
    ? { ...d, categories: [], genres: [] }
    : d.categories.length
      ? { ...d, genres: [] }
      : d;

// loadSettings is saved room or training settings with their deck kept to one kind.
export function loadSettings<T extends { deck: Deck }>(
  key: "room" | "train",
): T | null {
  const v = load<T>(key);
  return v?.deck ? { ...v, deck: oneKind(v.deck) } : v;
}

export const defaultRoom = (deck: Deck): RoomSettings => ({
  mode: "years",
  deck,
  toWin: 10,
  clip: { length: 20, start: "chorus" },
  difficulty: "medium",
});

export const defaultTraining = (deck: Deck): TrainSettings => ({
  deck,
  mode: "years",
  difficulty: "medium",
  clip: { length: 20, start: "chorus" },
  length: 10,
});

// playerKey: this browser's player id in a room, used at once on a reload while the room confirms it
export const playerKey = (code: string) => `player.${code}`;
