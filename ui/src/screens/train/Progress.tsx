// The top of /train: the streak, the record award with mastered / learning / new, and the decade tiles, each a
// one-off session of that decade with the settings in the form below (the training defaults stay).
import type { ApiError } from "../../api/client";
import type { TrainSettings, TrainStats } from "../../api/types";
import { AwardBadge } from "../../components/AwardBadge";
import { Card } from "../../components/Controls";
import { Icon } from "../../components/Icon";
import { allYears } from "../../lib/prefs";
import { accColor, Problem } from "../common";

const cap = (s: string) => s[0].toUpperCase() + s.slice(1);

// DecadeStart: a tile's one-off start. It is the screen's single start, shared with its Start button, so busy
// disables both; error is a failed tile start.
export type DecadeStart = {
  start: (s: TrainSettings) => void;
  busy: boolean;
  error: ApiError | null;
};

export function Progress({
  stats,
  settings,
  training,
}: {
  stats: TrainStats;
  settings: TrainSettings;
  training: DecadeStart;
}) {
  return (
    <div className="cols">
      <div className="col">
        <Record stats={stats} />
      </div>
      <div className="col">
        {stats.decades.length > 0 && (
          <Decades stats={stats} settings={settings} training={training} />
        )}
      </div>
    </div>
  );
}

function Record({ stats }: { stats: TrainStats }) {
  const total = stats.mastered + stats.learning + stats.new || 1;
  return (
    <>
      <div className="label row between">
        <span>Your progress</span>
        <span
          className="row paper"
          style={{ gap: 4, letterSpacing: 0, textTransform: "none" }}
        >
          <Icon name="flame" size={14} />
          <b className="num">{stats.streakDays}</b> days
        </span>
      </div>
      <Card>
        <div className="row between" style={{ marginBottom: 12 }}>
          <AwardBadge tier={stats.award}>{cap(stats.award)} record</AwardBadge>
          <span className="sub num">
            {stats.xp.toLocaleString("en")}
            {stats.nextAt != null &&
              ` / ${stats.nextAt.toLocaleString("en")}`}{" "}
            XP
          </span>
        </div>
        <div className="buckets" aria-hidden="true">
          <i
            style={{
              width: `${(stats.mastered / total) * 100}%`,
              background: "var(--jade)",
            }}
          />
          <i
            style={{
              width: `${(stats.learning / total) * 100}%`,
              background: "var(--gold)",
            }}
          />
          <i style={{ flex: 1, background: "#3a332c" }} />
        </div>
        <div className="legend">
          <span>
            <i className="dot" style={{ background: "var(--jade)" }} />
            <b>{stats.mastered}</b> mastered
          </span>
          <span>
            <i className="dot" style={{ background: "var(--gold)" }} />
            <b>{stats.learning}</b> learning
          </span>
          <span>
            <i className="dot" style={{ background: "#5a5047" }} />
            <b>{stats.new}</b> new
          </span>
        </div>
      </Card>
    </>
  );
}

function Decades({
  stats,
  settings,
  training,
}: {
  stats: TrainStats;
  settings: TrainSettings;
  training: DecadeStart;
}) {
  const decade = (d: number | null) => {
    const deck =
      d == null
        ? { ...settings.deck, from: allYears().from, to: allYears().to }
        : { ...settings.deck, from: d, to: d + 9 };
    training.start({ ...settings, deck });
  };
  const weakest = [...stats.decades].sort((a, b) => a.accuracy - b.accuracy)[0];
  const overall =
    stats.decades.reduce((a, d) => a + d.accuracy, 0) / stats.decades.length;
  return (
    <>
      <div className="label row between">
        <span>Decades</span>
        <button
          type="button"
          className="link"
          style={{ textTransform: "none", letterSpacing: 0 }}
          disabled={training.busy}
          onClick={() => decade(weakest.decade)}
        >
          Train weakest →
        </button>
      </div>
      <div className="decades">
        {stats.decades.map((d) => (
          <button
            key={d.decade}
            type="button"
            className="dec"
            aria-label={`Train the ${d.decade}s`}
            disabled={training.busy}
            onClick={() => decade(d.decade)}
            style={{
              background: `color-mix(in srgb,${accColor(d.accuracy)} 16%,var(--panel))`,
              color: accColor(d.accuracy),
            }}
          >
            <div className="d">’{String(d.decade).slice(2)}s</div>
            <div className="p num">{Math.round(d.accuracy * 100)}%</div>
            <div className="bar">
              <i style={{ width: `${d.accuracy * 100}%` }} />
            </div>
          </button>
        ))}
        <button
          type="button"
          className="dec"
          aria-label="Train all decades"
          disabled={training.busy}
          onClick={() => decade(null)}
          style={{
            background: `color-mix(in srgb,${accColor(overall)} 16%,var(--panel))`,
            color: accColor(overall),
          }}
        >
          <div className="d">All</div>
          <div className="p num">{Math.round(overall * 100)}%</div>
          <div className="bar">
            <i style={{ width: `${overall * 100}%` }} />
          </div>
        </button>
      </div>
      {training.error && <Problem error={training.error} />}
    </>
  );
}
