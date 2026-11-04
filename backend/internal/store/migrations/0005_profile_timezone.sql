-- The IANA zone a profile trains in (sent by its client as X-Timezone): the daily streak counts the player's days,
-- not the server's.
ALTER TABLE profiles ADD COLUMN tz TEXT NOT NULL DEFAULT 'UTC';
