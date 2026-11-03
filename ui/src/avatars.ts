// The avatar set: 12 comic music objects, our own drawings (the mock's AVATARS), shipped as static SVGs
// under public/avatars/<key>.svg. bg is each drawing's circle colour, used for the selection ring.

export const AVATARS = [
  { key: "vinyl", name: "Vinyl Vince", bg: "#e9b44c" },
  { key: "cassette", name: "Cass Ette", bg: "#69aee8" },
  { key: "boombox", name: "Boom Bo", bg: "#c86bd9" },
  { key: "mic", name: "Mic Drop", bg: "#3dbb8c" },
  { key: "worm", name: "Ear Worm", bg: "#ffd166" },
  { key: "disco", name: "Disco Dot", bg: "#7b5cff" },
  { key: "guitar", name: "Strummy", bg: "#e5482f" },
  { key: "drum", name: "Ba-Dum", bg: "#3dbb8c" },
  { key: "note", name: "Quaver", bg: "#69aee8" },
  { key: "keys", name: "Keys", bg: "#c86bd9" },
  { key: "speaker", name: "Woofer", bg: "#ffd166" },
  { key: "jukebox", name: "Jukey", bg: "#e5482f" },
] as const;

export type AvatarKey = (typeof AVATARS)[number]["key"];
export type AvatarInfo = (typeof AVATARS)[number];

export const AVATAR_KEYS: readonly AvatarKey[] = AVATARS.map((a) => a.key);

export function isAvatarKey(k: string): k is AvatarKey {
  return (AVATAR_KEYS as readonly string[]).includes(k);
}

// avatarInfo falls back to the first avatar for an unknown key (a server value from a newer set).
export function avatarInfo(key: string): AvatarInfo {
  return AVATARS.find((a) => a.key === key) ?? AVATARS[0];
}

export function avatarUrl(key: string): string {
  return `/avatars/${avatarInfo(key).key}.svg`;
}
