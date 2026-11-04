import type { RoomState } from "../../api/types";
import { useSecondsTo } from "../../components/Countdown";

// started is the room as the screens draw it: a countdown whose start passed by the server clock already plays,
// without waiting for the event that says so (the server's tick, up to 100 ms later, plus the network). A press
// then is fine: the server starts the round itself when the press arrives (Room.touch).
export function started(s: RoomState, left: number): RoomState {
  return s.phase === "countdown" && left === 0 ? { ...s, phase: "playing" } : s;
}

// useStartedRoom is the room as drawn (started) and the countdown's seconds left, for stage and pad alike.
export function useStartedRoom<S extends RoomState | null | undefined>(
  raw: S,
): { s: S; left: number } {
  const left = useSecondsTo(
    raw?.phase === "countdown" ? (raw.round?.startedAt ?? null) : null,
  );
  return { s: (raw && started(raw, left)) as S, left };
}
