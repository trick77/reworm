-- One row per song of the dataset (data/songs.jsonl, upserted at boot). id is stable across re-imports: the
-- MusicBrainz recording MBID of the earliest recording, else a normalized artist|title slug. year is the initial
-- release year, never a re-release's. genres, categories and sets are JSON arrays of strings.
CREATE TABLE songs (
    id           TEXT PRIMARY KEY,
    mbid         TEXT,
    artist       TEXT NOT NULL,
    title        TEXT NOT NULL,
    year         INTEGER NOT NULL,
    year_source  TEXT NOT NULL,
    isrc         TEXT,
    apple_id     TEXT,
    storefront   TEXT,
    apple_album  TEXT,
    cover_url    TEXT,
    cover_source TEXT,
    genres       TEXT NOT NULL DEFAULT '[]',
    categories   TEXT NOT NULL DEFAULT '[]',
    sets         TEXT NOT NULL DEFAULT '[]',
    listeners    INTEGER
) STRICT;
