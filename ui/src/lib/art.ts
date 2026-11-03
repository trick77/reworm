// Placeholder art from a string, as in the mock: a stable hue pair per song, so a missing cover still
// looks like the same record every time.

export function hash(s: string): number {
  let h = 7;
  for (const c of s) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return h;
}

export function artGradient(s: string): string {
  const h = hash(s) % 360;
  const h2 = (h + 40 + (hash(s + "x") % 80)) % 360;
  return `linear-gradient(135deg,hsl(${h} 62% 52%),hsl(${h2} 55% 28%))`;
}

// initials: the first letters of the first two words, "Mark Ronson, Bruno Mars" → "MR", "Die Ärzte" → "DÄ".
export function initials(s: string): string {
  return s
    .split(/[\s,]+/)
    .filter((w) => /^[\p{L}\p{N}(]/u.test(w))
    .slice(0, 2)
    .map((w) => [...w.replace(/[^\p{L}\p{N}]/gu, "")][0] ?? "")
    .join("")
    .toUpperCase();
}
