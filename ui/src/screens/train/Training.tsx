// /train/:id — a training session on this device: listen & guess, the reveal, the summary. The session
// lives on the server, so a reload resumes at the current card.
import { useEffect, useEffectEvent, useRef, useState } from "react";
import { useLocation } from "wouter";
import type { ApiError } from "../../api/client";
import { toApiError, useApi, useAsync } from "../../api/context";
import type {
  Award,
  Card as TCard,
  Choice,
  TrainAnswer,
  TrainSettings,
} from "../../api/types";
import { player } from "../../audio/player";
import { usePlayer } from "../../audio/usePlayer";
import { AppShell } from "../../components/AppShell";
import { AWARD_TIERS, AwardBadge } from "../../components/AwardBadge";
import { Button, Card, IconButton } from "../../components/Controls";
import { Cover, SongHead } from "../../components/Cover";
import { Icon } from "../../components/Icon";
import { Verdict, type VerdictResult } from "../../components/Verdict";
import { Vinyl } from "../../components/Vinyl";
import { ClipTime, Waveform } from "../../components/Waveform";
import { YearCard } from "../../components/YearCard";
import { cx } from "../../lib/cx";
import { load, YEAR_MAX, YEAR_MIN } from "../../lib/prefs";
import {
  Loading,
  Problem,
  Ring,
  Screen,
  ScreenHead,
  TIERS,
  Unpaired,
} from "../common";
import { useStartTraining } from "./useStart";

type Mark = "ok" | "bad";
type Answered = { card: TCard; pick: Choice | number; res: TrainAnswer };

const plural = (n: number, w: string) => `${n} ${w}${n === 1 ? "" : "s"}`;

function Progress({
  marks,
  index,
  total,
}: {
  marks: (Mark | null)[];
  index: number;
  total: number;
}) {
  return (
    <div className="prog" aria-label={`Song ${index + 1} of ${total}`}>
      {Array.from({ length: Math.max(total, marks.length) }, (_, i) => (
        <i key={i} className={cx(marks[i] ?? (i === index && "cur"))} />
      ))}
    </div>
  );
}

function Counts({
  done,
  total,
  right,
  learning,
  streak,
}: {
  done: number;
  total: number;
  right: number;
  learning: number;
  streak: number;
}) {
  return (
    <div className="counts">
      <span>
        <b className="num">{done}</b> / {total}
      </span>
      <span>
        <i className="dot" style={{ background: "var(--jade)" }} />
        <b className="num">{right}</b> right &nbsp;{" "}
        <i className="dot" style={{ background: "var(--gold)" }} />
        <b className="num">{learning}</b> learning
      </span>
      <span>
        <Icon name="flame" /> <b className="num">{streak}</b>
      </span>
    </div>
  );
}

function Listen() {
  const clip = usePlayer();
  const p = clip.player;
  return (
    <>
      <Vinyl spinning={clip.playing} />
      <Waveform progress={clip.pos / clip.total} level={clip.level} />
      <ClipTime
        pos={clip.pos}
        total={clip.total}
        level={clip.level}
        note="♪ live preview"
      />
      <div className="player-ctl">
        {clip.playing ? (
          <button type="button" onClick={() => void p.pause()}>
            <Icon name="pause" />
            Pause
          </button>
        ) : (
          <button type="button" onClick={() => p.resume()}>
            <Icon name="play" />
            Play
          </button>
        )}
        <button type="button" onClick={() => p.replay()}>
          <Icon name="replay" />
          Replay
        </button>
      </div>
    </>
  );
}

function PinDial({ onPin }: { onPin: (year: number) => void }) {
  const [year, setYear] = useState(1990);
  const lock = useEffectEvent(() => onPin(year));
  useEffect(() => {
    const on = (e: KeyboardEvent) => {
      if (e.key === "Enter") {
        e.preventDefault();
        lock();
      }
    };
    window.addEventListener("keydown", on);
    return () => window.removeEventListener("keydown", on);
  }, []);
  return (
    <>
      <div className="dial">
        <div className="sub" style={{ fontSize: 13 }}>
          Your year
        </div>
        <div className="y">{year}</div>
        <input
          type="range"
          aria-label="Your year"
          min={YEAR_MIN}
          max={YEAR_MAX}
          value={year}
          onChange={(e) => setYear(+e.target.value)}
        />
        <div className="row between sub num" style={{ fontSize: 11 }}>
          <span>{YEAR_MIN}</span>
          <span>{YEAR_MAX}</span>
        </div>
      </div>
      <Button style={{ marginTop: 14 }} onClick={() => onPin(year)}>
        Lock it in <span className="kbd">⏎</span>
      </Button>
    </>
  );
}

function verdictOf(a: Answered): {
  result: VerdictResult;
  what: string;
  diff: number;
} {
  const y = a.res.song.year;
  if (typeof a.pick === "number") {
    const diff = Math.abs(a.pick - y);
    return {
      result: diff === 0 ? "bullseye" : a.res.correct ? "hit" : "miss",
      diff,
      what:
        diff === 0
          ? "Exact year!"
          : a.res.correct
            ? "Within a year"
            : `Off by ${plural(diff, "year")}`,
    };
  }
  const p = a.card.proposedYear!;
  const diff = Math.abs(p - y);
  const dir = y < p ? "earlier" : y > p ? "later" : "exact";
  if (a.res.correct)
    return {
      result: dir === "exact" ? "bullseye" : "hit",
      diff,
      what:
        dir === "exact"
          ? `Spot on — it's ${y}`
          : `Yes, ${dir} — ${plural(diff, "year")} ${dir === "earlier" ? "before" : "after"} ${p}`,
    };
  return {
    result: "miss",
    diff,
    what:
      a.pick === "exact"
        ? `Not ${p} — it's ${dir}`
        : dir === "exact"
          ? `It was exactly ${p}!`
          : `It's ${dir}, not ${a.pick}`,
  };
}

function Reveal({ a, streak }: { a: Answered; streak: number }) {
  const api = useApi();
  const s = a.res.song;
  const v = verdictOf(a);
  const tier = TIERS.find((t) => t.k === a.res.tier);
  const pin = typeof a.pick === "number";
  return (
    <div className="cols wide">
      <div className="col">
        <SongHead
          cover={
            <Cover
              key={s.id}
              title={s.title}
              artist={s.artist}
              album={s.title}
              src={api.coverUrl(s.id)}
              flip
            />
          }
          title={s.title}
          artist={s.artist}
        />
        <Verdict
          result={v.result}
          year={s.year}
          what={v.what}
          tier={
            !pin && tier
              ? {
                  label:
                    tier.k === "exact"
                      ? tier.label
                      : `${tier.label} · ${v.diff} yr${v.diff > 1 ? "s" : ""} off`,
                  color: tier.col,
                }
              : undefined
          }
          why={
            a.res.correct
              ? `+${a.res.xp} XP${streak > 1 ? ` · streak ${streak}` : ""}`
              : a.card.retry
                ? "Still learning — it comes back next session."
                : "Back in a few cards with a different year."
          }
          ruler={{
            shown: pin ? (a.pick as number) : a.card.proposedYear!,
            mode: pin ? "pin" : "ab",
          }}
        />
      </div>
      <div className="col">
        <div className="src">
          <div className="ic" aria-hidden="true">
            ⓘ
          </div>
          <div>
            <b>First released {s.year}</b>
            <div className="m">
              The initial release, never a remaster, re-release or compilation
              date.
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

const nextAward = (a: Award) =>
  AWARD_TIERS[Math.min(AWARD_TIERS.length - 1, AWARD_TIERS.indexOf(a) + 1)];

function Summary({ id, t0 }: { id: string; t0: number }) {
  const api = useApi();
  const [, go] = useLocation();
  const sum = useAsync(`sum:${id}`, () => api.trainSummary(id));
  const stats = useAsync("stats", () => api.trainStats());
  const again = useStartTraining();
  const [now] = useState(() => Date.now());
  const s = sum.data;
  const col = (p: number) =>
    p > 0.78 ? "var(--jade)" : p > 0.55 ? "var(--gold)" : "var(--rose)";
  const mins = Math.max(1, Math.round((now - t0) / 60000));
  const settings = load<TrainSettings>("train");
  return (
    <AppShell
      footer={
        <div className="cta2">
          <Button variant="ghost" onClick={() => go("/")}>
            Done
          </Button>
          <Button
            disabled={!settings || again.busy}
            onClick={() => settings && again.start(settings)}
          >
            Another round
          </Button>
        </div>
      }
      scrollClassName="fade-in"
    >
      <div className="screen-head">
        <span />
        <div className="t">Session complete</div>
        <IconButton icon="x" label="Close" onClick={() => go("/")} />
      </div>
      {sum.error && <Problem error={sum.error} retry={sum.reload} />}
      {!s && !sum.error && <Loading />}
      {s && (
        <div className="cols">
          <div className="col">
            <div className="ring">
              <Ring p={s.total ? s.correct / s.total : 0} />
              <div className="c">
                <b>{s.total ? Math.round((s.correct / s.total) * 100) : 0}%</b>
                <span>
                  {s.correct} of {s.total} right
                </span>
              </div>
            </div>
            <div className="stats3">
              <Card>
                <b className="num">+{s.xp}</b>
                <span>XP</span>
              </Card>
              <Card>
                <b className="num">{s.bestStreak}</b>
                <span>best streak</span>
              </Card>
              <Card>
                <b className="num">{mins}m</b>
                <span>time</span>
              </Card>
            </div>
            {stats.data && (
              <Card style={{ marginTop: 10 }}>
                <div className="beltprog">
                  <AwardBadge tier={stats.data.award} />
                  <div className="b">
                    <i
                      style={{
                        width: `${stats.data.nextAt ? Math.min(100, (stats.data.xp / stats.data.nextAt) * 100) : 100}%`,
                      }}
                    />
                  </div>
                  <span className="sub num" style={{ fontSize: 12 }}>
                    {stats.data.nextAt != null
                      ? `${Math.max(0, stats.data.nextAt - stats.data.xp)} XP to ${nextAward(stats.data.award)}`
                      : "Top award"}
                  </span>
                </div>
              </Card>
            )}
          </div>
          <div className="col">
            <div className="label">By decade</div>
            <Card>
              {s.byDecade.length ? (
                s.byDecade.map((d) => {
                  const p = d.n ? d.ok / d.n : 0;
                  return (
                    <div className="dbar" key={d.decade}>
                      <span className="n">’{String(d.decade).slice(2)}s</span>
                      <div className="b">
                        <i
                          style={{
                            width: `${Math.max(p * 100, 4)}%`,
                            background: col(p),
                          }}
                        />
                      </div>
                      <span className="v">
                        {d.ok}/{d.n}
                      </span>
                    </div>
                  );
                })
              ) : (
                <div className="sub">No answers yet.</div>
              )}
            </Card>
            {s.misses.length > 0 && (
              <>
                <div className="label">To learn ({s.misses.length})</div>
                <Card style={{ padding: "4px 14px" }}>
                  {s.misses.map((m, i) => (
                    <div className="miss" key={`${m.song.id}${i}`}>
                      <Cover
                        title={m.song.title}
                        artist={m.song.artist}
                        album={m.song.title}
                        src={api.coverUrl(m.song.id)}
                        size={42}
                        mini
                      />
                      <div style={{ minWidth: 0 }}>
                        <div className="nm">{m.song.title}</div>
                        <div className="sub" style={{ fontSize: 12 }}>
                          {m.song.artist}
                        </div>
                      </div>
                      <div className="y">
                        <b>{m.song.year}</b>
                        <div>
                          {typeof m.choice === "number"
                            ? `you: ${m.choice}`
                            : `${m.proposedYear} → ${m.choice === "exact" ? "spot on" : m.choice}`}
                        </div>
                      </div>
                    </div>
                  ))}
                </Card>
              </>
            )}
          </div>
        </div>
      )}
      <div style={{ height: 8 }} />
    </AppShell>
  );
}

type View = "play" | "reveal" | "summary";

export function Training({ id }: { id: string }) {
  const api = useApi();
  const ses = useAsync(`ses:${id}`, () => api.trainSession(id));
  const [card, setCard] = useState<TCard | null>(null);
  const [view, setView] = useState<View>("play");
  const [answered, setAnswered] = useState<Answered | null>(null);
  const [marks, setMarks] = useState<Mark[]>([]);
  const [total, setTotal] = useState<number | null>(null);
  const [right, setRight] = useState(0);
  const [learning, setLearning] = useState(0);
  // null until the first answer here: the resumed session's own streak counts
  const [ownStreak, setStreak] = useState<number | null>(null);
  const streak = ownStreak ?? ses.data?.stats?.streak ?? 0;
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const [t0] = useState(() => Date.now());
  const started = useRef<string | null>(null);

  // the session as loaded: resume at its card (or straight to the summary when it's done)
  const cur = card ?? ses.data?.card ?? null;
  const index =
    (ses.data?.index ?? 0) + marks.length - (view === "reveal" ? 1 : 0);
  const count = total ?? ses.data?.total ?? 0;
  const done = ses.data && !ses.data.card;
  const shown: View = done ? "summary" : view;

  // play the card's clip once per card; the next card's clip loads during the reveal
  useEffect(() => {
    if (shown !== "play" || !cur || started.current === cur.id) return;
    started.current = cur.id;
    player().play(cur.clip.previewUrl, {
      from: cur.clip.from,
      length: cur.clip.length,
    });
  }, [shown, cur]);
  useEffect(() => () => void player().fadeOut(), []);

  const answer = (pick: Choice | number) => {
    if (!cur || busy) return;
    setBusy(true);
    setError(null);
    void player().fadeOut(0.6);
    api
      .trainAnswer(
        id,
        typeof pick === "number"
          ? { cardId: cur.id, year: pick }
          : { cardId: cur.id, choice: pick },
      )
      .then(
        (res) => {
          setBusy(false);
          setAnswered({ card: cur, pick, res });
          setMarks((m) => [...m, res.correct ? "ok" : "bad"]);
          setStreak(res.correct ? streak + 1 : 0);
          if (res.correct) setRight((n) => n + 1);
          if (!res.correct && !cur.retry) {
            setLearning((n) => n + 1);
            setTotal((t) => (t ?? ses.data?.total ?? 0) + 1);
          } else if (res.correct && cur.retry)
            setLearning((n) => Math.max(0, n - 1));
          if (res.next) player().prefetch(res.next.clip.previewUrl);
          setView("reveal");
        },
        (e: unknown) => {
          setBusy(false);
          setError(toApiError(e));
        },
      );
  };

  const next = () => {
    if (!answered) return;
    if (answered.res.next) {
      setCard(answered.res.next);
      setView("play");
    } else setView("summary");
  };
  const onEnter = useEffectEvent((e: KeyboardEvent) => {
    if (e.key !== "Enter" || shown !== "reveal") return;
    e.preventDefault();
    next();
  });
  const onKey = useEffectEvent((e: KeyboardEvent) => {
    if (shown !== "play" || e.target instanceof HTMLInputElement) return;
    const p = player();
    if (e.key === "r") p.replay();
    else if (e.key === "p") {
      if (p.state().playing) void p.pause();
      else p.resume();
    }
  });
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      onEnter(e);
      onKey(e);
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, []);

  if (ses.error)
    return (
      <Screen tab="train">
        <ScreenHead back="/" title="Training" />
        {ses.error.status === 401 ? (
          <Unpaired what="Training" />
        ) : ses.error.status === 404 ? (
          <Card style={{ marginTop: 18 }} role="alert">
            <div style={{ fontWeight: 700 }}>This session is gone</div>
            <div className="sub" style={{ fontSize: 13, marginTop: 4 }}>
              Start a new one from the Train tab.
            </div>
          </Card>
        ) : (
          <Problem error={ses.error} retry={ses.reload} />
        )}
      </Screen>
    );
  if (!ses.data)
    return (
      <AppShell>
        <Loading />
      </AppShell>
    );
  if (shown === "summary") return <Summary id={id} t0={t0} />;

  const head = (
    <>
      <div className="playhead">
        <IconButton
          icon="x"
          label="End session"
          onClick={() => setView("summary")}
        />
        <Progress
          marks={[...Array(ses.data.index).fill(null), ...marks]}
          index={index}
          total={count}
        />
      </div>
      <Counts
        done={Math.min(count, index + 1)}
        total={count}
        right={right + (ses.data.stats?.correct ?? 0)}
        learning={learning + (ses.data.stats?.learning ?? 0)}
        streak={streak}
      />
    </>
  );

  if (shown === "reveal" && answered)
    return (
      <AppShell
        footer={
          <Button onClick={next}>
            {answered.res.next ? "Next song" : "See results"}{" "}
            <span className="kbd">⏎</span>
          </Button>
        }
        scrollClassName="fade-in"
      >
        {head}
        <Reveal a={answered} streak={streak} />
        <div style={{ height: 8 }} />
      </AppShell>
    );

  return (
    <AppShell scrollClassName="playscr">
      {head}
      <div className="cols mid wide">
        <div className="col">
          <Listen />
        </div>
        <div className="col">
          {cur && cur.proposedYear != null ? (
            <YearCard
              key={cur.id}
              year={cur.proposedYear}
              question={
                cur.retry ? "Second try · new year — Released …" : "Released …"
              }
              keyboard={!busy}
              choices
              onAnswer={answer}
            />
          ) : cur ? (
            <PinDial key={cur.id} onPin={answer} />
          ) : null}
          {error && <Problem error={error} />}
        </div>
      </div>
    </AppShell>
  );
}
