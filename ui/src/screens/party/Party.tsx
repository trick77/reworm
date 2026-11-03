// Party entry: the Host / Join chooser, /host (creates or resumes this device's room) and /room/:code
// (the lobby, then the stage, on the host device).
import { useEffect, useState } from "react";
import { useLocation } from "wouter";
import { ApiError, type Api } from "../../api/client";
import { toApiError, useApi, useAsync, useMe } from "../../api/context";
import type {
  Decks,
  Difficulty,
  RoomMode,
  RoomSettings,
  RoomState,
} from "../../api/types";
import { useRoom } from "../../api/useRoom";
import { unlockAudio } from "../../audio/unlock";
import { StageScale } from "../../components/AppShell";
import { Avatar } from "../../components/Avatar";
import { Button, Card, IconButton, Segmented } from "../../components/Controls";
import { Icon } from "../../components/Icon";
import { allYears, defaultRoom, load, save } from "../../lib/prefs";
import {
  deckLabel,
  Loading,
  Problem,
  QrCode,
  Screen,
  ScreenHead,
  Unpaired,
} from "../common";
import { DeckPicker } from "../DeckPicker";
import { Stage } from "./Stage";

export const joinUrl = (code: string) =>
  `${window.location.origin}/join/${code}`;

export function Party() {
  const [, go] = useLocation();
  return (
    <Screen tab="party">
      <div className="h-title" style={{ fontSize: 26 }}>
        Game night
      </div>
      <div className="sub">
        One screen plays the song and shows the year. Everyone else answers on
        their phone. Fastest right answer takes the round.
      </div>
      <div className="cols">
        <div className="col">
          <button
            type="button"
            className="opt"
            style={{ marginTop: 18 }}
            onClick={() => go("/host")}
          >
            <div
              className="ic"
              style={{ background: "var(--sky)", color: "#13110f" }}
            >
              <Icon name="party" />
            </div>
            <div>
              <div className="t">Host a game</div>
              <div className="s">
                On the iPad, laptop or TV. It plays the music, shows the year
                and the scoreboard.
              </div>
            </div>
          </button>
          <button type="button" className="opt" onClick={() => go("/join")}>
            <div className="ic">
              <Icon name="qr" />
            </div>
            <div>
              <div className="t">Join a game</div>
              <div className="s">
                Scan the QR on the host screen or type the 4-letter code. Your
                phone becomes the buzzer, no app, no account.
              </div>
            </div>
          </button>
        </div>
        <div className="col">
          <div className="label">How a round works</div>
          <Card className="sub" style={{ fontSize: 13, lineHeight: 1.6 }}>
            <b className="paper">Earlier / Later</b>: the host screen plays a
            clip and shows a year. Everyone taps Earlier, Spot on or Later; the
            fastest right answer scores, a wrong one is locked out for the
            round. Same year for everyone: fair, not personal.
            <br />
            <br />
            <b className="paper">Buzzer</b>: first to buzz stops the music and
            says artist + title. The room judges: 2 for both, 1 for one of them.
          </Card>
        </div>
      </div>
    </Screen>
  );
}

// One room creation at a time: StrictMode's double effect and a double tap must not open two rooms.
let creating: Promise<{ code: string }> | null = null;

function openRoom(api: Api, decks: Decks): Promise<{ code: string }> {
  creating ??= (async () => {
    const hosting = load<string>("hosting");
    if (hosting) {
      try {
        const s = await api.room(hosting);
        if (s.phase !== "over") return { code: s.code };
      } catch {
        // gone: a new one
      }
    }
    const first = decks.sets[0]?.id;
    const settings =
      load<RoomSettings>("room") ??
      defaultRoom(allYears({ sets: first ? [first] : [] }));
    const r = await api.createRoom(settings);
    save("hosting", r.code);
    return r;
  })().finally(() => {
    creating = null;
  });
  return creating;
}

export function Host() {
  const api = useApi();
  const [, go] = useLocation();
  const { me, unpaired } = useMe();
  const decks = useAsync(me ? "decks" : null, () => api.decks());
  const [error, setError] = useState<ApiError | null>(null);
  useEffect(() => {
    if (!decks.data) return;
    let live = true;
    openRoom(api, decks.data).then(
      (r) => live && go(`/room/${r.code}`, { replace: true }),
      (e: unknown) => live && setError(toApiError(e)),
    );
    return () => {
      live = false;
    };
  }, [api, decks.data, go]);
  return (
    <Screen>
      <ScreenHead back="/party" title="Host a game" />
      {unpaired && <Unpaired what="Hosting a game" />}
      {(error ?? decks.error) && <Problem error={(error ?? decks.error)!} />}
      {!unpaired && !error && !decks.error && <Loading />}
    </Screen>
  );
}

const roomSettings = (s: RoomState): RoomSettings => ({
  mode: s.mode,
  toWin: s.toWin,
  deck: s.settings.deck,
  clip: s.settings.clip,
  difficulty: s.settings.difficulty,
});

function Lobby({ state, host }: { state: RoomState; host: boolean }) {
  const api = useApi();
  const decks = useAsync(host ? "decks" : null, () => api.decks());
  const [error, setError] = useState<ApiError | null>(null);
  const [picking, setPicking] = useState(false);
  // draft: the settings sent but not yet echoed; a newer room state replaces it
  const [draft, setDraft] = useState<{ s: RoomSettings; on: RoomState } | null>(
    null,
  );
  const cur = draft?.on === state ? draft.s : roomSettings(state);
  const change = (patch: Partial<RoomSettings>) => {
    const next = { ...cur, ...patch };
    setDraft({ s: next, on: state });
    save("room", next);
    api
      .roomSettings(state.code, next)
      .catch((e: unknown) => setError(toApiError(e)));
  };
  const start = () => {
    unlockAudio(); // inside the tap: the stage plays every round's clip without another one
    setError(null);
    api
      .host(state.code, "start")
      .catch((e: unknown) => setError(toApiError(e)));
  };
  const n = state.players.length;
  return (
    <Screen
      footer={
        host && (
          <Button icon="play" disabled={n === 0} onClick={start}>
            Start game · {n} {n === 1 ? "player" : "players"}
          </Button>
        )
      }
    >
      <ScreenHead back="/party" title="Host a game" />
      <div className="cols">
        <div className="col center">
          <div className="label center">Join on your phone</div>
          <QrCode value={joinUrl(state.code)} />
          <div className="sub" style={{ marginTop: 12 }}>
            or open <b className="paper">{window.location.host}</b> and type
          </div>
          <div className="num room-code" aria-label="Room code">
            {state.code}
          </div>
        </div>
        <div className="col">
          <div className="label">Players · {n}</div>
          <Card style={{ padding: "6px 14px" }}>
            {state.players.map((p) => (
              <div className="set-row" key={p.id}>
                <div className="row">
                  <Avatar avatar={p.avatar} alt="" />
                  <b>{p.name}</b>
                </div>
                <div className="row" style={{ gap: 12 }}>
                  <span className="row sub" style={{ gap: 6, fontSize: 12 }}>
                    <i
                      className="ok-dot"
                      style={
                        p.connected ? undefined : { background: "var(--faint)" }
                      }
                    />
                    {p.connected ? "ready" : "away"}
                  </span>
                  {host && (
                    <IconButton
                      icon="x"
                      label={`Remove ${p.name}`}
                      style={{ width: 32, height: 32 }}
                      onClick={() => {
                        api
                          .kick(state.code, p.id)
                          .catch((e: unknown) => setError(toApiError(e)));
                      }}
                    />
                  )}
                </div>
              </div>
            ))}
            <div className="set-row sub" style={{ fontSize: 13 }}>
              {n >= 8 ? "Full" : "Waiting for more…"} <span>up to 8</span>
            </div>
          </Card>
          {error && <div className="err">{error.message}</div>}
          {host && (
            <>
              <div className="label">Mode</div>
              <Segmented<RoomMode>
                label="Mode"
                value={cur.mode}
                onChange={(mode) => change({ mode })}
                options={[
                  { value: "years", label: "Earlier / Later" },
                  { value: "buzzer", label: "Buzzer" },
                ]}
              />
              <div className="sub" style={{ fontSize: 12.5, marginTop: 8 }}>
                {cur.mode === "years"
                  ? "A year is shown; the fastest right Earlier, Spot on or Later takes the round."
                  : "First buzz stops the clip; say artist + title, the room judges 2 / 1 / 0."}
              </div>
              <div className="label">Game</div>
              <Card style={{ padding: "4px 16px" }}>
                <div className="set-row">
                  <div style={{ minWidth: 0 }}>
                    <div>Songs</div>
                    <div className="m">{deckLabel(cur.deck, decks.data)}</div>
                  </div>
                  <button
                    type="button"
                    className="link"
                    onClick={() => setPicking((v) => !v)}
                  >
                    {picking ? "Done" : "Change"}
                  </button>
                </div>
                {picking && decks.data && (
                  <div style={{ padding: "4px 0 14px" }}>
                    <DeckPicker
                      decks={decks.data}
                      deck={cur.deck}
                      hints={false}
                      onChange={(deck) => change({ deck })}
                    />
                  </div>
                )}
                <div className="set-row">
                  <div>First to</div>
                  <Segmented
                    label="First to"
                    value={String(cur.toWin)}
                    onChange={(v) => change({ toWin: +v })}
                    options={[5, 10, 15].map((v) => ({
                      value: String(v),
                      label: `${v}`,
                    }))}
                  />
                </div>
                <div className="set-row">
                  <div>Clip</div>
                  <Segmented
                    label="Clip length"
                    value={String(cur.clip.length)}
                    onChange={(v) =>
                      change({ clip: { ...cur.clip, length: +v } })
                    }
                    options={[10, 20, 30].map((v) => ({
                      value: String(v),
                      label: `${v} s`,
                    }))}
                  />
                </div>
                <div className="set-row">
                  <div>Clip start</div>
                  <Segmented
                    label="Clip start"
                    value={cur.clip.start}
                    onChange={(start) =>
                      change({ clip: { ...cur.clip, start } })
                    }
                    options={[
                      { value: "chorus", label: "Chorus" },
                      { value: "random", label: "Random" },
                    ]}
                  />
                </div>
                {cur.mode === "years" && (
                  <div className="set-row">
                    <div>Difficulty</div>
                    <Segmented<Difficulty>
                      label="Difficulty"
                      value={cur.difficulty}
                      onChange={(difficulty) => change({ difficulty })}
                      options={[
                        { value: "easy", label: "Easy" },
                        { value: "medium", label: "Medium" },
                        { value: "hard", label: "Hard" },
                      ]}
                    />
                  </div>
                )}
              </Card>
            </>
          )}
          {!host && (
            <div className="sub" style={{ marginTop: 12, fontSize: 13 }}>
              The host's device starts the game.
            </div>
          )}
        </div>
      </div>
      <div style={{ height: 8 }} />
    </Screen>
  );
}

export function Room({ code }: { code: string }) {
  const { me } = useMe();
  const { state, error, live } = useRoom(code);
  const [, go] = useLocation();
  if (error && !state)
    return (
      <Screen>
        <ScreenHead back="/party" title="Host a game" />
        {error.status === 404 ? (
          <Card style={{ marginTop: 18 }} role="alert">
            <div style={{ fontWeight: 700 }}>Room {code} is gone</div>
            <div className="sub" style={{ fontSize: 13, margin: "4px 0 12px" }}>
              Rooms close after a while without play.
            </div>
            <Button
              small
              variant="sky"
              icon="party"
              onClick={() => {
                save("hosting", null);
                go("/host");
              }}
            >
              Host a new game
            </Button>
          </Card>
        ) : (
          <Problem error={error} />
        )}
      </Screen>
    );
  if (!state)
    return (
      <Screen>
        <Loading />
      </Screen>
    );
  const host = !!me && me.profile.id === state.hostId;
  if (state.phase === "lobby") return <Lobby state={state} host={host} />;
  return (
    <StageScale>
      <Stage state={state} host={host} live={live} />
    </StageScale>
  );
}
