// Profiles and paired devices (plan D3): who's playing, profile & devices with pairing codes, claiming a
// code on a new device, and the first device's setup link.
import { useEffect, useEffectEvent, useState } from "react";
import { useLocation, useSearchParams } from "wouter";
import type { ApiError } from "../api/client";
import { toApiError, useApi, useAsync, useMe } from "../api/context";
import type { PairCode, PairKind } from "../api/types";
import { avatarInfo } from "../avatars";
import { Avatar, AvatarPicker } from "../components/Avatar";
import { Button, Card } from "../components/Controls";
import { Icon } from "../components/Icon";
import { WormLogo } from "../WormLogo";
import {
  AvatarStrip,
  CodeBoxes,
  Loading,
  Problem,
  QrCode,
  Screen,
  ScreenHead,
  secondsLeft,
  Unpaired,
  useNow,
} from "./common";

export function Who() {
  const api = useApi();
  const [, go] = useLocation();
  const { unpaired, reload } = useMe();
  const profiles = useAsync(unpaired ? null : "profiles", () => api.profiles());
  const [error, setError] = useState<ApiError | null>(null);
  const pick = (id: string) =>
    api.switchProfile(id).then(
      () => {
        reload();
        go("/");
      },
      (e: unknown) => setError(toApiError(e)),
    );
  return (
    <Screen>
      <div
        style={{
          maxWidth: 560,
          margin: "0 auto",
          textAlign: "center",
          paddingTop: 40,
        }}
      >
        <div
          className="seal"
          style={{ margin: "0 auto", width: 46, height: 46, borderRadius: 12 }}
        >
          <WormLogo />
        </div>
        <div className="h-title" style={{ marginTop: 18 }}>
          Who's playing?
        </div>
        <div className="sub">This device remembers the choice.</div>
        {unpaired && <Unpaired what="Picking a profile" />}
        {profiles.error && (
          <Problem error={profiles.error} retry={profiles.reload} />
        )}
        {error && <Problem error={error} />}
        {profiles.data && (
          <div className="who-grid">
            {profiles.data.map((p) => (
              <button key={p.id} type="button" onClick={() => void pick(p.id)}>
                <Avatar avatar={p.avatar} size={88} alt="" />
                <b style={{ fontSize: 15 }}>{p.name}</b>
              </button>
            ))}
            <button type="button" onClick={() => go("/me?invite=1")}>
              <span className="who-add">
                <Icon name="plus" size={30} />
              </span>
              <span className="sub" style={{ fontSize: 14 }}>
                Add someone
              </span>
            </button>
          </div>
        )}
        <div className="sub" style={{ marginTop: 26, fontSize: 12 }}>
          Only paired devices see this. New people join with an invite code.
        </div>
      </div>
    </Screen>
  );
}

const ago = (t: number, now: number) => {
  const s = Math.max(0, (now - t) / 1000);
  if (s < 90) return "now";
  if (s < 3600) return `${Math.round(s / 60)} min ago`;
  if (s < 86400 * 1.5) return `${Math.round(s / 3600)} h ago`;
  return `${Math.round(s / 86400)} days ago`;
};

const mmss = (s: number) =>
  `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;

function PairPanel({
  kind,
  code,
  onClose,
}: {
  kind: PairKind;
  code: PairCode;
  onClose: () => void;
}) {
  const now = useNow(1000);
  const left = secondsLeft(code.expiresAt, now);
  return (
    <Card
      className="pair-panel fade-in"
      role="dialog"
      aria-label={kind === "invite" ? "Invite someone" : "Add a device"}
    >
      <div style={{ fontWeight: 700 }}>
        {kind === "invite" ? "Invite someone" : "Add one of your devices"}
      </div>
      <div className="sub" style={{ fontSize: 12.5, margin: "4px 0 14px" }}>
        {kind === "invite"
          ? "Scan on their phone: they pick a name and avatar and get their own progress."
          : "Scan with the new device's camera, or open the site there and type the code. It logs in as you."}
      </div>
      <QrCode value={code.url} size={150} />
      <div
        className="num"
        style={{
          fontSize: 30,
          fontWeight: 800,
          letterSpacing: 5,
          marginTop: 12,
        }}
      >
        {code.code}
      </div>
      <div className="sub" style={{ fontSize: 12 }}>
        {left > 0
          ? `valid ${mmss(left)} · works once`
          : "expired — make a new one"}
      </div>
      <Button
        small
        variant="ghost"
        style={{ margin: "12px auto 0" }}
        onClick={onClose}
      >
        Done
      </Button>
    </Card>
  );
}

export function MePage() {
  const api = useApi();
  const [, go] = useLocation();
  const [params] = useSearchParams();
  const { me, unpaired, reload } = useMe();
  const devices = useAsync(me ? "devices" : null, () => api.devices());
  const profiles = useAsync(me ? "profiles" : null, () => api.profiles());
  const [pair, setPair] = useState<{ kind: PairKind; code: PairCode } | null>(
    null,
  );
  const [error, setError] = useState<ApiError | null>(null);
  const [avatar, setAvatar] = useState<string | null>(null);
  const now = useNow(30_000);

  const fetchCode = (kind: PairKind) =>
    api.pair(kind).then(
      (code) => setPair({ kind, code }),
      (e: unknown) => setError(toApiError(e)),
    );
  const makeCode = (kind: PairKind) => {
    setError(null);
    void fetchCode(kind);
  };
  // /me?invite=1 (Who's playing → Add someone) opens with an invite code
  const wantInvite = !!me && params.get("invite") === "1";
  const askInvite = useEffectEvent(() => void fetchCode("invite"));
  useEffect(() => {
    if (wantInvite) askInvite();
  }, [wantInvite]);

  if (unpaired)
    return (
      <Screen tab="settings">
        <ScreenHead back="/" title="Profile & devices" />
        <Unpaired what="A profile" />
      </Screen>
    );
  if (!me)
    return (
      <Screen tab="settings">
        <ScreenHead back="/" title="Profile & devices" />
        <Loading />
      </Screen>
    );

  const av = avatar ?? me.profile.avatar;
  const setAv = (k: string) => {
    setAvatar(k);
    api
      .updateMe({ avatar: k })
      .then(reload, (e: unknown) => setError(toApiError(e)));
  };
  const remove = (id: string) =>
    api
      .removeDevice(id)
      .then(devices.reload, (e: unknown) => setError(toApiError(e)));

  return (
    <Screen tab="settings">
      <ScreenHead back="/" title="Profile & devices" />
      <div className="cols">
        <div className="col">
          <div className="center" style={{ marginTop: 18 }}>
            <div
              key={av}
              style={{
                display: "inline-block",
                animation: "flipin .45s ease both",
              }}
            >
              <Avatar avatar={av} size={112} />
            </div>
            <div style={{ fontSize: 24, fontWeight: 750, marginTop: 10 }}>
              {me.profile.name}
            </div>
            <div className="sub" style={{ fontSize: 13 }}>
              {avatarInfo(av).name}
              {me.profile.admin ? " · admin" : ""}
            </div>
          </div>
          <div className="label">Pick your avatar</div>
          <AvatarPicker value={av} onChange={setAv} />
        </div>
        <div className="col">
          <div className="label">Your devices</div>
          <Card style={{ padding: "4px 16px" }}>
            {[...(devices.data ?? [])]
              .sort((a, b) => Number(b.current) - Number(a.current))
              .map((d) => (
                <div className="set-row" key={d.id}>
                  <div>
                    <div>{d.label}</div>
                    <div className="m">
                      {d.current ? "now" : ago(d.lastSeen, now)}
                    </div>
                  </div>
                  {d.current ? (
                    <span className="sub" style={{ fontSize: 12 }}>
                      this one
                    </span>
                  ) : (
                    <button
                      type="button"
                      className="remove"
                      aria-label={`Remove ${d.label}`}
                      onClick={() => void remove(d.id)}
                    >
                      Remove
                    </button>
                  )}
                </div>
              ))}
            {!devices.data && <div className="set-row sub">Loading…</div>}
          </Card>
          {pair ? (
            <PairPanel
              kind={pair.kind}
              code={pair.code}
              onClose={() => setPair(null)}
            />
          ) : (
            <div className="row" style={{ gap: 8, marginTop: 12 }}>
              <Button
                small
                variant="sky"
                icon="qr"
                style={{ flex: 1 }}
                onClick={() => makeCode("device")}
              >
                Add a device
              </Button>
              <Button
                small
                variant="ghost"
                icon="plus"
                style={{ flex: 1 }}
                onClick={() => makeCode("invite")}
              >
                Invite someone
              </Button>
            </div>
          )}
          {error && <div className="err">{error.message}</div>}
          <div className="sub" style={{ fontSize: 12, marginTop: 10 }}>
            No passwords: each device keeps a login cookie for a year. Remove a
            lost phone here.
          </div>
          <div className="label">Profiles on this server</div>
          <Card style={{ padding: "6px 14px" }}>
            {profiles.data?.map((p) => (
              <div className="set-row" key={p.id}>
                <div className="row">
                  <Avatar
                    avatar={p.id === me.profile.id ? av : p.avatar}
                    size={34}
                    alt=""
                  />
                  <b>{p.name}</b>
                </div>
                {p.id === me.profile.id && (
                  <span className="sub" style={{ fontSize: 12 }}>
                    you
                  </span>
                )}
              </div>
            ))}
          </Card>
          <Button
            small
            variant="ghost"
            style={{ marginTop: 12, width: "100%" }}
            onClick={() => go("/who")}
          >
            Switch profile on this device
          </Button>
        </div>
      </div>
      <div style={{ height: 10 }} />
    </Screen>
  );
}

// NAME_REQUIRED: a claim of a code alone refused with these is an invite: it needs a name and an
// avatar (the server answers 400 invalid_name and leaves the code usable).
const NAME_REQUIRED = [
  "invalid_name",
  "invalid_avatar",
  "name_required",
  "profile_required",
];

export function Pair({ code: given }: { code?: string }) {
  const api = useApi();
  const [, go] = useLocation();
  const { reload } = useMe();
  const [code, setCode] = useState(
    (given ?? "").toUpperCase().replace(/[^A-Z0-9]/g, ""),
  );
  const [invite, setInvite] = useState(false);
  const [name, setName] = useState("");
  const [avatar, setAvatar] = useState("vinyl");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const ready =
    code.length === 6 && (!invite || name.trim().length > 0) && !busy;

  const claim = () => {
    if (!ready) return;
    setBusy(true);
    setError(null);
    api.claimPair(invite ? { code, name: name.trim(), avatar } : { code }).then(
      () => {
        reload();
        go("/", { replace: true });
      },
      (e: unknown) => {
        const err = toApiError(e);
        setBusy(false);
        if (!invite && (NAME_REQUIRED.includes(err.code) || err.status === 422))
          setInvite(true);
        else if (err.status === 404 || err.status === 410)
          setError("That code is unknown or expired. Make a new one.");
        else setError(err.message);
      },
    );
  };

  return (
    <Screen
      footer={
        <Button disabled={!ready} onClick={claim}>
          {invite ? "Create my profile" : "Pair this device"}
        </Button>
      }
    >
      <ScreenHead back="/" title="Pair this device" />
      <div style={{ maxWidth: 420, margin: "0 auto", textAlign: "center" }}>
        <div className="sub" style={{ margin: "34px 0 8px" }}>
          The code on your other device
        </div>
        <CodeBoxes
          value={code}
          onChange={setCode}
          length={6}
          label="Pairing code"
          onDone={claim}
        />
        {invite && (
          <>
            <div style={{ fontWeight: 700, marginTop: 26 }}>
              You're invited!
            </div>
            <div className="sub" style={{ margin: "4px 0 10px" }}>
              Pick a name and an avatar for your profile.
            </div>
            <input
              className="input"
              aria-label="Your name"
              value={name}
              maxLength={20}
              onChange={(e) => setName(e.target.value)}
              style={{ textAlign: "center", maxWidth: 260 }}
            />
            <div style={{ marginTop: 14 }}>
              <AvatarStrip value={avatar} onChange={setAvatar} />
            </div>
          </>
        )}
        {error && (
          <div className="err" role="alert">
            {error}
          </div>
        )}
        <div className="sub" style={{ marginTop: 22, fontSize: 12.5 }}>
          Codes work once and for 5 minutes. On a paired device: Profile &amp;
          devices → Add a device.
        </div>
      </div>
    </Screen>
  );
}

export function Setup() {
  const api = useApi();
  const [, go] = useLocation();
  const [params] = useSearchParams();
  const { reload } = useMe();
  const token = params.get("token") ?? "";
  const [name, setName] = useState("");
  const [avatar, setAvatar] = useState("vinyl");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const ready = !!token && name.trim().length > 0 && !busy;
  const create = () => {
    if (!ready) return;
    setBusy(true);
    api.setup({ token, name: name.trim(), avatar }).then(
      () => {
        reload();
        go("/", { replace: true });
      },
      (e: unknown) => {
        const err = toApiError(e);
        setBusy(false);
        setError(
          err.status === 410
            ? "This setup link was already used. Pair this device from one that is set up."
            : err.status === 403 || err.status === 401
              ? "This setup link isn't valid. Copy it again from the server log."
              : err.message,
        );
      },
    );
  };
  return (
    <Screen
      footer={
        <Button disabled={!ready} onClick={create}>
          Create my profile
        </Button>
      }
    >
      <div
        style={{
          maxWidth: 560,
          margin: "0 auto",
          textAlign: "center",
          paddingTop: 30,
        }}
      >
        <div
          className="seal"
          style={{ margin: "0 auto", width: 46, height: 46, borderRadius: 12 }}
        >
          <WormLogo />
        </div>
        <div className="h-title" style={{ marginTop: 18 }}>
          Welcome to Reworm
        </div>
        <div className="sub">
          The first device of this server. You'll be its admin.
        </div>
        {!token && (
          <div className="err" role="alert">
            The setup link has no token. Open the link from the server log.
          </div>
        )}
        <div className="sub" style={{ margin: "28px 0 8px" }}>
          Your name
        </div>
        <input
          className="input"
          aria-label="Your name"
          value={name}
          maxLength={20}
          onChange={(e) => setName(e.target.value)}
          style={{ textAlign: "center", maxWidth: 260 }}
        />
        <div className="label" style={{ textAlign: "left" }}>
          Pick your avatar
        </div>
        <AvatarPicker value={avatar} onChange={setAvatar} />
        {error && (
          <div className="err" role="alert">
            {error}
          </div>
        )}
      </div>
    </Screen>
  );
}
