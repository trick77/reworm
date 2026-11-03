// Party players on the stage: the strip during a round, the standings after it, the podium at game over.
import { cx } from "../lib/cx";
import { Avatar } from "./Avatar";

export type Player = {
  name: string;
  avatar: string;
  score: number;
  // out: answered wrong, locked out for the round. The stage shows who, never what they answered.
  out?: boolean;
};

export function PlayerStrip({
  players,
  showScore = true,
  size = 62,
}: {
  players: Player[];
  showScore?: boolean;
  size?: number;
}) {
  return (
    <div className="strip">
      {players.map((p) => (
        <div
          key={p.name}
          className={cx("p", p.out && "out")}
          data-out={p.out ? "" : undefined}
        >
          <Avatar avatar={p.avatar} size={size} alt="" />
          {p.out && (
            <span className="x" aria-label="locked out">
              ✕
            </span>
          )}
          <div className="n">{p.name}</div>
          {showScore && <div className="s num sub">{p.score} pt</div>}
        </div>
      ))}
    </div>
  );
}

// Standings: the top rows by score, the round's winner marked ▲ (and first among equals).
export function Standings({
  players,
  winner,
  limit = 4,
}: {
  players: Player[];
  winner?: string;
  limit?: number;
}) {
  const rows = [...players]
    .sort(
      (a, b) =>
        b.score - a.score ||
        Number(b.name === winner) - Number(a.name === winner),
    )
    .slice(0, limit);
  return (
    <div className="card" style={{ padding: "8px 16px", textAlign: "left" }}>
      {rows.map((p, i) => (
        <div className="set-row" key={p.name}>
          <div className="row">
            <b className="num" style={{ width: 18, color: "var(--muted)" }}>
              {i + 1}
            </b>
            <Avatar avatar={p.avatar} size={30} alt="" />
            <b>{p.name}</b>
          </div>
          <b className="num">
            {p.score}
            {p.name === winner && (
              <span style={{ color: "var(--jade)" }}> ▲</span>
            )}
          </b>
        </div>
      ))}
    </div>
  );
}

const MEDAL = ["#cfd3d8", "#f3d27a", "#c98a52"]; // silver, gold, bronze: podium order 2 · 1 · 3
const HEIGHT = [120, 170, 90];
const PLACE = [2, 1, 3];

// Podium: the top three by score as 2 · 1 · 3, everyone else in a line under it.
export function Podium({ players }: { players: Player[] }) {
  const sorted = [...players].sort((a, b) => b.score - a.score);
  const pod = [sorted[1], sorted[0], sorted[2]];
  return (
    <>
      <div className="podium">
        {pod.map((p, i) =>
          p ? (
            <div className="place" key={p.name} data-place={PLACE[i]}>
              <div className={cx(i === 1 && "pop")}>
                <Avatar avatar={p.avatar} size={i === 1 ? 104 : 80} alt="" />
              </div>
              <div className="who">
                {p.name} · <span className="num">{p.score}</span>
              </div>
              <div
                className="block num"
                style={{
                  height: HEIGHT[i],
                  background: `linear-gradient(180deg,${MEDAL[i]},color-mix(in srgb,${MEDAL[i]} 45%,#13110f))`,
                }}
              >
                {PLACE[i]}
              </div>
            </div>
          ) : null,
        )}
      </div>
      {sorted.length > 3 && (
        <div className="sub podium-rest">
          {sorted
            .slice(3)
            .map((p) => `${p.name} ${p.score}`)
            .join(" · ")}
        </div>
      )}
    </>
  );
}

const CONFETTI = ["#e5482f", "#e9b44c", "#3dbb8c", "#69aee8", "#c86bd9"];

export function Confetti() {
  return (
    <div className="confetti" aria-hidden="true">
      {Array.from({ length: 36 }, (_, i) => (
        <i
          key={i}
          style={{
            left: `${(i * 137) % 100}%`,
            background: CONFETTI[i % 5],
            animationDelay: `${(i % 9) * 0.25}s`,
            animationDuration: `${2.6 + (i % 5) * 0.4}s`,
          }}
        />
      ))}
    </div>
  );
}
