import { useLocation } from "wouter";
import { useApi, useAsync, useMe } from "../api/context";
import type { Decks, TrainSettings, TrainStats } from "../api/types";
import { avatarInfo } from "../avatars";
import { Avatar } from "../components/Avatar";
import { AwardBadge } from "../components/AwardBadge";
import { Button, Card, Pill } from "../components/Controls";
import { Icon } from "../components/Icon";
import { allYears, defaultTraining, load, save } from "../lib/prefs";
import {
  accColor,
  Brand,
  DeckArt,
  deckLabel,
  Problem,
  Screen,
  Unpaired,
} from "./common";
import { useStartTraining } from "./train/useStart";

const cap = (s: string) => s[0].toUpperCase() + s.slice(1);

// lastTraining is the saved training settings, else the first card set.
export function lastTraining(decks?: Decks): TrainSettings {
  return (
    load<TrainSettings>("train") ??
    defaultTraining(
      allYears({ sets: decks?.sets[0] ? [decks.sets[0].id] : [] }),
    )
  );
}

function GameNight({ avatars }: { avatars: string[] }) {
  const [, go] = useLocation();
  return (
    <Card className="hero" style={{ marginTop: 18, padding: 18 }}>
      <div className="kata">Game night</div>
      <div
        style={{
          fontSize: 24,
          fontWeight: 750,
          margin: "6px 0 2px",
          letterSpacing: -0.3,
        }}
      >
        Play with friends
      </div>
      <div className="sub" style={{ maxWidth: 250 }}>
        One screen plays, phones are buzzers. Fastest right answer wins.
      </div>
      <div className="stack" style={{ marginTop: 12 }}>
        {avatars.slice(0, 3).map((a, i) => (
          <Avatar key={i} avatar={a} size={30} alt="" />
        ))}
      </div>
      <div className="row" style={{ marginTop: 16, gap: 8 }}>
        <Button icon="party" style={{ flex: 2 }} onClick={() => go("/host")}>
          Host a game
        </Button>
        <Button
          variant="ghost"
          icon="qr"
          style={{ flex: 1 }}
          onClick={() => go("/join")}
        >
          Join
        </Button>
      </div>
    </Card>
  );
}

function Progress({ stats }: { stats: TrainStats }) {
  const total = stats.mastered + stats.learning + stats.new || 1;
  return (
    <>
      <div className="label">Your progress</div>
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

export function Home() {
  const api = useApi();
  const [, go] = useLocation();
  const { me, unpaired, loading, error: meError, reload } = useMe();
  const paired = !!me;
  const stats = useAsync(paired ? "stats" : null, () => api.trainStats());
  const decks = useAsync(paired ? "decks" : null, () => api.decks());
  const profiles = useAsync(paired ? "profiles" : null, () => api.profiles());
  const training = useStartTraining();
  const today = lastTraining(decks.data);

  const decade = (d: number | null) => {
    const deck =
      d == null
        ? { ...today.deck, from: allYears().from, to: allYears().to }
        : { ...today.deck, from: d, to: d + 9 };
    training.start({ ...today, deck }, false);
  };
  const weakest = stats.data?.decades.length
    ? [...stats.data.decades].sort((a, b) => a.accuracy - b.accuracy)[0]
    : null;
  const overall = stats.data?.decades.length
    ? stats.data.decades.reduce((a, d) => a + d.accuracy, 0) /
      stats.data.decades.length
    : 0;

  const pickDeck = (deck: TrainSettings["deck"]) => {
    save("train", { ...today, deck });
    go("/train");
  };

  return (
    <Screen tab="home">
      <div className="row between" style={{ marginTop: 4 }}>
        <Brand />
        <div className="row">
          {stats.data && (
            <Pill>
              <Icon name="flame" size={16} />
              <b className="num">{stats.data.streakDays}</b> days
            </Pill>
          )}
          {me && (
            <button
              type="button"
              className="me-btn"
              title="Profile & devices"
              aria-label="Profile & devices"
              onClick={() => go("/me")}
              style={{
                boxShadow: `0 0 0 2px var(--ink),0 0 0 3.5px ${avatarInfo(me.profile.avatar).bg}`,
              }}
            >
              <Avatar avatar={me.profile.avatar} size={36} alt="" />
            </button>
          )}
        </div>
      </div>
      <div className="cols">
        <div className="col">
          <GameNight
            avatars={
              profiles.data?.map((p) => p.avatar) ?? ["vinyl", "mic", "guitar"]
            }
          />
          {unpaired && <Unpaired what="Training, hosting and progress" />}
          {meError && <Problem error={meError} retry={reload} />}
          {paired && (
            <Card style={{ marginTop: 10, padding: "12px 14px" }}>
              <div className="row between" style={{ gap: 8 }}>
                <div style={{ minWidth: 0 }}>
                  <div className="kata" style={{ color: "var(--sky)" }}>
                    Solo training
                  </div>
                  <div style={{ fontWeight: 650, fontSize: 15, marginTop: 2 }}>
                    {deckLabel(today.deck, decks.data)}
                  </div>
                  <div className="sub" style={{ fontSize: 12.5 }}>
                    {today.length} songs · ~
                    {Math.ceil((today.length * 25) / 60)} min
                    {stats.data ? ` · ${stats.data.learning} learning` : ""}
                  </div>
                </div>
                <div className="row" style={{ gap: 6, flex: "none" }}>
                  <button
                    type="button"
                    className="icon-btn"
                    title="Change deck"
                    aria-label="Change deck"
                    onClick={() => go("/train")}
                  >
                    <Icon name="train" />
                  </button>
                  <Button
                    small
                    variant="sky"
                    icon="play"
                    aria-label="Start training"
                    disabled={training.busy}
                    onClick={() => training.start(today)}
                  >
                    Train
                  </Button>
                </div>
              </div>
            </Card>
          )}
          {training.error && <Problem error={training.error} />}
          {stats.error && <Problem error={stats.error} retry={stats.reload} />}
          {stats.data && <Progress stats={stats.data} />}
        </div>
        <div className="col">
          {stats.data && stats.data.decades.length > 0 && (
            <>
              <div className="label row between">
                <span>Decades</span>
                {weakest && (
                  <button
                    type="button"
                    className="link"
                    style={{ textTransform: "none", letterSpacing: 0 }}
                    onClick={() => decade(weakest.decade)}
                  >
                    Train weakest →
                  </button>
                )}
              </div>
              <div className="decades">
                {stats.data.decades.map((d) => (
                  <button
                    key={d.decade}
                    type="button"
                    className="dec"
                    aria-label={`Train the ${d.decade}s`}
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
            </>
          )}
          {decks.data && (
            <>
              <div className="label">Decks</div>
              {decks.data.sets.slice(0, 4).map((s, i) => (
                <button
                  key={s.id}
                  type="button"
                  className="deck"
                  onClick={() => pickDeck(allYears({ sets: [s.id] }))}
                >
                  <DeckArt name={s.name} i={i} />
                  <div className="meta">
                    <div className="t">{s.name}</div>
                    <div className="s">
                      Card set · {s.from}–{s.to} · {s.count} cards
                    </div>
                  </div>
                </button>
              ))}
              {decks.data.categories.slice(0, 2).map((c, i) => (
                <button
                  key={c.id}
                  type="button"
                  className="deck"
                  onClick={() => pickDeck(allYears({ categories: [c.id] }))}
                >
                  <DeckArt name={c.name} i={i + 4} />
                  <div className="meta">
                    <div className="t">{c.name}</div>
                    <div className="s">Category · {c.count} songs</div>
                  </div>
                </button>
              ))}
            </>
          )}
          {loading && !me && !unpaired && !meError && (
            <div className="sub" style={{ marginTop: 18 }} role="status">
              Loading…
            </div>
          )}
        </div>
      </div>
    </Screen>
  );
}
