// Pieces shared by the screens: header, brand, loading and error states, toggle, QR, rings, the year range,
// the avatar strip and the code boxes.
import { QRCodeSVG } from "qrcode.react";
import { useEffect, useState, type ReactNode } from "react";
import { useLocation } from "wouter";
import type { ApiError } from "../api/client";
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
  party: "/party",
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

// Unpaired: this device has no profile yet (401), with the way to pair it.
export function Unpaired({ what }: { what: string }) {
  const [, go] = useLocation();
  return (
    <Card style={{ marginTop: 18 }}>
      <div style={{ fontWeight: 700 }}>This device isn't paired</div>
      <div className="sub" style={{ fontSize: 13, margin: "4px 0 12px" }}>
        {what} needs a profile. On a paired device open Profile &amp; devices →
        Add a device, then enter the code here.
      </div>
      <Button small variant="sky" icon="qr" onClick={() => go("/pair")}>
        Pair this device
      </Button>
    </Card>
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

export { DeckArt } from "../components/DeckArt";

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

// The proposed-year tiers (plan §2) and each difficulty's base mix, easiest first.
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
