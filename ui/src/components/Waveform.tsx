import { cx } from "../lib/cx";

const BARS = 40;
// The mock's bar heights: a fixed pseudo-waveform, 25–100 %.
const HEIGHTS = Array.from({ length: BARS }, (_, i) =>
  Math.round(
    (0.25 + 0.75 * Math.abs(Math.sin(i * 1.7) * Math.cos(i * 0.45))) * 100,
  ),
);

// unit clamps to 0–1; NaN (0 / 0, an unknown length) is 0.
const unit = (n: number) => (Number.isNaN(n) ? 0 : Math.min(1, Math.max(0, n)));

// Waveform shows clip progress (0–1, pos / total: an unknown length gives 0); level (0–1, the fade) dims it,
// so a fade is visible.
export function Waveform({
  progress: raw,
  level = 1,
  className,
}: {
  progress: number;
  level?: number;
  className?: string;
}) {
  const progress = unit(raw);
  const opacity = 0.25 + 0.75 * unit(level);
  return (
    <div
      className={cx("wave", className)}
      style={{ opacity }}
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(progress * 100)}
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

// clock is m:ss; an unknown time (NaN, Infinity: before the metadata) is –:––, a negative one 0:00.
export function clock(s: number): string {
  if (!Number.isFinite(s)) return "–:––";
  const t = Math.max(0, Math.floor(s));
  return `${Math.floor(t / 60)}:${String(t % 60).padStart(2, "0")}`;
}

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
                width: `${Math.round(unit(level) * 100)}%`,
              }}
            />
          </span>
        </span>
      )}
      <span>{clock(total)}</span>
    </div>
  );
}
