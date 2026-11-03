import { useEffect, useState } from "react";
import {
  player as sharedPlayer,
  type ClipState,
  type PreviewPlayer,
} from "./player";

// usePlayer re-renders on the player's progress (every 100 ms while a clip plays) and returns its state.
export function usePlayer(p: PreviewPlayer = sharedPlayer()): ClipState & {
  player: PreviewPlayer;
} {
  const [state, setState] = useState<ClipState>(() => p.state());
  useEffect(() => {
    const off = p.onProgress(setState);
    const offEnd = p.onEnd(() => setState(p.state()));
    return () => {
      off();
      offEnd();
    };
  }, [p]);
  return { ...state, player: p };
}
