// Profile and paired devices: profile & devices with device codes, claiming a code on a new device, and the
// first device's setup link. Profiles are private: no list of them, no switching between them.
import { useState } from "react";
import { useLocation, useSearchParams } from "wouter";
import type { ApiError } from "../api/client";
import { toApiError, useApi, useAsync, useMe } from "../api/context";
import type { PairCode } from "../api/types";
import { avatarInfo } from "../avatars";
import { appUrl } from "../base";
import { Avatar, AvatarPicker } from "../components/Avatar";
import { Button, Card } from "../components/Controls";
import { WormLogo } from "../WormLogo";
import {
  GetProfile,
  Loading,
  Problem,
  QrCode,
  Screen,
  ScreenHead,
  secondsLeft,
  useNow,
} from "./common";

const ago = (t: number, now: number) => {
  const s = Math.max(0, (now - t) / 1000);
  if (s < 90) return "now";
  if (s < 3600) return `${Math.round(s / 60)} min ago`;
  if (s < 86400 * 1.5) return `${Math.round(s / 3600)} h ago`;
  return `${Math.round(s / 86400)} days ago`;
};

const mmss = (s: number) =>
  `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;

function PairPanel({ code, onClose }: { code: PairCode; onClose: () => void }) {
  useNow(1000); // ticks the expiry; the time itself is the server's (expiresAt)
  const left = secondsLeft(code.expiresAt);
  // the server may hand an app path (/pair/CODE): the QR needs the full address
  const url = appUrl(code.url);
  return (
    <Card
      className="pair-panel fade-in"
      role="dialog"
      aria-label="Add a device"
    >
      <div style={{ fontWeight: 700 }}>Add one of your devices</div>
      <div className="sub" style={{ fontSize: 12.5, margin: "4px 0 14px" }}>
        Scan with the new device's camera, or open the site there and type the
        code. It logs in as you.
      </div>
      <QrCode value={url} size={150} />
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
  const { me, unpaired, error: meError, reload } = useMe();
  const devices = useAsync(me ? "devices" : null, () => api.devices());
  const [pair, setPair] = useState<PairCode | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [avatar, setAvatar] = useState<string | null>(null);
  const now = useNow(30_000);

  const makeCode = () => {
    setError(null);
    api.pair().then(setPair, (e: unknown) => setError(toApiError(e)));
  };

  if (unpaired)
    return (
      <Screen tab="settings">
        <ScreenHead back="/" title="Profile & devices" />
        <GetProfile title="No profile on this device yet" />
      </Screen>
    );
  if (!me)
    return (
      <Screen tab="settings">
        <ScreenHead back="/" title="Profile & devices" />
        {meError ? <Problem error={meError} retry={reload} /> : <Loading />}
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
            <PairPanel code={pair} onClose={() => setPair(null)} />
          ) : (
            <Button
              small
              variant="sky"
              icon="qr"
              style={{ marginTop: 12, width: "100%" }}
              onClick={makeCode}
            >
              Add a device
            </Button>
          )}
          {error && <div className="err">{error.message}</div>}
          <div className="sub" style={{ fontSize: 12, marginTop: 10 }}>
            No passwords: each device keeps a login cookie for a year. Remove a
            lost phone here.
          </div>
          <Button
            small
            variant="ghost"
            style={{ marginTop: 18, width: "100%" }}
            onClick={() => go("/pair")}
          >
            Use another profile on this device
          </Button>
        </div>
      </div>
      <div style={{ height: 10 }} />
    </Screen>
  );
}

// Pair is /pair and a code's link /pair/<code>: the code of one of your own devices, or a new profile. A device
// with a profile gets here to use another one (Profile & devices); a fresh install has nobody to get a code
// from: the setup hint instead (GetProfile).
export function Pair({ code: given }: { code?: string }) {
  const [, go] = useLocation();
  const { me } = useMe();
  return (
    <Screen className="form">
      <ScreenHead
        back={me ? "/me" : "/"}
        title={me ? "Another profile" : "Enter your code"}
      />
      <GetProfile
        key={given ?? ""} // a code's link opened in this tab starts over with that code
        title={
          me
            ? `Use another profile instead of ${me.profile.name}`
            : "The code from your other device"
        }
        given={given}
        codeFirst
        onDone={() => go("/", { replace: true })}
      />
    </Screen>
  );
}

// Setup is the server log's one-time link, /setup/<token> (?token= works too).
export function Setup({ token: given }: { token?: string }) {
  const api = useApi();
  const [, go] = useLocation();
  const [params] = useSearchParams();
  const { reload } = useMe();
  const token = given ?? params.get("token") ?? "";
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
            ? "This setup link was already used. Get a code from a device that has a profile."
            : err.status === 403 || err.status === 401
              ? "This setup link isn't valid. Copy it again from the container log."
              : err.message,
        );
      },
    );
  };
  return (
    <Screen
      className="form"
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
          The first profile on this Reworm. You'll be its admin.
        </div>
        {!token && (
          <div className="err" role="alert">
            The setup link has no token. Open the link from the container log.
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
