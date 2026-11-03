import type { ReactNode } from "react";
import { cx } from "../lib/cx";

export type Grow = "left" | "right" | "center";
export type RulerMarker = {
  kind: "shown" | "actual";
  year: number;
  at: number; // percent from the left
  grow: Grow;
};
export type RulerLayout = {
  ticks: { year: number; at: number; big: boolean }[];
  gap: { left: number; width: number } | null;
  markers: RulerMarker[];
};

// rulerLayout places the shown and the actual year on a scale 4 years past both, a big tick every 5 years.
// The earlier year's label grows left from its dot and the later one's right, so years a step apart never
// collide; the same year is one centered marker.
export function rulerLayout(shown: number, actual: number): RulerLayout {
  const lo = Math.min(shown, actual) - 4;
  const hi = Math.max(shown, actual) + 4;
  const x = (y: number) => ((y - lo) / (hi - lo)) * 100;
  const ticks = [];
  for (let y = lo; y <= hi; y++)
    ticks.push({ year: y, at: x(y), big: y % 5 === 0 });
  if (shown === actual) {
    return {
      ticks,
      gap: null,
      markers: [
        { kind: "actual", year: actual, at: x(actual), grow: "center" },
      ],
    };
  }
  const a = x(Math.min(shown, actual));
  const b = x(Math.max(shown, actual));
  return {
    ticks,
    gap: { left: a, width: b - a },
    markers: [
      {
        kind: "shown",
        year: shown,
        at: x(shown),
        grow: shown > actual ? "right" : "left",
      },
      {
        kind: "actual",
        year: actual,
        at: x(actual),
        grow: actual > shown ? "right" : "left",
      },
    ],
  };
}

const pct = (n: number) => `${n.toFixed(2)}%`;

// Ruler: shown (or "you", in pin mode) vs. actual year on a scale, the gap between them in red.
export function Ruler({
  shown,
  actual,
  mode = "ab",
}: {
  shown: number;
  actual: number;
  mode?: "ab" | "pin";
}) {
  const r = rulerLayout(shown, actual);
  return (
    <div className="ruler" data-testid="ruler">
      <div className="line" />
      {r.ticks.map((t) => (
        <div key={t.year}>
          <div
            className={cx("tick", t.big && "big")}
            style={{ left: pct(t.at) }}
          />
          {t.big && (
            <div className="tl" style={{ left: pct(t.at) }}>
              {t.year}
            </div>
          )}
        </div>
      ))}
      {r.gap && (
        <div
          className="gap"
          style={{ left: pct(r.gap.left), width: pct(r.gap.width) }}
        />
      )}
      {r.markers.map((m) => (
        <div
          key={m.kind}
          className={cx("mk", m.kind, m.grow !== "center" && `grow-${m.grow}`)}
          style={{ left: pct(m.at) }}
        >
          {m.kind === "actual" ? "actual" : mode === "pin" ? "you" : "shown"}{" "}
          {m.year}
          <i />
        </div>
      ))}
    </div>
  );
}

export type VerdictResult = "hit" | "miss" | "bullseye";

const STAMP: Record<VerdictResult, ReactNode> = {
  hit: "HIT",
  miss: "MISS",
  bullseye: (
    <>
      BULLS
      <br />
      EYE
    </>
  ),
};

// Verdict is the reveal: stamp, the real year big, what happened, and the ruler.
export function Verdict({
  result,
  year,
  what,
  why,
  tier,
  ruler,
  stamp = true,
  center,
}: {
  result: VerdictResult;
  year: number;
  what: ReactNode;
  why?: ReactNode;
  tier?: { label: ReactNode; color: string };
  ruler?: { shown: number; mode?: "ab" | "pin" };
  stamp?: boolean;
  center?: boolean;
}) {
  return (
    <div
      className={cx(
        "verdict",
        result === "miss" ? "bad" : "ok",
        center && "center",
      )}
      data-result={result}
    >
      {stamp && <div className="stamp anim">{STAMP[result]}</div>}
      <div className="big num">{year}</div>
      <div className="what">{what}</div>
      {tier && (
        <div className="tier">
          <span className="pill" style={{ borderColor: tier.color }}>
            {tier.label}
          </span>
        </div>
      )}
      {why && <div className="why">{why}</div>}
      {ruler && <Ruler shown={ruler.shown} actual={year} mode={ruler.mode} />}
    </div>
  );
}
