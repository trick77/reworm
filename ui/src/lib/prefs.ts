// Small things this device remembers (localStorage): last room and training settings, the player id per
// joined room, the room this device hosts. Storage may be blocked (private mode): then nothing sticks.
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
export const YEAR_MAX = 2025;

export const allYears = (deck: Partial<Deck> = {}): Deck => ({
  sets: [],
  categories: [],
  genres: [],
  from: YEAR_MIN,
  to: YEAR_MAX,
  ...deck,
});

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

export const playerKey = (code: string) => `player.${code}`;
