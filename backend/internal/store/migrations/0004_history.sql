-- Songs played in a host's party games: the next game puts them at the back of the deck, so the same crowd does not
-- hear last week's songs first.
CREATE TABLE played_history (
    profile_id TEXT NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    song_id    TEXT NOT NULL REFERENCES songs (id),
    played_at  INTEGER NOT NULL
) STRICT;

CREATE INDEX played_history_profile ON played_history (profile_id, played_at);
