import { useEffect, useState } from "react";
import {
  player as sharedPlayer,
  type ClipState,
  type PreviewPlayer,
} from "./player";

// usePlayer re-renders on the player's progress (every 100 ms while a clip plays, and on every play, pause
// and end) and returns its state.
export function usePlayer(p: PreviewPlayer = sharedPlayer()): ClipState & {
  player: PreviewPlayer;
} {
  const [state, setState] = useState<ClipState>(() => p.state());
  useEffect(() => p.onProgress(setState), [p]);
  return { ...state, player: p };
}
