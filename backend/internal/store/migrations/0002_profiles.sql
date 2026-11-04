-- Profiles, paired devices and pairing codes (plan D3). No passwords: a device holds a signed cookie whose claim is
-- its id, and every request checks the id against devices, so removing the row signs that device out. A device
-- belongs to one profile at a time; "Who's playing?" on a shared device moves it to another. Times are unix ms.
CREATE TABLE profiles (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    avatar     TEXT NOT NULL,
    admin      INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE devices (
    id         TEXT PRIMARY KEY,
    profile_id TEXT NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    label      TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    last_seen  INTEGER NOT NULL
) STRICT;

CREATE INDEX devices_profile ON devices (profile_id);

-- A code a paired device shows: kind device adds a device to its profile, invite creates a new profile. Single use
-- (used_at) and short-lived (expires_at).
CREATE TABLE pairing_codes (
    code       TEXT PRIMARY KEY,
    kind       TEXT NOT NULL CHECK (kind IN ('device', 'invite')),
    profile_id TEXT NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL,
    used_at    INTEGER
) STRICT;
