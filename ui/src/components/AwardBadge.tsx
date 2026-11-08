import { useId, type ReactNode } from "react";

// Training levels are record awards, lowest first.
export const AWARD_TIERS = [
  "bronze",
  "silver",
  "gold",
  "platinum",
  "diamond",
] as const;
export type AwardTier = (typeof AWARD_TIERS)[number];

const COLORS: Record<AwardTier, [string, string]> = {
  bronze: ["#c98a52", "#7a4a22"],
  silver: ["#e2e6ea", "#8d96a3"],
  gold: ["#f3d27a", "#b8862b"],
  platinum: ["#f5f7fa", "#aab4c3"],
  diamond: ["#bfeaff", "#5aa9e6"],
};

export function AwardBadge({
  tier,
  children,
}: {
  tier: AwardTier;
  children?: ReactNode;
}) {
  const id = useId();
  const [a, b] = COLORS[tier];
  return (
    <span className="belt" data-tier={tier}>
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <defs>
          <linearGradient id={id} x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stopColor={a} />
            <stop offset="1" stopColor={b} />
          </linearGradient>
        </defs>
        <circle cx="12" cy="12" r="11" fill={`url(#${id})`} stroke="#0006" />
        <circle cx="12" cy="12" r="7.5" fill="none" stroke="#0003" />
        <circle cx="12" cy="12" r="5" fill="none" stroke="#0003" />
        <circle cx="12" cy="12" r="2.6" fill="#13110f" />
        <path
          d="M6 7.5a8 8 0 0 1 4-3"
          stroke="#fff"
          strokeWidth="1.4"
          strokeLinecap="round"
          opacity=".7"
        />
      </svg>
      {children}
    </span>
  );
}
