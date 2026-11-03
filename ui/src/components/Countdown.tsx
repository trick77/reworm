import { useEffect, useEffectEvent, useState, type CSSProperties } from "react";
import { cx } from "../lib/cx";

// CountdownRing drains a ring over `seconds` and counts the seconds down; onDone fires at 0.
// It counts from mount: give it a new key to restart (a new round).
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
  const done = useEffectEvent(() => onDone?.());

  useEffect(() => {
    if (paused) return;
    const id = setInterval(() => setLeft((l) => Math.max(0, l - 1)), 1000);
    return () => clearInterval(id);
  }, [paused]);

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
