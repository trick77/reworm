// /join/:code? — the phone's way in: the room code (from the QR or typed), a name and an avatar. Once
// joined, the same route is the pad.
import { useState } from "react";
import { useLocation } from "wouter";
import type { ApiError } from "../../api/client";
import { toApiError, useApi, useAsync, useMe } from "../../api/context";
import { avatarInfo } from "../../avatars";
import { Button } from "../../components/Controls";
import { load, save } from "../../lib/prefs";
import { AvatarStrip, CodeBoxes, Loading, Screen, ScreenHead } from "../common";
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

export function Join({ code: raw }: { code?: string }) {
  const [, go] = useLocation();
  const code = raw?.toUpperCase();
  const api = useApi();
  // joined: the player this page just joined as; note: why the form is back (the pad lost its player)
  const [joined, setJoined] = useState<string | null>(null);
  const [note, setNote] = useState<string | undefined>();
  // the server knows this browser's player (guest cookie or profile): a reload, the QR again, another tab
  const asking = !!code && !joined && !note;
  const asked = useAsync(asking ? `me:${code}` : null, () => api.roomMe(code!));
  const id = joined ?? (asking ? (asked.data ?? null) : null);
  if (code && id)
    return (
      <Pad
        key={`${code}:${id}`}
        code={code}
        playerId={id}
        onGone={() => {
          setJoined(null);
          setNote("You're not in this game any more. Join again?");
        }}
      />
    );
  if (asking && asked.loading)
    return (
      <Screen>
        <Loading />
      </Screen>
    );
  const refused = asking && asked.error ? joinError(asked.error, code!) : null;
  return (
    <JoinForm
      key={code ?? ""}
      code={code}
      note={note ?? refused ?? undefined}
      onJoined={(c, playerId) => {
        if (c !== code) go(`/join/${c}`, { replace: true });
        else setJoined(playerId);
      }}
    />
  );
}
