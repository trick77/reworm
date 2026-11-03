import { player } from "./player";

let unlocked = false;

// unlockAudio runs inside a tap (Start training, Start game, the stage's sound button): it lets the shared
// player play every later clip without one.
export function unlockAudio() {
  player().unlock();
  unlocked = true;
}

// audioUnlocked: a tap has unlocked the player since the page loaded (a reload needs a new one).
export const audioUnlocked = () => unlocked;
