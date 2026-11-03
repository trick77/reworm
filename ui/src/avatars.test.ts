import {
  AVATARS,
  AVATAR_KEYS,
  avatarInfo,
  avatarUrl,
  isAvatarKey,
} from "./avatars";

// the shipped files, read through vite (the app's tsconfig has no node types)
const files = import.meta.glob<string>("../public/avatars/*.svg", {
  query: "?raw",
  import: "default",
  eager: true,
});
const svgOf = (key: string) => files[`../public/avatars/${key}.svg`];

describe("avatars", () => {
  it("is the set of 12 music objects", () => {
    expect(AVATAR_KEYS).toEqual([
      "vinyl",
      "cassette",
      "boombox",
      "mic",
      "worm",
      "disco",
      "guitar",
      "drum",
      "note",
      "keys",
      "speaker",
      "jukebox",
    ]);
    expect(new Set(AVATARS.map((a) => a.name)).size).toBe(12);
  });

  it("ships one 64×64 SVG per key, on its own background colour", () => {
    expect(Object.keys(files).sort()).toEqual(
      AVATAR_KEYS.map((k) => `../public/avatars/${k}.svg`).sort(),
    );
    for (const a of AVATARS) {
      const svg = svgOf(a.key);
      expect(svg).toMatch(
        /^<svg [^>]*width="64" height="64" viewBox="0 0 64 64"/,
      );
      expect(svg).toContain(`<circle cx="32" cy="32" r="32" fill="${a.bg}"/>`);
      expect(svg).toContain('clip-path="url(#c)"');
    }
  });

  it("resolves keys, falling back to the first avatar", () => {
    expect(isAvatarKey("disco")).toBe(true);
    expect(isAvatarKey("kazoo")).toBe(false);
    expect(avatarInfo("note").name).toBe("Quaver");
    expect(avatarInfo("kazoo").key).toBe("vinyl");
    expect(avatarUrl("keys")).toBe("/avatars/keys.svg");
    expect(avatarUrl("kazoo")).toBe("/avatars/vinyl.svg");
  });
});
