import { cx } from "../lib/cx";

const BARS = 40;
// The mock's bar heights: a fixed pseudo-waveform, 25–100 %.
const HEIGHTS = Array.from({ length: BARS }, (_, i) =>
  Math.round(
    (0.25 + 0.75 * Math.abs(Math.sin(i * 1.7) * Math.cos(i * 0.45))) * 100,
  ),
);

// Waveform shows clip progress (0–1); level (0–1, the fade) dims it, so a fade is visible.
export function Waveform({
  progress,
  level = 1,
  className,
}: {
  progress: number;
  level?: number;
  className?: string;
}) {
  const opacity = 0.25 + 0.75 * Math.min(1, Math.max(0, level));
  return (
    <div
      className={cx("wave", className)}
      style={{ opacity }}
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(Math.min(1, Math.max(0, progress)) * 100)}
    >
      {HEIGHTS.map((h, i) => (
        <i
          key={i}
          className={cx(i / BARS < progress && "on")}
          style={{ height: `${h}%` }}
        />
      ))}
    </div>
  );
}

export const clock = (s: number) =>
  `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, "0")}`;

// ClipTime is the row under the waveform: position, the volume meter (the fade, live), clip length.
export function ClipTime({
  pos,
  total,
  level,
  note,
}: {
  pos: number;
  total: number;
  level?: number;
  note?: string;
}) {
  return (
    <div className="time num">
      <span>{clock(pos)}</span>
      {level != null && (
        <span className="volrow" title="Volume: fades in 1.5 s, out 0.8 s">
          {note}
          <span className="vb">
            <i
              style={{
                width: `${Math.round(Math.min(1, Math.max(0, level)) * 100)}%`,
              }}
            />
          </span>
        </span>
      )}
      <span>{clock(total)}</span>
    </div>
  );
}
