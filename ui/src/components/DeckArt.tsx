// Deck tiles: a picture per theme (record, cassette, bolt, sun, …) on its own colours, picked from the deck's
// name. Letters were dropped: "RT" told nobody it was Rock Anthems.
import type { ReactNode } from "react";

type Art = { glyph: ReactNode; from: string; to: string };

const G = {
  vinyl: (
    <>
      <circle cx="12" cy="12" r="9" />
      <circle cx="12" cy="12" r="5.5" opacity=".45" />
      <circle cx="12" cy="12" r="2.2" fill="currentColor" />
    </>
  ),
  cassette: (
    <>
      <rect x="2.5" y="5.5" width="19" height="13" rx="2" />
      <circle cx="8.5" cy="11" r="1.9" />
      <circle cx="15.5" cy="11" r="1.9" />
      <path d="M10.4 11h3.2M6.5 18.5l1.6-3h7.8l1.6 3" />
    </>
  ),
  bolt: (
    <path d="M13.5 2 4.5 13.5h6.5L9.5 22l10-12.5h-6.8z" fill="currentColor" />
  ),
  heart: (
    <path
      d="M12 20.5S3.5 15.2 3.5 9.2A4.3 4.3 0 0 1 12 7a4.3 4.3 0 0 1 8.5 2.2c0 6-8.5 11.3-8.5 11.3z"
      fill="currentColor"
    />
  ),
  sun: (
    <>
      <circle cx="12" cy="12" r="4.2" fill="currentColor" />
      <path d="M12 2v2.5M12 19.5V22M2 12h2.5M19.5 12H22M4.9 4.9l1.8 1.8M17.3 17.3l1.8 1.8M4.9 19.1l1.8-1.8M17.3 6.7l1.8-1.8" />
    </>
  ),
  disco: (
    <>
      <path d="M12 1.5V4" />
      <circle cx="12" cy="13" r="8" />
      <ellipse cx="12" cy="13" rx="8" ry="3.2" />
      <ellipse cx="12" cy="13" rx="3.2" ry="8" />
    </>
  ),
  popper: (
    <>
      <path d="M3.5 20.5 8.5 8l7.5 7.5z" fill="currentColor" />
      <path d="M14 3.5v2.5M19 8.5h2.5M17 5.5l1.8-1.8M12 6.5l.8-1M18 13l1 .6" />
    </>
  ),
  mic: (
    <>
      <rect x="9" y="2.5" width="6" height="11" rx="3" fill="currentColor" />
      <path d="M5.5 10.5a6.5 6.5 0 0 0 13 0M12 17v4M8.5 21h7" />
    </>
  ),
  clapper: (
    <>
      <rect x="3" y="10" width="18" height="10.5" rx="1.5" />
      <path d="m3 10-.6-3.2L19.5 3.5l.6 3.2zM7.2 9.3 6 5.8M12 8.4l-1.2-3.5M16.8 7.5l-1.2-3.5" />
    </>
  ),
  tree: (
    <>
      <path d="M12 2.5 17 9h-2.8l4.3 6.5h-13L9.8 9H7z" fill="currentColor" />
      <path d="M12 15.5v5M9 20.5h6" />
    </>
  ),
  radio: (
    <>
      <rect x="2.5" y="8" width="19" height="12.5" rx="2" />
      <circle cx="15.5" cy="14.2" r="3" />
      <path d="M6 12h4.5M6 15h4.5M6 18h4.5M6.5 8l10-5" />
    </>
  ),
  star: (
    <path
      d="m12 2.5 2.9 6 6.6.9-4.8 4.6 1.2 6.5L12 17.4l-5.9 3.1 1.2-6.5-4.8-4.6 6.6-.9z"
      fill="currentColor"
    />
  ),
  crown: (
    <path
      d="M3 18.5h18L19.5 7l-4.5 4.5L12 4l-3 7.5L4.5 7zM3 21h18"
      fill="currentColor"
    />
  ),
  gramophone: (
    <>
      <path d="M13 9.5 21 3v13z" fill="currentColor" />
      <path d="M13 12.5h-2.5v5M5 21h11M10.5 17.5 7 21M10.5 17.5l3.5 3.5" />
    </>
  ),
  note: (
    <>
      <path d="M9 18V5.5L20 3.5v12" />
      <circle cx="6.5" cy="18" r="2.5" fill="currentColor" />
      <circle cx="17.5" cy="15.5" r="2.5" fill="currentColor" />
    </>
  ),
};

// First matching rule wins; keys are lower-case words of the deck name.
const RULES: [RegExp, Art][] = [
  [
    /christmas|xmas|weihnacht/,
    { glyph: G.tree, from: "#2f9e6b", to: "#7a1f14" },
  ],
  [/rock|metal|punk/, { glyph: G.bolt, from: "#3a3a44", to: "#e5482f" }],
  [/schlager|love/, { glyph: G.heart, from: "#ff6f91", to: "#9c2a5a" }],
  [/sun|summer/, { glyph: G.sun, from: "#ffcf4a", to: "#ff7a3d" }],
  [/cheesy|disco|guilty/, { glyph: G.disco, from: "#c86bd9", to: "#3a2a7a" }],
  [/party|starter/, { glyph: G.popper, from: "#ff5ab4", to: "#ffb347" }],
  [
    /crowd|eurovision|karaoke/,
    { glyph: G.mic, from: "#69aee8", to: "#5a2a9a" },
  ],
  [
    /screen|movie|film|\btv\b/,
    { glyph: G.clapper, from: "#2c2c34", to: "#e9b44c" },
  ],
  [/radio/, { glyph: G.radio, from: "#e9b44c", to: "#8a4b1c" }],
  [/mix/, { glyph: G.cassette, from: "#3dbb8c", to: "#1f4a7a" }],
  [/smash|\bhits?\b/, { glyph: G.star, from: "#e5482f", to: "#e9b44c" }],
  [/number|\bones\b|chart/, { glyph: G.crown, from: "#f2c14e", to: "#b0702a" }],
  [
    /evergreen|classic|oldie/,
    { glyph: G.gramophone, from: "#b08a5a", to: "#4a3020" },
  ],
  [/essential/, { glyph: G.vinyl, from: "#e5482f", to: "#3a1410" }],
];

const FALLBACK: [string, string][] = [
  ["#69aee8", "#3a4f9a"],
  ["#3dbb8c", "#1f6a6a"],
  ["#c86bd9", "#5a2a7a"],
  ["#e9b44c", "#c0662b"],
];

export function deckArt(name: string, i = 0): Art {
  const n = name.toLowerCase();
  const hit = RULES.find(([re]) => re.test(n));
  if (hit) return hit[1];
  const [from, to] = FALLBACK[i % FALLBACK.length];
  return { glyph: G.note, from, to };
}

function Glyph({
  children,
  ...rest
}: { children: ReactNode } & Record<string, unknown>) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.8}
      strokeLinecap="round"
      strokeLinejoin="round"
      {...rest}
    >
      {children}
    </svg>
  );
}

export function DeckArt({ name, i = 0 }: { name: string; i?: number }) {
  const { glyph, from, to } = deckArt(name, i);
  return (
    <div
      className="art"
      style={{ background: `linear-gradient(135deg,${from},${to})` }}
      aria-hidden="true"
    >
      <Glyph className="art-bg" width={64} height={64}>
        {glyph}
      </Glyph>
      <Glyph className="art-fg" width={28} height={28}>
        {glyph}
      </Glyph>
    </div>
  );
}
