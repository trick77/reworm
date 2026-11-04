// The stage: the host device (iPad, laptop on the TV) plays each round's clip and shows the round, the
// result and the final podium. Phones are the pads (Pad.tsx).
import { useEffect, useRef, useState } from "react";
import { useLocation } from "wouter";
import { useApi } from "../../api/context";
import type {
  Phase,
  RoomPlayer,
  RoomState,
  Round,
  RoundResult,
} from "../../api/types";
import { player } from "../../audio/player";
import { audioUnlocked, unlockAudio } from "../../audio/unlock";
import { usePlayer } from "../../audio/usePlayer";
import { AppShell } from "../../components/AppShell";
import { Avatar } from "../../components/Avatar";
import { Button, IconButton, Pill } from "../../components/Controls";
import { DeadlineRing } from "../../components/Countdown";
import { Cover, SongHead } from "../../components/Cover";
import {
  Confetti,
  PlayerStrip,
  Podium,
  Standings,
  type Player,
} from "../../components/Players";
import { Verdict } from "../../components/Verdict";
import { Vinyl } from "../../components/Vinyl";
import { Waveform } from "../../components/Waveform";
import { YearCard } from "../../components/YearCard";
import { save } from "../../lib/prefs";
import { Brand } from "../common";

// The windows (D11, buzzer rules): 15 s to answer, 10 s to say it after a buzz, 20 s to judge. The
// rings count to the server's deadlines; these are only their full length.
export const ROUND_S = 15;
export const ANSWER_S = 10;
export const JUDGE_S = 20;

export const toPlayers = (ps: RoomPlayer[]): Player[] =>
  ps.map((p) => ({
    id: p.id,
    name: p.name,
    avatar: p.avatar,
    score: p.score,
    out: p.state === "out",
  }));

export const PHOTO_MS = 50;

// roundKey identifies a round across a rematch (which counts from 1 again): number + start time.
export const roundKey = (r: Round) => `${r.n}:${r.startedAt}`;

// winTime: " in 3.4 s" when the server sends the winner's time.
export const winTime = (res: RoundResult) =>
  res.winnerMs != null ? ` in ${(res.winnerMs / 1000).toFixed(1)} s` : null;

// roundView: what the round shows. Paused keeps what was on screen: the round has a result → the
// result, unless it is a buzzer round still being judged (its result holds the song, no winner yet).
// `before` is the phase this screen saw before the pause (usePhaseBeforePause); without it (opened while
// paused) a buzzer round with a song but no winner counts as still being judged.
export type RoundView = "live" | "judging" | "result";
export function roundView(s: RoomState, before?: Phase | null): RoundView {
  const r = s.round;
  if (!r) return "live";
  const phase = s.phase === "paused" && before ? before : s.phase;
  if (phase === "result") return "result";
  if (phase === "judging") return r.result ? "judging" : "live";
  if (phase !== "paused" || !r.result) return "live";
  const open = s.mode === "buzzer" && r.buzz && r.result.winnerId == null;
  return open ? "judging" : "result";
}

// usePhaseBeforePause remembers this round's last phase before a pause (what was on screen).
export function usePhaseBeforePause(s: RoomState | undefined): Phase | null {
  const key = s?.round ? roundKey(s.round) : "";
  const [seen, setSeen] = useState<{ key: string; phase: Phase } | null>(null);
  if (
    s &&
    s.phase !== "paused" &&
    (seen?.key !== key || seen.phase !== s.phase)
  )
    setSeen({ key, phase: s.phase });
  return seen?.key === key ? seen.phase : null;
}

// photoLine: a later press in the 1 s window, "+0.18 s", or "photo finish" under 50 ms.
export const photoLine = (deltaMs: number) =>
  deltaMs < PHOTO_MS ? "photo finish" : `+${(deltaMs / 1000).toFixed(2)} s`;

export function direction(shown: number, year: number) {
  return year === shown ? "exact" : year > shown ? "later" : "earlier";
}

// useStageAudio plays each new round's clip (fade in), pauses with the room, fades out on a buzz and on
// the result, and keeps the next round's clip preloaded.
function useStageAudio(s: RoomState) {
  const played = useRef<string | null>(null);
  const prev = useRef<string | null>(null);
  const n = s.round ? roundKey(s.round) : null;
  const url = s.round?.clip.previewUrl;
  const from = s.round?.clip.from ?? 0;
  const length = s.round?.clip.length;
  const next = s.nextClip?.previewUrl;
  useEffect(() => {
    const p = player();
    const was = prev.current;
    prev.current = s.phase;
    if (n == null || !url) return;
    if (s.phase === "playing") {
      if (played.current !== n) {
        played.current = n;
        p.play(url, { from, length });
      } else if (was === "paused") p.resume();
    } else if (s.phase === "paused") void p.pause();
    else void p.fadeOut();
  }, [s.phase, n, url, from, length]);
  useEffect(() => {
    if (next) player().prefetch(next);
  }, [next]);
  useEffect(() => () => void player().fadeOut(), []);
}

function StageTop({
  s,
  host,
  live,
}: {
  s: RoomState;
  host: boolean;
  live: boolean;
}) {
  const api = useApi();
  const paused = s.phase === "paused";
  const canPause = host && s.round && s.phase !== "over";
  return (
    <div className="row between stage-top" style={{ marginTop: 4 }}>
      <div className="row">
        <Brand small />
        <Pill>Room {s.code}</Pill>
        {!live && <Pill style={{ color: "var(--gold)" }}>Reconnecting…</Pill>}
      </div>
      <div className="row" style={{ gap: 16 }}>
        {s.round && (
          <span className="sub">
            Round <b className="num paper">{s.round.n}</b>
          </span>
        )}
        <span className="sub">
          First to <b className="num paper">{s.toWin}</b>
        </span>
        {canPause && (
          <IconButton
            icon={paused ? "play" : "pause"}
            label={paused ? "Resume" : "Pause"}
            onClick={() =>
              void api.host(s.code, paused ? "resume" : "pause").catch(() => {})
            }
          />
        )}
      </div>
    </div>
  );
}

function Clip({ spinning }: { spinning: boolean }) {
  const clip = usePlayer();
  return (
    <div className="col center">
      <Vinyl spinning={spinning} />
      <Waveform
        progress={clip.pos / clip.total}
        level={clip.level}
        className="stage-wave"
      />
      <div className="sub" style={{ marginTop: 6 }}>
        ♪ clip plays here · phones are silent
      </div>
    </div>
  );
}

function VoteChips({
  ids,
  players,
  label,
}: {
  ids: string[];
  players: RoomPlayer[];
  label: (p: RoomPlayer) => string;
}) {
  return (
    <div className="votes">
      {ids.map((id) => {
        const p = players.find((x) => x.id === id);
        if (!p) return null;
        return (
          <span className="vote in" key={id}>
            <Avatar avatar={p.avatar} size={26} alt="" />
            {label(p)}
          </span>
        );
      })}
    </div>
  );
}

function YearsLive({ s }: { s: RoomState }) {
  const r = s.round!;
  const out = s.players.filter((p) => p.state === "out").length;
  return (
    <div className="col center">
      <YearCard
        year={r.proposedYear ?? "?"}
        hint="Earlier · Spot on · Later — answer on your phone"
        className="stage-card"
      />
      <div
        className="row"
        style={{ justifyContent: "center", gap: 22, marginTop: 20 }}
      >
        <DeadlineRing
          endsAt={r.endsAt}
          total={ROUND_S}
          size={104}
          paused={s.phase === "paused"}
        />
        <div style={{ textAlign: "left" }}>
          <div style={{ fontSize: 15, fontWeight: 650 }}>{out} locked out</div>
          <div className="sub" style={{ fontSize: 13 }}>
            First right answer takes the round
          </div>
        </div>
      </div>
    </div>
  );
}

function BuzzerLive({ s }: { s: RoomState }) {
  const r = s.round!;
  const buzzer = r.buzz && s.players.find((p) => p.id === r.buzz!.playerId);
  if (r.buzz && buzzer) {
    return (
      <div className="col center">
        <div className="pop" style={{ display: "inline-block" }}>
          <Avatar avatar={buzzer.avatar} size={110} />
        </div>
        <div className="stage-big" style={{ marginTop: 10 }}>
          {buzzer.name} buzzed — say artist + title!
        </div>
        <div
          className="row"
          style={{ justifyContent: "center", gap: 22, marginTop: 18 }}
        >
          <DeadlineRing
            endsAt={r.buzz.answerEndsAt}
            total={ANSWER_S}
            size={96}
            paused={s.phase === "paused"}
          />
          <div style={{ textAlign: "left" }}>
            <div style={{ fontSize: 15, fontWeight: 650 }}>
              Reveal · {r.revealVotes.length} pressed
            </div>
            <div className="sub" style={{ fontSize: 13 }}>
              {buzzer.name} and one more press Reveal on their phone
            </div>
          </div>
        </div>
        <VoteChips
          ids={r.revealVotes}
          players={s.players}
          label={(p) => p.name}
        />
      </div>
    );
  }
  return (
    <div className="col center">
      <div className="kata">Buzzer round</div>
      <div className="stage-big" style={{ margin: "8px 0 4px" }}>
        Know it? Buzz!
      </div>
      <div className="sub">
        First buzz stops the music — then say artist + title
      </div>
      <div className="row" style={{ justifyContent: "center", marginTop: 22 }}>
        <DeadlineRing
          endsAt={r.endsAt}
          total={ROUND_S}
          size={104}
          paused={s.phase === "paused"}
        />
      </div>
    </div>
  );
}

const JUDGE_LABEL = ["neither", "one of them", "artist + title"];

function SongCol({
  s,
  res,
  judging,
}: {
  s: RoomState;
  res: RoundResult;
  judging: boolean;
}) {
  const api = useApi();
  const song = res.song;
  const shown = s.round?.proposedYear;
  const years = s.mode === "years" && shown != null;
  const dir = years ? direction(shown, song.year) : null;
  return (
    <div className="col">
      <SongHead
        center
        cover={
          <Cover
            key={song.id}
            title={song.title}
            artist={song.artist}
            album={song.title}
            src={api.coverUrl(song.id)}
            flip
          />
        }
        title={song.title}
        artist={song.artist}
      />
      <Verdict
        result={res.winnerId || judging ? "hit" : "miss"}
        stamp={false}
        center
        year={song.year}
        what={
          !years
            ? `Released ${song.year}`
            : dir === "exact"
              ? `Spot on — it's ${shown}`
              : `${dir === "later" ? "Later" : "Earlier"} than ${shown} — ${Math.abs(song.year - shown)} year${Math.abs(song.year - shown) > 1 ? "s" : ""}`
        }
        ruler={years ? { shown } : undefined}
      />
    </div>
  );
}

function Judging({ s }: { s: RoomState }) {
  const r = s.round!;
  const buzzer = s.players.find((p) => p.id === r.buzz?.playerId);
  const votes = Object.keys(r.judgeVotes);
  return (
    <div className="col center">
      <div className="kata">Judging</div>
      <div className="stage-big" style={{ margin: "8px 0 6px" }}>
        Did {buzzer?.name ?? "they"} get it?
      </div>
      <div className="sub">
        On the phones: 2 = artist + title · 1 = one of them · 0 = neither
      </div>
      <VoteChips
        ids={votes}
        players={s.players}
        label={(p) => `${p.name}: ${r.judgeVotes[p.id]}`}
      />
      {r.judgeEndsAt != null && (
        <div
          className="row"
          style={{ justifyContent: "center", marginTop: 14 }}
        >
          <DeadlineRing
            endsAt={r.judgeEndsAt}
            total={JUDGE_S}
            size={72}
            paused={s.phase === "paused"}
          />
        </div>
      )}
      <div className="sub" style={{ marginTop: 12, fontSize: 13 }}>
        {buzzer?.name ?? "The buzzer"} and one more must agree; otherwise the
        room's middle vote decides.
      </div>
    </div>
  );
}

function Winner({ s, res }: { s: RoomState; res: RoundResult }) {
  const w = s.players.find((p) => p.id === res.winnerId);
  const shown = s.round?.proposedYear;
  const photo = res.photoFinish
    .map((f) => ({ ...f, p: s.players.find((x) => x.id === f.playerId) }))
    .filter((f) => f.p);
  return (
    <div className="col center">
      {w ? (
        <>
          <div className="kata" style={{ color: "var(--jade)" }}>
            Round {s.round!.n} goes to
          </div>
          <div
            className="pop"
            style={{ margin: "12px 0 6px", display: "inline-block" }}
          >
            <Avatar avatar={w.avatar} size={120} />
          </div>
          <div style={{ fontSize: 30, fontWeight: 800 }}>
            {w.name} <span className="num jade">+{res.points}</span>
          </div>
          <div className="sub">
            {s.mode === "years" && shown != null
              ? direction(shown, res.song.year) === "exact"
                ? "said spot on — bonus point"
                : `said ${direction(shown, res.song.year)}${winTime(res) ?? ", fastest in the room"}`
              : `knew ${JUDGE_LABEL[res.points] ?? "it"}`}
          </div>
        </>
      ) : (
        <>
          <div className="kata" style={{ color: "var(--rose)" }}>
            Round {s.round!.n}
          </div>
          <div style={{ fontSize: 30, fontWeight: 800, margin: "10px 0 4px" }}>
            Nobody got it
          </div>
          <div className="sub">No points this round</div>
        </>
      )}
      {photo.length > 0 && (
        <div className="photo">
          {photo.some((f) => f.deltaMs < PHOTO_MS) && <b>Photo finish! </b>}
          {photo.map((f) => `${f.p!.name} ${photoLine(f.deltaMs)}`).join(" · ")}
        </div>
      )}
      <div style={{ marginTop: 18 }}>
        <Standings
          players={toPlayers(s.players)}
          winnerId={res.winnerId ?? undefined}
        />
      </div>
    </div>
  );
}

function Over({ s, host }: { s: RoomState; host: boolean }) {
  const api = useApi();
  const [, go] = useLocation();
  const top = [...s.players].sort((a, b) => b.score - a.score)[0];
  return (
    <>
      <Confetti />
      <div className="over center">
        <div className="kata" style={{ marginTop: 6 }}>
          Game over
        </div>
        <div style={{ fontSize: 34, fontWeight: 800, margin: "4px 0 0" }}>
          {top ? `${top.name} wins!` : "Game over"}
        </div>
        <Podium
          players={toPlayers(s.players).map((p) => ({ ...p, out: false }))}
        />
        <div
          className="row"
          style={{ justifyContent: "center", gap: 10, marginTop: 18 }}
        >
          {host && (
            <Button
              small
              onClick={() => {
                unlockAudio();
                void api.host(s.code, "rematch").catch(() => {});
              }}
            >
              Rematch
            </Button>
          )}
          <Button
            small
            variant="ghost"
            onClick={() => {
              save("hosting", null);
              go("/");
            }}
          >
            Done
          </Button>
        </div>
      </div>
    </>
  );
}

export function Stage({
  state: s,
  host,
  live,
}: {
  state: RoomState;
  host: boolean;
  live: boolean;
}) {
  const api = useApi();
  useStageAudio(s);
  const [sound, setSound] = useState(audioUnlocked);
  const r = s.round;
  const res = r?.result;
  const view = roundView(s, usePhaseBeforePause(s));
  const judging = view === "judging";
  const showResult = view === "result";
  const enableSound = () => {
    unlockAudio();
    setSound(true);
    if (s.phase === "playing" && r)
      player().play(r.clip.previewUrl, {
        from: r.clip.from,
        length: r.clip.length,
      });
  };

  let body;
  if (s.phase === "over") body = <Over s={s} host={host} />;
  else if (!r)
    body = <div className="over center sub">Waiting for the next round…</div>;
  else if (showResult || judging)
    body = (
      <div className="cols mid">
        <SongCol s={s} res={res!} judging={judging} />
        {judging ? <Judging s={s} /> : <Winner s={s} res={res!} />}
      </div>
    );
  else
    body = (
      <>
        <div className="cols mid">
          <Clip spinning={s.phase === "playing"} />
          {s.mode === "years" ? <YearsLive s={s} /> : <BuzzerLive s={s} />}
        </div>
        <div style={{ marginTop: 8 }}>
          <PlayerStrip players={toPlayers(s.players)} />
        </div>
      </>
    );

  return (
    <AppShell
      scrollClassName={`playscr fade-in${s.phase === "over" ? " center" : ""}`}
    >
      <StageTop s={s} host={host} live={live} />
      {body}
      {host && s.phase === "result" && (
        <div
          className="row"
          style={{ justifyContent: "center", gap: 8, marginTop: 10 }}
        >
          <Button
            small
            onClick={() => void api.host(s.code, "next").catch(() => {})}
          >
            Next round ›
          </Button>
        </div>
      )}
      {s.phase === "paused" && (
        <div className="overlay">
          <div>
            <div className="stage-big">Paused</div>
            <div className="sub" style={{ margin: "6px 0 16px" }}>
              Clip and timer stop for everyone
            </div>
            {host && (
              <Button
                small
                icon="play"
                onClick={() => void api.host(s.code, "resume").catch(() => {})}
              >
                Resume
              </Button>
            )}
          </div>
        </div>
      )}
      {host && !sound && s.phase !== "over" && s.phase !== "paused" && (
        <div className="overlay">
          <div>
            <div className="stage-big">Sound is off</div>
            <div className="sub" style={{ margin: "6px 0 16px" }}>
              The browser needs one tap before this screen may play the clips.
            </div>
            <Button small icon="play" onClick={enableSound}>
              Turn sound on
            </Button>
          </div>
        </div>
      )}
    </AppShell>
  );
}
