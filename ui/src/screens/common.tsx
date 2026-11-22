// Pieces shared by the screens: header, brand, loading and error states, the way to a profile (code claim, setup
// hint), toggle, QR, rings, the year range, the avatar strip and the code boxes.
import { QRCodeSVG } from "qrcode.react";
import { useEffect, useState, type ReactNode } from "react";
import { useLocation } from "wouter";
import type { ApiError } from "../api/client";
import { toApiError, useApi, useMe } from "../api/context";
import type { Deck, Decks, Difficulty } from "../api/types";
import { AVATARS } from "../avatars";
import { AppShell, type Tab } from "../components/AppShell";
import { Avatar } from "../components/Avatar";
import { Button, Card, IconButton } from "../components/Controls";
import { cx } from "../lib/cx";
import { YEAR_MAX, YEAR_MIN } from "../lib/prefs";
import { WormLogo } from "../WormLogo";

const TAB_PATH: Record<Tab, string> = {
  home: "/",
  train: "/train",
  settings: "/settings",
};

// Screen is the app shell with the bottom tabs wired to the routes.
export function Screen({
  tab,
  footer,
  className,
  children,
}: {
  tab?: Tab;
  footer?: ReactNode;
  className?: string;
  children: ReactNode;
}) {
  const [, go] = useLocation();
  return (
    <AppShell
      tab={tab}
      onTab={(t) => go(TAB_PATH[t])}
      footer={footer}
      scrollClassName={cx("fade-in", className)}
    >
      {children}
    </AppShell>
  );
}

export function Brand({ small }: { small?: boolean }) {
  return (
    <div className="row">
      <div
        className="seal"
        style={small ? { width: 30, height: 30 } : undefined}
      >
        <WormLogo />
      </div>
      <div className="brand">
        Re<span>worm</span>
      </div>
    </div>
  );
}

// ScreenHead: back button, centered title, optional right-hand control.
export function ScreenHead({
  back,
  title,
  right,
}: {
  back?: string;
  title: ReactNode;
  right?: ReactNode;
}) {
  const [, go] = useLocation();
  return (
    <div className="screen-head">
      {back ? (
        <IconButton icon="back" label="Back" onClick={() => go(back)} />
      ) : (
        <span />
      )}
      <div className="t">{title}</div>
      {right ?? <span />}
    </div>
  );
}

export function Loading() {
  return (
    <div className="sub empty" role="status">
      Loading…
    </div>
  );
}

export function Problem({
  error,
  retry,
}: {
  error: ApiError;
  retry?: () => void;
}) {
  return (
    <Card style={{ marginTop: 18 }} role="alert">
      <div style={{ fontWeight: 650 }}>
        {error.status === 0 ? "Can't reach the server" : "Something went wrong"}
      </div>
      <div className="sub" style={{ fontSize: 13, marginTop: 4 }}>
        {error.message}
      </div>
      {retry && (
        <Button small variant="ghost" style={{ marginTop: 12 }} onClick={retry}>
          Try again
        </Button>
      )}
    </Card>
  );
}

// NARROW: a card alone on a wide screen keeps a readable width, centred.
const NARROW = { marginTop: 18, maxWidth: 460, marginInline: "auto" };

// SetupPending: no profile exists yet (401 setup_pending): only the setup link from the log makes the first.
export function SetupPending({ narrow = false }: { narrow?: boolean }) {
  const { reload } = useMe();
  return (
    <Card style={narrow ? NARROW : { marginTop: 18 }}>
      <div style={{ fontWeight: 700 }}>Reworm isn't set up yet</div>
      <div className="sub" style={{ fontSize: 13, margin: "4px 0 12px" }}>
        Reworm printed a one-time setup link in its container log (docker logs)
        when it started. Open it on this device to create the first profile.
      </div>
      <Button small variant="ghost" onClick={reload}>
        Check again
      </Button>
    </Card>
  );
}

// GetProfile: a profile for this device: make one right here, or bring yours over from another device with its
// code (given: from a code's link, opens on the code). Shown where a device without a profile needs one (401),
// and on /pair to move a device to another profile. onDone runs once the profile is reloaded.
export function GetProfile({
  title,
  given,
  codeFirst = false,
  onDone,
}: {
  title: string;
  given?: string;
  codeFirst?: boolean;
  onDone?: () => void;
}) {
  const { fresh, reload } = useMe();
  const [withCode, setWithCode] = useState(codeFirst || !!given);
  if (fresh) return <SetupPending narrow />;
  const done = () => {
    reload();
    onDone?.();
  };
  return (
    <Card style={NARROW}>
      <div style={{ fontWeight: 700 }}>{title}</div>
      <div className="sub" style={{ fontSize: 13, margin: "4px 0 16px" }}>
        {withCode
          ? "On your other device, open Profile & devices → Add a device and enter its code here. Codes work once and for 5 minutes."
          : "Pick a name and an avatar. Your profile keeps your training progress and is yours alone."}
      </div>
      {withCode ? (
        <ClaimCode given={given} onDone={done} />
      ) : (
        <CreateProfile onDone={done} />
      )}
      <button
        type="button"
        className="link"
        style={{ marginTop: 14, padding: "12px 4px" }}
        onClick={() => setWithCode((v) => !v)}
      >
        {withCode
          ? "New here? Create a profile"
          : "Have a profile on another device? Use its code"}
      </button>
    </Card>
  );
}

// useMove runs a call that puts this device on a profile. 409 last_device asks first (ask): this device is the only
// one of its profile, so going on (run(true)) deletes that profile and its progress.
function useMove(
  call: (leave: boolean) => Promise<void>,
  onDone: () => void,
  wording: (e: ApiError) => string = (e) => e.message,
) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ask, setAsk] = useState(false);
  const run = (leave = false) => {
    setBusy(true);
    setError(null);
    setAsk(false);
    call(leave).then(onDone, (e: unknown) => {
      const err = toApiError(e);
      setBusy(false);
      if (err.code === "last_device") setAsk(true);
      else setError(wording(err));
    });
  };
  return { busy, error, ask, run, cancel: () => setAsk(false) };
}

// LeaveConfirm: the question useMove asks before this device leaves its profile for good.
function LeaveConfirm({
  onLeave,
  onCancel,
}: {
  onLeave: () => void;
  onCancel: () => void;
}) {
  const { me } = useMe();
  const name = me?.profile.name ?? "This profile";
  return (
    <div className="leave-confirm" role="alert">
      <b>This is {name}'s only device.</b> Going on deletes {name}'s profile and
      training progress for good.
      <div className="row" style={{ gap: 8, marginTop: 12 }}>
        <Button small variant="ghost" style={{ flex: 1 }} onClick={onCancel}>
          Keep {name}
        </Button>
        <Button small style={{ flex: 1 }} onClick={onLeave}>
          Delete it
        </Button>
      </div>
    </div>
  );
}

// CreateProfile: a new profile for this device, name and avatar. onDone runs once the device has it. No maxLength:
// the server counts characters, the browser UTF-16 units; a name too long is its invalid_name.
export function CreateProfile({ onDone }: { onDone: () => void }) {
  const api = useApi();
  const [name, setName] = useState("");
  const [avatar, setAvatar] = useState("vinyl");
  const m = useMove(
    (leave) => api.createProfile({ name: name.trim(), avatar, leave }),
    onDone,
  );
  const ready = name.trim().length > 0 && !m.busy;
  const create = () => {
    if (ready) m.run();
  };
  return (
    <div style={{ textAlign: "center" }}>
      <input
        className="input"
        aria-label="Your name"
        value={name}
        onChange={(e) => {
          m.cancel(); // the question was about the input it was asked for
          setName(e.target.value);
        }}
        onKeyDown={(e) => e.key === "Enter" && create()}
        style={{ textAlign: "center", maxWidth: 260 }}
      />
      <div style={{ marginTop: 14 }}>
        <AvatarStrip value={avatar} onChange={setAvatar} />
      </div>
      {m.error && (
        <div className="err" role="alert">
          {m.error}
        </div>
      )}
      {m.ask ? (
        <LeaveConfirm onLeave={() => m.run(true)} onCancel={m.cancel} />
      ) : (
        <Button disabled={!ready} onClick={create} style={{ marginTop: 18 }}>
          Create my profile
        </Button>
      )}
    </div>
  );
}

// ClaimCode: the code of one of your own devices (Add a device), claimed for this device. onDone runs once the
// device has the profile.
export function ClaimCode({
  given,
  onDone,
}: {
  given?: string;
  onDone: () => void;
}) {
  const api = useApi();
  const [code, setCode] = useState(
    (given ?? "").toUpperCase().replace(/[^A-Z0-9]/g, ""),
  );
  const m = useMove(
    (leave) => api.claimPair({ code, leave }),
    onDone,
    (err) =>
      err.status === 404 || err.status === 410
        ? "That code is unknown or expired. Make a new one."
        : err.message,
  );
  const ready = code.length === 6 && !m.busy;
  const claim = () => {
    if (ready) m.run();
  };

  return (
    <div style={{ textAlign: "center" }}>
      <CodeBoxes
        value={code}
        onChange={(c) => {
          m.cancel(); // a new code is another profile: ask again
          setCode(c);
        }}
        length={6}
        label="Your code"
        onDone={claim}
      />
      {m.error && (
        <div className="err" role="alert">
          {m.error}
        </div>
      )}
      {m.ask ? (
        <LeaveConfirm onLeave={() => m.run(true)} onCancel={m.cancel} />
      ) : (
        <Button disabled={!ready} onClick={claim} style={{ marginTop: 18 }}>
          Use this code
        </Button>
      )}
    </div>
  );
}

export function Toggle({
  on,
  onChange,
  label,
}: {
  on: boolean;
  onChange: (v: boolean) => void;
  label: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      aria-label={label}
      className={cx("toggle", on && "on")}
      onClick={() => onChange(!on)}
    />
  );
}

export function QrCode({
  value,
  size = 168,
}: {
  value: string;
  size?: number;
}) {
  return (
    <div className="qr" data-qr={value}>
      <QRCodeSVG
        value={value}
        size={size}
        bgColor="#f5eee4"
        fgColor="#13110f"
        level="M"
        title={value}
      />
    </div>
  );
}

// Ring is the summary's score ring and the decks' mini ring: p (0–1) of the circle in col.
export function Ring({
  p,
  size = 150,
  stroke = 12,
  col = "var(--jade)",
}: {
  p: number;
  size?: number;
  stroke?: number;
  col?: string;
}) {
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  return (
    <svg width={size} height={size} aria-hidden="true">
      <circle
        cx={size / 2}
        cy={size / 2}
        r={r}
        stroke="var(--panel2)"
        strokeWidth={stroke}
        fill="none"
      />
      {p > 0 && (
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          stroke={col}
          strokeWidth={stroke}
          fill="none"
          strokeLinecap="round"
          strokeDasharray={`${c * Math.min(1, p)} ${c}`}
        />
      )}
    </svg>
  );
}

// accColor: an accuracy (0–1) as the mock's heat colour, green to red (decade tiles, by-decade bars).
export const accColor = (p: number) =>
  p > 0.78
    ? "#3dbb8c"
    : p > 0.65
      ? "#9bc36b"
      : p > 0.55
        ? "#e9b44c"
        : "#e0675d";

// deckLabel names a deck selector: "Essentials + Smash Hits", "All songs · 1960–1989".
export function deckLabel(deck: Deck, decks?: Decks): string {
  const names = [
    ...deck.sets.map((id) => decks?.sets.find((s) => s.id === id)?.name ?? id),
    ...deck.categories.map(
      (id) => decks?.categories.find((s) => s.id === id)?.name ?? id,
    ),
    ...deck.genres.map(
      (id) => decks?.genres.find((s) => s.id === id)?.name ?? id,
    ),
  ];
  const all = deck.from <= YEAR_MIN && deck.to >= YEAR_MAX;
  const years = all ? "" : ` · ${deck.from}–${deck.to}`;
  return (names.length ? names.join(" + ") : "All songs") + years;
}

// The proposed-year tiers and each difficulty's base mix, easiest first.
export const TIERS = [
  { k: "easy", label: "Easy one", r: "±10–25", col: "var(--sky)" },
  { k: "clear", label: "Clear one", r: "±4–9", col: "var(--jade)" },
  { k: "tight", label: "Tight one", r: "±1–3", col: "var(--gold)" },
  { k: "exact", label: "Spot-on card", r: "exact", col: "var(--hanko)" },
] as const;

export const DIFF: Record<
  Difficulty,
  { label: string; mix: number[]; desc: string }
> = {
  easy: {
    label: "Easy",
    mix: [0.55, 0.25, 0.08, 0.12],
    desc: "Mostly other eras. Learn the decades first.",
  },
  medium: {
    label: "Medium",
    mix: [0.35, 0.3, 0.2, 0.15],
    desc: "Mostly easy and clear cards, a tight one now and then.",
  },
  hard: {
    label: "Hard",
    mix: [0.12, 0.28, 0.38, 0.22],
    desc: "Lots of ±1–3. Game-night pressure.",
  },
};

export function Distrib({
  difficulty,
  adapts = true,
}: {
  difficulty: Difficulty;
  adapts?: boolean;
}) {
  const d = DIFF[difficulty];
  return (
    <>
      <div className="distrib" aria-label="Year mix">
        {TIERS.map((t, i) => (
          <i
            key={t.k}
            style={{
              width: `${d.mix[i] * 100}%`,
              background: t.col,
              color: i === 3 ? "#fff" : undefined,
            }}
          >
            {Math.round(d.mix[i] * 100)}%
          </i>
        ))}
      </div>
      <div className="legend" style={{ flexWrap: "wrap", gap: "4px 12px" }}>
        {TIERS.map((t) => (
          <span key={t.k}>
            <i className="dot" style={{ background: t.col }} />
            {t.r}
          </span>
        ))}
        {adapts && (
          <span style={{ color: "var(--faint)" }}>· adapts per card</span>
        )}
      </div>
      <div className="sub" style={{ fontSize: 12.5, marginTop: 8 }}>
        {d.desc}
      </div>
    </>
  );
}

const PRESETS: [string, number, number][] = [
  ["All", YEAR_MIN, YEAR_MAX],
  ["Evergreens", YEAR_MIN, 1949],
  ["50s", 1950, 1959],
  ["60s–80s", 1960, 1989],
  ["90s–00s", 1990, 2009],
  ["Recent", 2010, YEAR_MAX],
];

// YearRange: two knobs on one track plus the preset chips.
export function YearRange({
  from,
  to,
  onChange,
}: {
  from: number;
  to: number;
  onChange: (from: number, to: number) => void;
}) {
  const pct = (v: number) => ((v - YEAR_MIN) / (YEAR_MAX - YEAR_MIN)) * 100;
  return (
    <Card style={{ padding: "10px 16px 14px" }}>
      <div className="years">
        <div className="track" />
        <div
          className="fill"
          style={{ left: `${pct(from)}%`, right: `${100 - pct(to)}%` }}
        />
        <input
          type="range"
          aria-label="From year"
          min={YEAR_MIN}
          max={YEAR_MAX}
          value={from}
          onChange={(e) => onChange(Math.min(+e.target.value, to), to)}
        />
        <input
          type="range"
          aria-label="To year"
          min={YEAR_MIN}
          max={YEAR_MAX}
          value={to}
          onChange={(e) => onChange(from, Math.max(+e.target.value, from))}
        />
        <div className="ticks" aria-hidden="true">
          {[1900, 1925, 1950, 1975, 2000, 2025]
            .filter((y) => y <= YEAR_MAX)
            .map((y) => (
              <span key={y} style={{ left: `${pct(y)}%` }}>
                {y % 100 ? `’${String(y).slice(2)}` : y}
              </span>
            ))}
        </div>
      </div>
      <div className="chips" style={{ marginTop: 10 }}>
        {PRESETS.map(([l, a, b]) => (
          <button
            key={l}
            type="button"
            className={cx("chip", from === a && to === b && "on")}
            aria-pressed={from === a && to === b}
            style={{ padding: "6px 11px", fontSize: 13 }}
            onClick={() => onChange(a, b)}
          >
            {l}
          </button>
        ))}
      </div>
    </Card>
  );
}

// AvatarStrip: the join screen's horizontal picker.
export function AvatarStrip({
  value,
  onChange,
}: {
  value: string;
  onChange: (k: string) => void;
}) {
  return (
    <div className="av-strip" role="radiogroup" aria-label="Avatar">
      {AVATARS.map((a) => (
        <button
          key={a.key}
          type="button"
          role="radio"
          aria-checked={a.key === value}
          aria-label={a.name}
          onClick={() => onChange(a.key)}
        >
          <Avatar avatar={a.key} size={52} selected={a.key === value} alt="" />
        </button>
      ))}
    </div>
  );
}

// CodeBoxes: one hidden input under a row of letter boxes (4 for a room, 6 for pairing).
export function CodeBoxes({
  value,
  onChange,
  length,
  label,
  onDone,
}: {
  value: string;
  onChange: (v: string) => void;
  length: number;
  label: string;
  onDone?: () => void;
}) {
  return (
    <div className={cx("codebox", length > 4 && "six")}>
      {Array.from({ length }, (_, i) => (
        <div
          key={i}
          className={cx(
            "b num",
            i === Math.min(value.length, length - 1) && "cur",
          )}
          aria-hidden="true"
        >
          {value[i] ?? ""}
        </div>
      ))}
      <input
        aria-label={label}
        value={value}
        autoCapitalize="characters"
        autoComplete="off"
        spellCheck={false}
        maxLength={length}
        onChange={(e) =>
          onChange(
            e.target.value
              .toUpperCase()
              .replace(/[^A-Z0-9]/g, "")
              .slice(0, length),
          )
        }
        onKeyDown={(e) => {
          if (e.key === "Enter") onDone?.();
        }}
      />
    </div>
  );
}

// useNow ticks every `ms` (a countdown text, an expiry).
export function useNow(ms = 1000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), ms);
    return () => clearInterval(id);
  }, [ms]);
  return now;
}

export { secondsLeft } from "../api/clock";
