// The pad: a phone in a party room. No audio: the stage plays. Years mode is three big buttons; buzzer
// mode one BUZZ button, then Reveal and the 2 / 1 / 0 judging.
import { useEffect, useState, type ReactNode } from "react";
import { ApiError } from "../../api/client";
import { useApi } from "../../api/context";
import type {
  Points,
  RoomPlayer,
  RoomState,
  Round,
  RoundResult,
} from "../../api/types";
import { useRoom } from "../../api/useRoom";
import { AnswerPad } from "../../components/AnswerPad";
import { Avatar } from "../../components/Avatar";
import { Button, Card, Pill } from "../../components/Controls";
import { DeadlineBar, DeadlineRing } from "../../components/Countdown";
import { Cover } from "../../components/Cover";
import { Loading, Problem, Screen } from "../common";
import {
  ANSWER_S,
  direction,
  PHOTO_MS,
  ROUND_S,
  roundKey,
  roundView,
  winTime,
  type RoundView,
} from "./Stage";

const ord = (n: number) =>
  n +
  (n % 10 === 1 && n !== 11
    ? "st"
    : n % 10 === 2 && n !== 12
      ? "nd"
      : n % 10 === 3 && n !== 13
        ? "rd"
        : "th");

// rankOf: 1 + everyone with more points (ties share a place).
const rankOf = (s: RoomState, id: string) => {
  const mine = s.players.find((p) => p.id === id)?.score ?? 0;
  return 1 + s.players.filter((p) => p.score > mine).length;
};

function PadTop({ s, me }: { s: RoomState; me: RoomPlayer }) {
  return (
    <div className="row between" style={{ marginTop: 2 }}>
      <div className="row">
        <Avatar avatar={me.avatar} size={34} />
        <b>{me.name}</b>
      </div>
      <Pill className="num">
        {me.score} pt · #{rankOf(s, me.id)}
      </Pill>
    </div>
  );
}

function Center({ children }: { children: ReactNode }) {
  return (
    <div className="pad-center">
      <div>{children}</div>
    </div>
  );
}

function SongCard({ res }: { res: RoundResult }) {
  const api = useApi();
  const s = res.song;
  return (
    <Card className="pad-song">
      <Cover
        title={s.title}
        artist={s.artist}
        album={s.title}
        src={api.coverUrl(s.id)}
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
  );
}

const JUDGE: { points: Points; label: string }[] = [
  { points: 2, label: "artist + title" },
  { points: 1, label: "one of them" },
  { points: 0, label: "neither" },
];

function Result({ s, me }: { s: RoomState; me: RoomPlayer }) {
  const r = s.round!;
  const res = r.result!;
  const w = s.players.find((p) => p.id === res.winnerId);
  const shown = r.proposedYear;
  const said =
    s.mode === "years" && shown != null
      ? direction(shown, res.song.year)
      : null;
  const mine = res.photoFinish.find((f) => f.playerId === me.id);
  let head: ReactNode;
  if (w && w.id === me.id)
    head = (
      <>
        <div className="pop num jade" style={{ fontSize: 64, fontWeight: 900 }}>
          +{res.points}
        </div>
        <div style={{ fontSize: 28, fontWeight: 800 }}>You got it!</div>
        <div className="sub">
          {said
            ? said === "exact"
              ? "Spot on — bonus point"
              : `${said === "later" ? "Later" : "Earlier"}${winTime(res) ?? " — fastest in the room"}`
            : res.points === 2
              ? "Artist + title"
              : "One of them"}
        </div>
      </>
    );
  else if (w)
    head = (
      <>
        <div style={{ margin: "0 auto", width: "fit-content" }}>
          <Avatar avatar={w.avatar} size={96} />
        </div>
        <div style={{ fontSize: 24, fontWeight: 800, marginTop: 10 }}>
          {w.name} was faster
        </div>
        <div className="sub">
          {mine
            ? mine.deltaMs < PHOTO_MS
              ? "Photo finish! You were right behind."
              : `You were ${(mine.deltaMs / 1000).toFixed(2)} s behind`
            : said
              ? `${w.name} said ${said === "exact" ? "spot on" : said}`
              : `${w.name} knew it`}
        </div>
      </>
    );
  else
    head = (
      <>
        <div style={{ fontSize: 26, fontWeight: 800 }}>Nobody got it</div>
        <div className="sub">No points this round</div>
      </>
    );
  return (
    <>
      <Center>
        {head}
        <SongCard res={res} />
      </Center>
      <div className="sub center" style={{ fontSize: 12.5, marginBottom: 10 }}>
        Next round starts on the big screen
      </div>
    </>
  );
}

function Over({ s, me }: { s: RoomState; me: RoomPlayer }) {
  const rank = rankOf(s, me.id);
  return (
    <Center>
      <div className="kata">Game over</div>
      <div
        className="pop num"
        style={{
          fontSize: 72,
          fontWeight: 900,
          color: rank === 1 ? "var(--gold)" : undefined,
        }}
      >
        #{rank}
      </div>
      <div style={{ fontSize: 24, fontWeight: 800 }}>
        {rank === 1 ? "You win!" : `You finished ${ord(rank)}`}
      </div>
      <div className="sub">
        {me.score} {me.score === 1 ? "point" : "points"} · a rematch starts on
        the big screen
      </div>
    </Center>
  );
}

// LateNotice: joined mid-round, this round is sat out.
function LateNotice() {
  return (
    <Center>
      <div className="pad-big">You're in from the next round</div>
      <div className="sub" style={{ marginTop: 4 }}>
        Watch the big screen meanwhile.
      </div>
    </Center>
  );
}

// sent: this pad's last press, per round (roundKey: a rematch's round 1 is a new one), so a double tap
// goes up once and the buttons dim at once.
type Sent = { key: string; what: string } | null;
const sentIn = (sent: Sent, r: Round) =>
  sent?.key === roundKey(r) ? sent.what : null;

function YearsLive({
  s,
  me,
  sent,
  send,
}: {
  s: RoomState;
  me: RoomPlayer;
  sent: Sent;
  send: (what: string, f: () => Promise<void>) => void;
}) {
  const api = useApi();
  const r = s.round!;
  const mine = sentIn(sent, r);
  if (me.state === "late") return <LateNotice />;
  if (me.state === "out")
    return (
      <Center>
        <div className="pad-out">✕</div>
        <div className="pad-big">Locked out</div>
        <div className="sub" style={{ marginTop: 4 }}>
          Wrong this time — watch the big screen.
          <br />
          Back in next round.
        </div>
      </Center>
    );
  const done = me.state === "answered" || !!mine;
  return (
    <>
      <div className="sub center" style={{ marginTop: 18 }}>
        Round {r.n} · listen to the big screen
      </div>
      <div className="pad-year num">{r.proposedYear}</div>
      <DeadlineBar
        endsAt={r.endsAt}
        total={ROUND_S}
        paused={s.phase === "paused"}
      />
      <AnswerPad
        variant="pad"
        disabled={done || s.phase !== "playing"}
        onAnswer={(c) => send(c, () => api.answer(s.code, r.n, c))}
      />
      <div
        className="sub center"
        style={{ fontSize: 12, margin: "10px 0 4px" }}
      >
        {done
          ? "Answer in — watch the big screen"
          : "Wrong answer = out for this round"}
      </div>
    </>
  );
}

function BuzzerLive({
  s,
  me,
  sent,
  send,
  view,
}: {
  s: RoomState;
  me: RoomPlayer;
  sent: Sent;
  send: (what: string, f: () => Promise<void>) => void;
  view: RoundView;
}) {
  const api = useApi();
  const r = s.round!;
  const buzz = r.buzz;
  const mine = sentIn(sent, r);
  if (view === "judging" && r.result && buzz) {
    const buzzer = s.players.find((p) => p.id === buzz.playerId);
    const voted = r.judgeVotes[me.id] != null || mine === "judge";
    return (
      <>
        <div className="sub center" style={{ marginTop: 18 }}>
          {buzz.playerId === me.id
            ? "How did you do?"
            : `How did ${buzzer?.name ?? "they"} do?`}
        </div>
        <SongCard res={r.result} />
        {voted ? (
          <Center>
            <div className="pad-big">Thanks!</div>
            <div className="sub">Waiting for the others to judge…</div>
          </Center>
        ) : (
          <div className="judge" role="group" aria-label="Judge">
            {JUDGE.map((j) => (
              <button
                key={j.points}
                type="button"
                className={`p${j.points}`}
                onClick={() =>
                  send("judge", () => api.judge(s.code, r.n, j.points))
                }
              >
                <b>{j.points}</b>
                {j.label}
              </button>
            ))}
          </div>
        )}
      </>
    );
  }
  if (buzz) {
    const isBuzzer = buzz.playerId === me.id;
    const buzzer = s.players.find((p) => p.id === buzz.playerId);
    const pressed = r.revealVotes.includes(me.id) || mine === "reveal";
    return (
      <>
        <Center>
          {isBuzzer ? (
            <>
              <div className="pad-big" style={{ marginTop: 0 }}>
                Say artist + title!
              </div>
              <div
                className="row"
                style={{ justifyContent: "center", margin: "16px 0" }}
              >
                <DeadlineRing
                  endsAt={buzz.answerEndsAt}
                  total={ANSWER_S}
                  size={120}
                  paused={s.phase === "paused"}
                />
              </div>
              <div className="sub">
                Out loud, to the room. Then press Reveal.
              </div>
            </>
          ) : (
            <>
              <div style={{ margin: "0 auto", width: "fit-content" }}>
                <Avatar avatar={buzzer?.avatar ?? "vinyl"} size={96} />
              </div>
              <div className="pad-big">{buzzer?.name ?? "Someone"} buzzed</div>
              <div className="sub">
                Listen to the answer, then press Reveal.
              </div>
            </>
          )}
        </Center>
        <Button
          variant={pressed ? "ghost" : "primary"}
          disabled={pressed || s.phase !== "buzzed"}
          onClick={() => send("reveal", () => api.reveal(s.code, r.n))}
          style={{ marginBottom: 6 }}
        >
          {pressed ? "Waiting for one more Reveal…" : "Reveal the song"}
        </Button>
        <div
          className="sub center"
          style={{ fontSize: 12, margin: "4px 0 6px" }}
        >
          Reveal · {r.revealVotes.length} pressed
        </div>
      </>
    );
  }
  if (me.state === "late") return <LateNotice />;
  const done = mine === "buzz";
  return (
    <>
      <div className="sub center" style={{ marginTop: 18 }}>
        Round {r.n} · listen to the big screen
      </div>
      <DeadlineBar
        endsAt={r.endsAt}
        total={ROUND_S}
        paused={s.phase === "paused"}
      />
      <button
        type="button"
        className="buzz"
        disabled={done || s.phase !== "playing"}
        onClick={() => send("buzz", () => api.buzz(s.code, r.n))}
      >
        BUZZ
      </button>
      <div
        className="sub center"
        style={{ fontSize: 12, margin: "10px 0 4px" }}
      >
        Know it? Buzz, then say artist + title.
      </div>
    </>
  );
}

export function Pad({
  code,
  playerId,
  onGone,
}: {
  code: string;
  playerId: string;
  onGone: () => void;
}) {
  const { state: s, error, live } = useRoom(code);
  const [sent, setSent] = useState<Sent>(null);
  const me = s?.players.find((p) => p.id === playerId);
  const gone = (!!s && !me) || error?.status === 404;
  useEffect(() => {
    if (gone) onGone();
  }, [gone, onGone]);

  if (error && !s)
    return (
      <Screen>
        <Problem error={error} />
      </Screen>
    );
  if (!s || !me)
    return (
      <Screen>
        <Loading />
      </Screen>
    );

  const r = s.round;
  const send = (what: string, f: () => Promise<void>) => {
    if (!r || sentIn(sent, r) === what) return;
    const press = { key: roundKey(r), what };
    setSent(press);
    f().catch((e: unknown) => {
      // a deliberate refusal (409: too late, out, late) stands, the room state says what happened;
      // a failed request (network, 5xx) frees the button to try again
      if (e instanceof ApiError && e.status === 409) return;
      setSent((cur) => (cur === press ? null : cur));
    });
  };
  const view = roundView(s);

  let body: ReactNode;
  if (s.phase === "lobby")
    body = (
      <Center>
        <div className="pop" style={{ display: "inline-block" }}>
          <Avatar avatar={me.avatar} size={110} />
        </div>
        <div style={{ fontSize: 26, fontWeight: 800, marginTop: 10 }}>
          You're in!
        </div>
        <div className="sub">
          Waiting for the host to start · {s.players.length}{" "}
          {s.players.length === 1 ? "player" : "players"}
        </div>
        <div
          className="stack"
          style={{ justifyContent: "center", marginTop: 14 }}
        >
          {s.players.map((p) => (
            <Avatar key={p.id} avatar={p.avatar} size={34} alt={p.name} />
          ))}
        </div>
        <div className="sub" style={{ marginTop: 18, fontSize: 12.5 }}>
          {s.mode === "years"
            ? "Earlier · Spot on · Later"
            : "Buzzer: say artist + title"}{" "}
          · first to {s.toWin}
        </div>
      </Center>
    );
  else if (s.phase === "over") body = <Over s={s} me={me} />;
  else if (!r)
    body = (
      <Center>
        <div className="sub">Waiting for the next round…</div>
      </Center>
    );
  else if (view === "result") body = <Result s={s} me={me} />;
  else if (s.mode === "years")
    body = <YearsLive s={s} me={me} sent={sent} send={send} />;
  else body = <BuzzerLive s={s} me={me} sent={sent} send={send} view={view} />;

  return (
    <Screen className="column">
      <PadTop s={s} me={me} />
      {!live && (
        <div
          className="sub center"
          style={{ fontSize: 12, color: "var(--gold)" }}
        >
          Reconnecting…
        </div>
      )}
      {s.phase === "paused" && (
        <div
          className="sub center"
          style={{ marginTop: 10, color: "var(--gold)" }}
        >
          Paused by the host
        </div>
      )}
      {body}
    </Screen>
  );
}
