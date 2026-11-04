-- Training (side feature). A session is server-side so a reload resumes at the current card: its queue, answers
-- and counters are one JSON document (state), read and written whole by the one device playing it.
CREATE TABLE train_sessions (
    id         TEXT PRIMARY KEY,
    profile_id TEXT NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    state      TEXT NOT NULL
) STRICT;

CREATE INDEX train_sessions_profile ON train_sessions (profile_id, updated_at);

-- Spaced repetition per profile and song: box counts first-try hits in a row (0 after a miss), due_at is when the
-- song is next worth asking. No row = a new song. attempts/correct feed the decade accuracy.
CREATE TABLE train_progress (
    profile_id TEXT NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    song_id    TEXT NOT NULL REFERENCES songs (id),
    box        INTEGER NOT NULL,
    due_at     INTEGER NOT NULL,
    attempts   INTEGER NOT NULL,
    correct    INTEGER NOT NULL,
    last_at    INTEGER NOT NULL,
    PRIMARY KEY (profile_id, song_id)
) STRICT;

-- XP and the daily streak per profile. last_day is the local date (YYYY-MM-DD) of the last answer.
CREATE TABLE train_profiles (
    profile_id  TEXT PRIMARY KEY REFERENCES profiles (id) ON DELETE CASCADE,
    xp          INTEGER NOT NULL DEFAULT 0,
    streak_days INTEGER NOT NULL DEFAULT 0,
    last_day    TEXT
) STRICT;
