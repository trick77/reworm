// The mock's party screens, composed from the components with demo data: stage (round live, result,
// game over) and pad (live, locked out, won, someone faster). The party PR builds the real ones.
import { Avatar } from "../components/Avatar";
import { AnswerPad, type Choice } from "../components/AnswerPad";
import { AppShell } from "../components/AppShell";
import { Button, Card, IconButton, Pill } from "../components/Controls";
import { CountdownRing, TimerBar } from "../components/Countdown";
import { Cover, SongHead } from "../components/Cover";
import {
  Confetti,
  PlayerStrip,
  Podium,
  Standings,
} from "../components/Players";
import { Verdict } from "../components/Verdict";
import { Vinyl } from "../components/Vinyl";
import { Waveform } from "../components/Waveform";
import { YearCard } from "../components/YearCard";
import { WormLogo } from "../WormLogo";
import { PARTY, PLAYERS } from "./demo";

function StageTop() {
  return (
    <div className="row between stage-top" style={{ marginTop: 4 }}>
      <div className="row">
        <div className="seal" style={{ width: 30, height: 30 }}>
          <WormLogo />
        </div>
        <div className="brand">
          Re<span>worm</span>
        </div>
        <Pill>Room {PARTY.room}</Pill>
      </div>
      <div className="row" style={{ gap: 16 }}>
        <span className="sub">
          Round <b className="num paper">{PARTY.round}</b>
        </span>
        <span className="sub">
          First to <b className="num paper">{PARTY.toWin}</b>
        </span>
        <IconButton icon="pause" label="Pause" />
      </div>
    </div>
  );
}

export function StageLive() {
  const out = PLAYERS.filter((p) => p.out).length;
  return (
    <AppShell scrollClassName="playscr">
      <StageTop />
      <div className="cols mid">
        <div className="col center">
          <Vinyl spinning />
          <Waveform progress={0.45} className="stage-wave" />
          <div className="sub" style={{ marginTop: 6 }}>
            ♪ clip plays here · phones are silent
          </div>
        </div>
        <div className="col center">
          <YearCard
            year={PARTY.shown}
            hint="Earlier · Spot on · Later — answer on your phone"
            className="stage-card"
          />
          <div
            className="row"
            style={{ justifyContent: "center", gap: 22, marginTop: 20 }}
          >
            <CountdownRing seconds={15} size={104} />
            <div style={{ textAlign: "left" }}>
              <div style={{ fontSize: 15, fontWeight: 650 }}>
                {out} locked out
              </div>
              <div className="sub" style={{ fontSize: 13 }}>
                First right answer takes the round
              </div>
            </div>
          </div>
        </div>
      </div>
      <div style={{ marginTop: 8 }}>
        <PlayerStrip players={PLAYERS} />
      </div>
    </AppShell>
  );
}

export function StageResult() {
  const s = PARTY.song;
  const w = PLAYERS.find((p) => p.name === PARTY.winner)!;
  const dir = s.year > PARTY.shown ? "Later" : "Earlier";
  const after = PLAYERS.map((p) =>
    p.name === w.name ? { ...p, score: p.score + PARTY.gain } : p,
  );
  return (
    <AppShell scrollClassName="playscr fade-in">
      <StageTop />
      <div className="cols mid">
        <div className="col">
          <SongHead
            center
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
          />
          <Verdict
            result="hit"
            stamp={false}
            center
            year={s.year}
            what={`${dir} than ${PARTY.shown} — ${Math.abs(s.year - PARTY.shown)} years`}
            ruler={{ shown: PARTY.shown }}
          />
        </div>
        <div className="col center">
          <div className="kata" style={{ color: "var(--jade)" }}>
            Round {PARTY.round} goes to
          </div>
          <div
            className="pop"
            style={{ margin: "12px 0 6px", display: "inline-block" }}
          >
            <Avatar avatar={w.avatar} size={120} />
          </div>
          <div style={{ fontSize: 30, fontWeight: 800 }}>
            {w.name} <span className="num jade">+{PARTY.gain}</span>
          </div>
          <div className="sub">
            said {dir.toLowerCase()} in {PARTY.winTime} s
          </div>
          <div style={{ marginTop: 18 }}>
            <Standings players={after} winner={w.name} />
          </div>
        </div>
      </div>
      <div
        className="row"
        style={{ justifyContent: "center", gap: 8, marginTop: 10 }}
      >
        <Button small>Next round ›</Button>
      </div>
    </AppShell>
  );
}

export function StageOver() {
  const final = PLAYERS.map((p) =>
    p.name === "Mia" ? { ...p, score: 10 } : { ...p, out: false },
  );
  const top = [...final].sort((a, b) => b.score - a.score)[0];
  return (
    <AppShell scrollClassName="playscr fade-in center">
      <StageTop />
      <Confetti />
      <div className="over">
        <div className="kata" style={{ marginTop: 6 }}>
          Game over
        </div>
        <div style={{ fontSize: 34, fontWeight: 800, margin: "4px 0 0" }}>
          {top.name} wins!
        </div>
        <Podium players={final} />
        <div
          className="row"
          style={{ justifyContent: "center", gap: 10, marginTop: 18 }}
        >
          <Button small>Rematch</Button>
          <Button small variant="ghost">
            Back to lobby
          </Button>
        </div>
      </div>
    </AppShell>
  );
}

function PadTop() {
  const me = PLAYERS.find((p) => p.name === PARTY.me)!;
  const rank =
    [...PLAYERS]
      .sort((a, b) => b.score - a.score)
      .findIndex((p) => p.name === me.name) + 1;
  return (
    <div className="row between" style={{ marginTop: 2 }}>
      <div className="row">
        <Avatar avatar={me.avatar} size={34} />
        <b>{me.name}</b>
      </div>
      <Pill className="num">
        {me.score} pt · #{rank}
      </Pill>
    </div>
  );
}

export function PadLive({ onAnswer }: { onAnswer?: (c: Choice) => void }) {
  return (
    <AppShell scrollClassName="column fade-in">
      <PadTop />
      <div className="sub center" style={{ marginTop: 18 }}>
        Round {PARTY.round} · listen to the big screen
      </div>
      <div className="pad-year num">{PARTY.shown}</div>
      <TimerBar seconds={15} />
      <AnswerPad variant="pad" onAnswer={onAnswer ?? (() => {})} />
      <div
        className="sub center"
        style={{ fontSize: 12, margin: "10px 0 4px" }}
      >
        Wrong answer = out for this round
      </div>
    </AppShell>
  );
}

export function PadOut() {
  return (
    <AppShell scrollClassName="column fade-in">
      <PadTop />
      <div className="pad-center">
        <div>
          <div className="pad-out">✕</div>
          <div className="pad-big">Locked out</div>
          <div className="sub" style={{ marginTop: 4 }}>
            Wrong this time — watch the big screen.
            <br />
            Back in next round.
          </div>
        </div>
      </div>
    </AppShell>
  );
}

export function PadResult({ won }: { won: boolean }) {
  const s = PARTY.song;
  return (
    <AppShell scrollClassName="column fade-in">
      <PadTop />
      <div className="pad-center">
        <div>
          {won ? (
            <>
              <div
                className="pop num jade"
                style={{ fontSize: 64, fontWeight: 900 }}
              >
                +1
              </div>
              <div style={{ fontSize: 28, fontWeight: 800 }}>You got it!</div>
              <div className="sub">Later — in 2.1 s, fastest in the room</div>
            </>
          ) : (
            <>
              <div style={{ margin: "0 auto", width: "fit-content" }}>
                <Avatar avatar="disco" size={96} />
              </div>
              <div style={{ fontSize: 24, fontWeight: 800, marginTop: 10 }}>
                Sarah was faster
              </div>
              <div className="sub">She said later in 3.4 s</div>
            </>
          )}
          <Card className="pad-song">
            <Cover
              title={s.title}
              artist={s.artist}
              album={s.album}
              size={56}
              mini
            />
            <div>
              <b>{s.title}</b>
              <div className="sub" style={{ fontSize: 13 }}>
                {s.artist} · <b className="num paper">{s.year}</b>
              </div>
            </div>
          </Card>
        </div>
      </div>
      <div className="sub center" style={{ fontSize: 12.5, marginBottom: 10 }}>
        Next round starts on the big screen
      </div>
    </AppShell>
  );
}
