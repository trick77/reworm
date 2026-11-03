// Dev-only component gallery (#gallery on the dev server): every component in its states, the party
// screens composed from them, and a hand test of both audio engines with real previews.
import { useState, type ReactNode } from "react";
import { usePlayer } from "../audio/usePlayer";
import { saveEngine, type Engine } from "../audio/player";
import { AVATARS } from "../avatars";
import type { Choice } from "../components/AnswerPad";
import { AnswerPad } from "../components/AnswerPad";
import { AppShell, type Tab } from "../components/AppShell";
import { Avatar, AvatarPicker } from "../components/Avatar";
import { AWARD_TIERS, AwardBadge } from "../components/AwardBadge";
import {
  Button,
  Card,
  Chip,
  IconButton,
  Pill,
  Segmented,
} from "../components/Controls";
import { CountdownRing, TimerBar } from "../components/Countdown";
import { Cover, SongHead } from "../components/Cover";
import { Icon } from "../components/Icon";
import { PlayerStrip, Podium, Standings } from "../components/Players";
import { Verdict } from "../components/Verdict";
import { Vinyl } from "../components/Vinyl";
import { ClipTime, Waveform } from "../components/Waveform";
import { YearCard } from "../components/YearCard";
import { WormLogo } from "../WormLogo";
import { PLAYERS, SONGS } from "./demo";
import {
  PadLive,
  PadOut,
  PadResult,
  StageLive,
  StageOver,
  StageResult,
} from "./PartyScreens";
import { TrainPlay, TrainReveal } from "./TrainingScreens";
import "./gallery.css";

const CLIP = 20;

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="g-section">
      <div className="label">{title}</div>
      {children}
    </section>
  );
}

function Frame({
  kind,
  caption,
  children,
}: {
  kind: "stage" | "portrait" | "pad";
  caption: string;
  children: ReactNode;
}) {
  return (
    <figure className={`g-fig ${kind}`}>
      <div className={`g-frame ${kind}`}>{children}</div>
      <figcaption>{caption}</figcaption>
    </figure>
  );
}

function AudioDemo() {
  const clip = usePlayer();
  const p = clip.player;
  const [song, setSong] = useState(0);
  const s = SONGS[song];
  const start = (i: number) => {
    setSong(i);
    p.unlock();
    p.play(SONGS[i].preview, { from: 0, length: CLIP });
    p.prefetch(SONGS[(i + 1) % SONGS.length].preview);
  };
  const setEngine = (e: Engine) => {
    p.setEngine(e);
    saveEngine(e);
  };
  return (
    <Card>
      <Segmented
        label="Audio engine"
        value={clip.engine}
        onChange={setEngine}
        options={[
          { value: "webaudio", label: "Web Audio" },
          { value: "element", label: "<audio>" },
        ]}
      />
      <Vinyl spinning={clip.playing} />
      <Waveform
        progress={Number.isFinite(clip.total) ? clip.pos / clip.total : 0}
        level={clip.level}
      />
      <ClipTime
        pos={clip.pos}
        total={Number.isFinite(clip.total) ? clip.total : CLIP}
        level={clip.level}
        note={clip.url ? `♪ ${s.title}` : "not started"}
      />
      <div className="player-ctl">
        {clip.playing ? (
          <button type="button" onClick={() => void p.pause()}>
            <Icon name="pause" />
            Pause
          </button>
        ) : (
          <button
            type="button"
            onClick={() => (clip.url ? p.resume() : start(song))}
          >
            <Icon name="play" />
            Play
          </button>
        )}
        <button type="button" onClick={() => start(song)}>
          <Icon name="replay" />
          Replay
        </button>
        <button type="button" onClick={() => start((song + 1) % SONGS.length)}>
          <Icon name="right" />
          Next
        </button>
      </div>
    </Card>
  );
}

function TrainingDemo() {
  const [card, setCard] = useState(0);
  const [last, setLast] = useState<Choice | null>(null);
  const years = [1983, 1991, 1968, 2004];
  return (
    <div>
      <YearCard
        key={card}
        year={years[card % years.length]}
        keyboard
        choices
        onAnswer={(c) => {
          setLast(c);
          setCard((n) => n + 1);
        }}
      />
      <div className="sub center" style={{ marginTop: 10 }}>
        {last
          ? `Answered: ${last}`
          : "Swipe the card, tap a button or press ← space →"}
      </div>
    </div>
  );
}

function TimerDemo() {
  const [run, setRun] = useState(0);
  const [paused, setPaused] = useState(false);
  return (
    <Card>
      <div className="row" style={{ gap: 22 }}>
        <CountdownRing
          key={`r${run}`}
          seconds={15}
          size={104}
          paused={paused}
        />
        <CountdownRing key={`s${run}`} seconds={15} size={64} paused={paused} />
        <div className="row" style={{ gap: 8, flexWrap: "wrap" }}>
          <Button
            small
            variant="ghost"
            icon="replay"
            onClick={() => setRun((n) => n + 1)}
          >
            Restart
          </Button>
          <Button
            small
            variant="ghost"
            icon={paused ? "play" : "pause"}
            onClick={() => setPaused((v) => !v)}
          >
            {paused ? "Resume" : "Pause"}
          </Button>
        </div>
      </div>
      <TimerBar key={`b${run}`} seconds={15} paused={paused} />
    </Card>
  );
}

export default function Gallery() {
  const [tab, setTab] = useState<Tab>("home");
  const [seg, setSeg] = useState("medium");
  const [chips, setChips] = useState<string[]>(["Essentials"]);
  const [av, setAv] = useState<string>("vinyl");
  const toggle = (c: string) =>
    setChips((cs) => (cs.includes(c) ? cs.filter((x) => x !== c) : [...cs, c]));
  const s = SONGS[1];

  return (
    <AppShell tab={tab} onTab={setTab}>
      <div className="row between" style={{ marginTop: 4 }}>
        <div className="row">
          <div className="seal">
            <WormLogo />
          </div>
          <div className="brand">
            Re<span>worm</span>
          </div>
        </div>
        <Pill>component gallery</Pill>
      </div>

      <div className="cols">
        <div className="col">
          <Section title="Buttons">
            <div className="g-stack">
              <Button icon="play">Start training</Button>
              <div className="row">
                <Button small variant="sky" icon="party" style={{ flex: 1 }}>
                  Host a game
                </Button>
                <Button small variant="ghost" icon="qr" style={{ flex: 1 }}>
                  Join
                </Button>
              </div>
              <div className="row">
                <Button variant="ghost">Done</Button>
                <Button disabled>Disabled</Button>
              </div>
              <div className="row">
                <IconButton icon="back" label="Back" />
                <IconButton icon="x" label="Close" />
                <IconButton icon="pause" label="Pause" />
                <Pill>
                  <Icon name="flame" size={16} />
                  <b className="num">12</b> days
                </Pill>
                <Pill>Room K7Q4</Pill>
              </div>
            </div>
          </Section>

          <Section title="Segmented · chips">
            <Segmented
              label="Difficulty"
              value={seg}
              onChange={setSeg}
              options={[
                { value: "easy", label: "Easy" },
                { value: "medium", label: "Medium" },
                { value: "hard", label: "Hard" },
              ]}
            />
            <div className="chips" style={{ marginTop: 12 }}>
              {["Essentials", "Mega Mix", "Rock Anthems", "Schlager Fest"].map(
                (c) => (
                  <Chip
                    key={c}
                    on={chips.includes(c)}
                    onClick={() => toggle(c)}
                    count={308}
                  >
                    {c}
                  </Chip>
                ),
              )}
            </div>
          </Section>

          <Section title="Card · record awards">
            <Card>
              <div className="g-awards">
                {AWARD_TIERS.map((t) => (
                  <AwardBadge key={t} tier={t}>
                    {t[0].toUpperCase() + t.slice(1)} record
                  </AwardBadge>
                ))}
              </div>
            </Card>
          </Section>

          <Section title={`Avatars · ${AVATARS.length}`}>
            <div className="row" style={{ marginBottom: 14 }}>
              <span className="g-me">
                <Avatar avatar={av} size={112} selected />
              </span>
              <div>
                <div style={{ fontSize: 22, fontWeight: 750 }}>Jan</div>
                <div className="sub">
                  {AVATARS.find((a) => a.key === av)?.name}
                </div>
              </div>
            </div>
            <AvatarPicker value={av} onChange={setAv} />
          </Section>
        </div>

        <div className="col">
          <Section title="Listen · audio engines (real previews)">
            <AudioDemo />
          </Section>

          <Section title="Year card · answers">
            <TrainingDemo />
          </Section>

          <Section title="Party pad buttons">
            <AnswerPad variant="pad" onAnswer={() => {}} />
          </Section>

          <Section title="Countdown ring · timer bar">
            <TimerDemo />
          </Section>
        </div>
      </div>

      <div className="cols">
        <div className="col">
          <Section title="Reveal · hit">
            <SongHead
              cover={
                <Cover
                  title={s.title}
                  artist={s.artist}
                  album={s.album}
                  src={s.art}
                  flip
                />
              }
              title={s.title}
              artist={s.artist}
              meta={`Pop · ${s.album}`}
            />
            <Verdict
              result="hit"
              year={1976}
              what="Yes, earlier — 4 years before 1980"
              tier={{ label: "Clear one · 4 yrs off", color: "var(--jade)" }}
              why="+20 XP · streak 3"
              ruler={{ shown: 1980 }}
            />
          </Section>
          <Section title="Reveal · miss, years a step apart">
            <SongHead
              cover={
                <Cover
                  title="Like a Prayer"
                  artist="Madonna"
                  album="Like a Prayer"
                />
              }
              title="Like a Prayer"
              artist="Madonna"
              meta="no cover found: the placeholder"
            />
            <Verdict
              result="miss"
              year={1989}
              what="It's later, not earlier"
              tier={{ label: "Tight one · 1 yr off", color: "var(--gold)" }}
              why="Back in a few cards with a different year."
              ruler={{ shown: 1988 }}
            />
          </Section>
        </div>
        <div className="col">
          <Section title="Reveal · bullseye">
            <Verdict
              result="bullseye"
              year={1975}
              what="Spot on — it's 1975"
              why="+30 XP"
              ruler={{ shown: 1975 }}
            />
          </Section>
          <Section title="Reveal · pin the year">
            <Verdict
              result="hit"
              year={1991}
              what="Within a year"
              ruler={{ shown: 1990, mode: "pin" }}
            />
          </Section>
          <Section title="Covers: image · placeholder · broken image · mini">
            <div
              className="row"
              style={{ alignItems: "flex-end", flexWrap: "wrap" }}
            >
              <Cover
                title={s.title}
                artist={s.artist}
                album={s.album}
                src={s.art}
                size={100}
              />
              <Cover
                title="Superstition"
                artist="Stevie Wonder"
                album="Talking Book"
                size={100}
              />
              <Cover
                title="Bohemian Rhapsody"
                artist="Queen"
                album="A Night at the Opera (2011 Remaster)"
                src="/missing-cover.jpg"
                size={100}
              />
              <Cover
                title="Hey Ya!"
                artist="OutKast"
                album="Speakerboxxx/The Love Below"
                size={56}
                mini
              />
            </div>
          </Section>
        </div>
      </div>

      <Section title="Players: strip (✕ locked out) · standings · podium">
        <PlayerStrip players={PLAYERS} />
        <div className="cols" style={{ marginTop: 16 }}>
          <div className="col">
            <Standings players={PLAYERS} winner="Sarah" />
          </div>
          <div className="col">
            <Podium players={PLAYERS} />
          </div>
        </div>
      </Section>

      <Section title="Training: listen & guess · reveal (two columns, stacked on iPad portrait)">
        <div className="g-frames">
          <Frame kind="stage" caption="Training · listen & guess">
            <TrainPlay />
          </Frame>
          <Frame kind="portrait" caption="Training · listen & guess, 820 px">
            <TrainPlay />
          </Frame>
          <Frame kind="portrait" caption="Training · reveal, 820 px">
            <TrainReveal />
          </Frame>
        </div>
      </Section>

      <Section title="Party stage (iPad / laptop / TV)">
        <div className="g-frames">
          <Frame kind="stage" caption="Stage · round live">
            <StageLive />
          </Frame>
          <Frame kind="stage" caption="Stage · round result">
            <StageResult />
          </Frame>
          <Frame kind="stage" caption="Stage · game over">
            <StageOver />
          </Frame>
        </div>
      </Section>

      <Section title="Party pad (phone)">
        <div className="g-frames pads">
          <Frame kind="pad" caption="Pad · round live">
            <PadLive />
          </Frame>
          <Frame kind="pad" caption="Pad · locked out">
            <PadOut />
          </Frame>
          <Frame kind="pad" caption="Pad · you got it">
            <PadResult won />
          </Frame>
          <Frame kind="pad" caption="Pad · someone was faster">
            <PadResult won={false} />
          </Frame>
        </div>
      </Section>
    </AppShell>
  );
}
