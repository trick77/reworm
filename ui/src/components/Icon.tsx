// The mock's icon set (ICON in docs/mock/index.html), 24×24, currentColor.
import type { ReactElement } from "react";

const line = {
  fill: "none",
  stroke: "currentColor",
  strokeLinecap: "round",
  strokeLinejoin: "round",
} as const;

const PATHS = {
  home: (
    <path
      {...line}
      strokeWidth={1.8}
      strokeLinecap="butt"
      strokeLinejoin="miter"
      d="M3 10.5 12 3l9 7.5V20a1 1 0 0 1-1 1h-5v-6h-6v6H4a1 1 0 0 1-1-1z"
    />
  ),
  train: (
    <g {...line} strokeWidth={1.8} strokeLinecap="butt" strokeLinejoin="miter">
      <circle cx="12" cy="12" r="9" />
      <circle cx="12" cy="12" r="2.5" />
      <path d="M12 3v3M12 18v3" />
    </g>
  ),
  gear: (
    <g {...line} strokeWidth={1.8} strokeLinecap="butt" strokeLinejoin="miter">
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z" />
    </g>
  ),
  party: (
    <g {...line} strokeWidth={1.8} strokeLinejoin="miter">
      <circle cx="8" cy="9" r="3" />
      <circle cx="16.5" cy="9.5" r="2.5" />
      <path d="M2.5 20c.6-3.4 2.8-5.5 5.5-5.5s4.9 2.1 5.5 5.5M14 15.2c.8-.5 1.6-.7 2.5-.7 2.3 0 4.2 1.8 4.8 4.8" />
    </g>
  ),
  left: <path {...line} strokeWidth={2.4} d="M15 5l-7 7 7 7" />,
  right: <path {...line} strokeWidth={2.4} d="M9 5l7 7-7 7" />,
  target: (
    <g fill="none" stroke="currentColor" strokeWidth={2.2}>
      <circle cx="12" cy="12" r="8" />
      <circle cx="12" cy="12" r="3" fill="currentColor" />
    </g>
  ),
  x: <path {...line} strokeWidth={2} d="M6 6l12 12M18 6 6 18" />,
  back: <path {...line} strokeWidth={2} d="M15 5l-7 7 7 7" />,
  replay: (
    <g {...line} strokeWidth={2} strokeLinejoin="miter">
      <path d="M3 12a9 9 0 1 0 3-6.7L3 8" />
      <path d="M3 3v5h5" />
    </g>
  ),
  plus: (
    <path
      {...line}
      strokeWidth={2}
      strokeLinejoin="miter"
      d="M12 5v14M5 12h14"
    />
  ),
  pause: (
    <g fill="currentColor">
      <rect x="6" y="5" width="4" height="14" rx="1" />
      <rect x="14" y="5" width="4" height="14" rx="1" />
    </g>
  ),
  play: <path fill="currentColor" d="M7 5v14l12-7z" />,
  qr: (
    <g {...line} strokeWidth={1.8} strokeLinecap="butt" strokeLinejoin="miter">
      <rect x="3" y="3" width="7" height="7" rx="1" />
      <rect x="14" y="3" width="7" height="7" rx="1" />
      <rect x="3" y="14" width="7" height="7" rx="1" />
      <path d="M14 14h3v3M21 14v7h-7" />
    </g>
  ),
  flame: (
    <path
      fill="#e9b44c"
      d="M12 2s5 4.5 5 10a5 5 0 0 1-10 0c0-2 1-3.5 1-3.5S9 11 11 11c0-4 1-9 1-9z"
    />
  ),
} satisfies Record<string, ReactElement>;

export type IconName = keyof typeof PATHS;

export function Icon({ name, size }: { name: IconName; size?: number }) {
  return (
    <svg viewBox="0 0 24 24" width={size} height={size} aria-hidden="true">
      {PATHS[name]}
    </svg>
  );
}
