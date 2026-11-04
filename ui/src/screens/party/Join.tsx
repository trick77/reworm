// /join/:code? — the phone's way in: the room code (from the QR or typed), a name and an avatar. Once
// joined, the same route is the pad.
import { useEffect, useState } from "react";
import { useLocation } from "wouter";
import { BACKOFF_MAX_MS, BACKOFF_MS, type ApiError } from "../../api/client";
import { toApiError, useApi, useMe } from "../../api/context";
import { avatarInfo } from "../../avatars";
import { Button } from "../../components/Controls";
import { load, playerKey, save } from "../../lib/prefs";
import {
  AvatarStrip,
  CodeBoxes,
  Loading,
  Problem,
  Screen,
  ScreenHead,
} from "../common";
import { Pad } from "./Pad";

type Guest = { name: string; avatar: string };

// JOIN_ERRORS: the refusals a joining phone gets, by the server's code; anything else shows its message.
const JOIN_ERRORS: Record<string, string> = {
  room_full: "This game is full (8 players).",
  kicked: "The host removed you from this game.",
  name_taken: "That name is taken in this game. Pick another one.",
  over: "This game is over.",
  game_over: "This game is over.",
  invalid_name: "Pick a name of 1 to 20 characters.",
  invalid_avatar: "Pick one of the avatars.",
};

function joinError(e: ApiError, code: string): string {
  if (e.status === 404) return `No game with the code ${code}.`;
  return JOIN_ERRORS[e.code] ?? e.message;
}

function JoinForm({
  code: given,
  note,
  onJoined,
}: {
  code?: string;
  note?: string;
  onJoined: (code: string, playerId: string) => void;
}) {
  const api = useApi();
  const { me } = useMe();
  const saved = load<Guest>("guest");
  const [code, setCode] = useState(given ?? "");
  // null: untouched, so the profile (loaded later) or the last guest name fills in
  const [typed, setName] = useState<string | null>(null);
  const [picked, setAvatar] = useState<string | null>(null);
  const name = typed ?? me?.profile.name ?? saved?.name ?? "";
  const avatar = picked ?? me?.profile.avatar ?? saved?.avatar ?? "mic";
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(note ?? null);
  const ready = code.length === 4 && name.trim().length > 0 && !busy;

  const join = () => {
    if (!ready) return;
    setBusy(true);
    setError(null);
    const guest = { name: name.trim(), avatar };
    api.joinRoom(code, guest).then(
      (r) => {
        save("guest", guest);
        onJoined(code, r.playerId);
      },
      (e: unknown) => {
        setError(joinError(toApiError(e), code));
        setBusy(false);
      },
    );
  };

  return (
    <Screen
      footer={
        <Button disabled={!ready} onClick={join}>
          {busy ? "Joining…" : "Join game"}
        </Button>
      }
    >
      <ScreenHead back="/party" title="Join a game" />
      <div style={{ maxWidth: 420, margin: "0 auto", textAlign: "center" }}>
        <div className="sub" style={{ margin: "34px 0 8px" }}>
          Code on the host screen
        </div>
        {given ? (
          <div className="codebox" aria-label={`Room ${given}`}>
            {given.split("").map((ch, i) => (
              <div key={i} className="b num">
                {ch}
              </div>
            ))}
          </div>
        ) : (
          <CodeBoxes
            value={code}
            onChange={setCode}
            length={4}
            label="Room code"
            onDone={join}
          />
        )}
        <div className="sub" style={{ margin: "28px 0 8px" }}>
          Your name
        </div>
        <input
          className="input"
          aria-label="Your name"
          value={name}
          maxLength={20}
          autoComplete="nickname"
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && join()}
          style={{ textAlign: "center", maxWidth: 260 }}
        />
        <div className="sub" style={{ margin: "22px 0 10px" }}>
          Pick your avatar
        </div>
        <AvatarStrip value={avatar} onChange={setAvatar} />
        <div className="sub" style={{ fontSize: 12 }}>
          {avatarInfo(avatar).name}
        </div>
        {error && (
          <div className="err" role="alert">
            {error}
          </div>
        )}
        <div className="sub" style={{ marginTop: 22, fontSize: 12.5 }}>
          No account needed. Your phone won't play music — the host screen does.
        </div>
      </div>
    </Screen>
  );
}

// handed: players just joined on /join, handed to the /join/CODE screen that opens next, which then needs
// no rooms/{code}/me of its own.
const handed = new Map<string, string>();

type Asked = { code: string; id: string | null; refused?: ApiError };

// useServerPlayer asks rooms/{code}/me for this browser's player (guest cookie or profile), the room's word
// over any stored id. A failed ask (network, 5xx) is tried again with a backoff; a kicked player is refused.
function useServerPlayer(code: string | null) {
  const api = useApi();
  const [asked, setAsked] = useState<Asked | null>(null);
  const [failed, setFailed] = useState<{ code: string; e: ApiError } | null>(
    null,
  );
  useEffect(() => {
    if (!code) return;
    let live = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;
    const ask = () =>
      api.roomMe(code).then(
        (id) => live && setAsked({ code, id }),
        (e: unknown) => {
          if (!live) return;
          const err = toApiError(e);
          if (err.status === 403)
            return setAsked({ code, id: null, refused: err });
          setFailed({ code, e: err });
          timer = setTimeout(
            () => void ask(),
            Math.min(BACKOFF_MAX_MS, BACKOFF_MS * 2 ** attempt++),
          );
        },
      );
    void ask();
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [api, code]);
  const mine = asked?.code === code ? asked : null;
  return {
    answered: !!mine,
    id: mine?.id ?? null,
    refused: mine?.refused,
    error: !mine && failed?.code === code ? failed.e : undefined,
  };
}

export function Join({ code: raw }: { code?: string }) {
  const [, go] = useLocation();
  const code = raw?.toUpperCase();
  // joined: the player this page (or /join just before it) joined as: confirmed, no need to ask
  const [joined, setJoined] = useState<string | null>(() =>
    code ? (handed.get(code) ?? null) : null,
  );
  useEffect(() => {
    if (code) handed.delete(code);
  }, [code]);
  // note: why the form is back (the pad lost its player)
  const [note, setNote] = useState<string | undefined>();
  // guessed: the stored id led nowhere; wait for the server's answer instead
  const [guessed, setGuessed] = useState(false);
  const asking = !!code && !joined && !note;
  const server = useServerPlayer(asking ? code : null);
  // the stored id plays at once on a reload; the server's answer confirms or corrects it
  const stored = code && !guessed ? load<string>(playerKey(code)) : null;
  const id = joined ?? (asking ? (server.answered ? server.id : stored) : null);
  useEffect(() => {
    if (code && asking && server.answered) save(playerKey(code), server.id);
  }, [code, asking, server.answered, server.id]);

  if (code && id)
    return (
      <Pad
        key={`${code}:${id}`}
        code={code}
        playerId={id}
        onGone={() => {
          save(playerKey(code), null);
          if (joined || server.answered) {
            setJoined(null);
            setNote("You're not in this game any more. Join again?");
          } else setGuessed(true);
        }}
      />
    );
  // nothing stored: wait for the server, through its failures, rather than offer the form
  if (asking && !server.answered)
    return (
      <Screen>
        {server.error ? <Problem error={server.error} /> : <Loading />}
      </Screen>
    );
  const refused = server.refused ? joinError(server.refused, code!) : null;
  return (
    <JoinForm
      key={code ?? ""}
      code={code}
      note={note ?? refused ?? undefined}
      onJoined={(c, playerId) => {
        save(playerKey(c), playerId);
        if (c !== code) {
          handed.set(c, playerId);
          go(`/join/${c}`, { replace: true });
        } else {
          setNote(undefined);
          setJoined(playerId);
        }
      }}
    />
  );
}
