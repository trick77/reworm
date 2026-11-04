// The fixture songs the server runs on (BACKEND_DEV_SONGS), so a test knows each round's answer.
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export type FixtureSong = {
  id: string;
  artist: string;
  title: string;
  year: number;
};

const file = path.join(
  path.dirname(fileURLToPath(import.meta.url)),
  "fixtures/songs.jsonl",
);

export const SONGS: FixtureSong[] = readFileSync(file, "utf8")
  .split("\n")
  .filter((l) => l.trim())
  .map((l) => JSON.parse(l) as FixtureSong);

// songOf is the song a dev clip URL plays: /api/dev/previews/<id>.wav
export function songOf(previewUrl: string): FixtureSong {
  const id = decodeURIComponent(
    previewUrl
      .split("/")
      .pop()!
      .replace(/\.wav$/, ""),
  );
  const s = SONGS.find((x) => x.id === id);
  if (!s) throw new Error(`no fixture song for ${previewUrl}`);
  return s;
}
