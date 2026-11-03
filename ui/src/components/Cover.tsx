import { useState, type ReactNode } from "react";
import { artGradient } from "../lib/art";
import { cx } from "../lib/cx";

// Cover is the original album cover, or the mock's record-sleeve placeholder when there is none (the
// import found no original cover) or the image fails to load.
export function Cover({
  src,
  title,
  artist,
  album,
  size,
  mini,
  flip,
}: {
  src?: string | null;
  title: string;
  artist: string;
  album: string;
  size?: number;
  mini?: boolean;
  flip?: boolean;
}) {
  const [failed, setFailed] = useState<string | null>(null);
  const dims = size ? { width: size, height: size } : undefined;
  const cls = cx("cover", mini && "mini", flip && "flip");
  // only the last bracket group goes ("(2011 Remaster)"), never a title's own "(What's the Story)"
  const albumName = album.replace(/\s*\([^()]*\)$/, "");
  if (src && src !== failed) {
    return (
      <div className={cx(cls, "img")} style={dims}>
        <img
          src={src}
          alt={`${albumName} by ${artist}`}
          onError={() => setFailed(src)}
        />
      </div>
    );
  }
  return (
    <div
      className={cls}
      style={{ ...dims, background: artGradient(title + artist) }}
      role="img"
      aria-label={`${albumName} by ${artist}`}
      data-placeholder=""
    >
      <div className="disc" />
      <div className="ca">{artist}</div>
      <div className="cb">{albumName}</div>
    </div>
  );
}

// SongHead is the reveal's top row: cover, title, artist, an optional line under them.
export function SongHead({
  cover,
  title,
  artist,
  meta,
  center,
}: {
  cover: ReactNode;
  title: string;
  artist: string;
  meta?: ReactNode;
  center?: boolean;
}) {
  return (
    <div
      className="reveal-top"
      style={center ? { justifyContent: "center" } : undefined}
    >
      {cover}
      <div style={{ minWidth: 0, paddingBottom: 4 }}>
        <div className="song-t">{title}</div>
        <div className="song-a">{artist}</div>
        {meta && (
          <div className="sub" style={{ fontSize: 12.5, marginTop: 6 }}>
            {meta}
          </div>
        )}
      </div>
    </div>
  );
}
