import {
  useEffect,
  useEffectEvent,
  useRef,
  useState,
  type CSSProperties,
} from "react";
import { secondsLeft, serverNow } from "../api/clock";
import type { RoomState } from "../api/types";
import { cx } from "../lib/cx";

const TICK_MS = 250;

// CountdownRing drains a ring over `seconds` and counts the seconds down; onDone fires at 0.
// It counts from mount: give it a new key to restart (a new round). The number is the run time without the
// paused stretches, as the CSS ring (paused animation) is, so the two stay in step across pauses.
export function CountdownRing({
  seconds,
  size = 120,
  paused,
  onDone,
}: {
  seconds: number;
  size?: number;
  paused?: boolean;
  onDone?: () => void;
}) {
  const [left, setLeft] = useState(seconds);
  const ran = useRef(0); // ms run before the current stretch
  const done = useEffectEvent(() => onDone?.());

  useEffect(() => {
    if (paused) return;
    const since = Date.now();
    const elapsed = () => ran.current + Date.now() - since;
    const id = setInterval(() => {
      const left = Math.max(0, seconds - Math.floor(elapsed() / 1000));
      setLeft(left);
      if (left === 0) clearInterval(id);
    }, TICK_MS);
    return () => {
      ran.current = elapsed();
      clearInterval(id);
    };
  }, [paused, seconds]);

  useEffect(() => {
    if (left === 0) done();
  }, [left]);

  const r = (size - 12) / 2;
  const c = 2 * Math.PI * r;
  const bar: CSSProperties & { "--c": number } = {
    animationDuration: `${seconds}s`,
    "--c": c,
  };
  return (
    <div
      className={cx("countdown", paused && "paused")}
      style={{ width: size, height: size }}
      role="timer"
      aria-label={`${left} seconds left`}
    >
      <svg width={size} height={size} aria-hidden="true">
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          stroke="var(--panel2)"
          strokeWidth={10}
          fill="none"
        />
        <circle
          className="bar"
          cx={size / 2}
          cy={size / 2}
          r={r}
          stroke="var(--gold)"
          strokeWidth={10}
          fill="none"
          strokeLinecap="round"
          strokeDasharray={c}
          style={bar}
        />
      </svg>
      <div className="sec num" style={{ fontSize: size * 0.32 }}>
        {left}
      </div>
    </div>
  );
}

// useClock is the server time, ticking every 250 ms; it stands still while paused.
function useClock(paused?: boolean): number {
  const [now, setNow] = useState(serverNow);
  useEffect(() => {
    if (paused) return;
    const id = setInterval(() => setNow(serverNow()), TICK_MS);
    return () => clearInterval(id);
  }, [paused]);
  return now;
}

// remaining: ms left to `endsAt` at `now`, and the share of `total` seconds it is (1 → 0).
function remaining(endsAt: number, total: number, now: number) {
  const ms = Math.max(0, endsAt - now);
  return { ms, share: total > 0 ? Math.min(1, ms / (total * 1000)) : 0 };
}

// DeadlineRing counts down to a server deadline (endsAt, server ms) out of `total` seconds. Each tick
// reads the server clock, so a new state (re-render) never restarts or speeds it up, and a phone that
// joins late shows the time actually left. A pause holds it; a resume's later endsAt carries on.
export function DeadlineRing({
  endsAt,
  total,
  size = 120,
  paused,
}: {
  endsAt: number;
  total: number;
  size?: number;
  paused?: boolean;
}) {
  const now = useClock(paused);
  const { ms, share } = remaining(endsAt, total, now);
  const left = Math.ceil(ms / 1000);
  const r = (size - 12) / 2;
  const c = 2 * Math.PI * r;
  return (
    <div
      className={cx("countdown", paused && "paused")}
      style={{ width: size, height: size }}
      role="timer"
      aria-label={`${left} seconds left`}
    >
      <svg width={size} height={size} aria-hidden="true">
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          stroke="var(--panel2)"
          strokeWidth={10}
          fill="none"
        />
        <circle
          className="bar-live"
          cx={size / 2}
          cy={size / 2}
          r={r}
          stroke="var(--gold)"
          strokeWidth={10}
          fill="none"
          strokeLinecap="round"
          strokeDasharray={c}
          strokeDashoffset={c * (1 - share)}
        />
      </svg>
      <div className="sec num" style={{ fontSize: size * 0.32 }}>
        {left}
      </div>
    </div>
  );
}

// useSecondsTo is the whole seconds left to `at` (server ms), 0 once it passed or for null. It re-renders exactly
// on each server-clock second, not on a poll: every screen changes digit, and starts the round, at the same moment.
export function useSecondsTo(at: number | null): number {
  const [tick, setTick] = useState(0);
  const left = at == null ? 0 : secondsLeft(at);
  useEffect(() => {
    if (at == null || left === 0) return;
    const ms = at - serverNow() - (left - 1) * 1000;
    const id = setTimeout(() => setTick((t) => t + 1), Math.max(0, ms));
    return () => clearTimeout(id);
  }, [at, left, tick]);
  return left;
}

// StartCountdown is the 3, 2, 1 before a round: `left` seconds to the clip's start (useSecondsTo).
export function StartCountdown({
  left,
  round,
}: {
  left: number;
  round: number;
}) {
  return (
    <div
      className="go center"
      role="timer"
      aria-label={`Round ${round} starts in ${left}`}
    >
      <div key={left} className="go-num num pop">
        {left}
      </div>
      <div className="sub">Round {round} · get ready</div>
    </div>
  );
}

// started is the room as the screens draw it: a countdown whose start passed by the server clock already plays,
// without waiting for the event that says so (the server's tick, up to 100 ms later, plus the network). A press
// then is fine: the server starts the round itself when the press arrives (Room.touch).
export function started(s: RoomState, left: number): RoomState {
  return s.phase === "countdown" && left === 0 ? { ...s, phase: "playing" } : s;
}

// DeadlineBar is the pad's bar for a server deadline, read from the server clock each tick (as
// DeadlineRing).
export function DeadlineBar({
  endsAt,
  total,
  paused,
}: {
  endsAt: number;
  total: number;
  paused?: boolean;
}) {
  const now = useClock(paused);
  const { share } = remaining(endsAt, total, now);
  return (
    <div
      className="tbar live"
      role="progressbar"
      aria-label="Time left"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(share * 100)}
    >
      <i style={{ width: `${share * 100}%` }} />
    </div>
  );
}

// TimerBar is the pad's draining bar for the answer window. Restart with a new key.
export function TimerBar({
  seconds,
  paused,
}: {
  seconds: number;
  paused?: boolean;
}) {
  return (
    <div className={cx("tbar", paused && "paused")} aria-hidden="true">
      <i style={{ animationDuration: `${seconds}s` }} />
    </div>
  );
}
