// The logo mark: a worm, white on the seal (the mock's WORM).
export function WormLogo({ size = 26 }: { size?: number }) {
  return (
    <svg
      viewBox="0 0 32 32"
      width={size}
      height={size}
      fill="none"
      stroke="#fff"
      strokeWidth={3.2}
      strokeLinecap="round"
      aria-hidden="true"
    >
      <path d="M4 18c3-8 6-8 8 0s5 8 8 0 4-6 6-4" />
      <circle cx="27" cy="12" r="2.4" fill="#fff" stroke="none" />
    </svg>
  );
}
