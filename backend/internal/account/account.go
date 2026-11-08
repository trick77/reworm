// Package account: no passwords, paired devices. A device holds a kit/auth session cookie whose claim is
// its device id; every request checks the id against the devices table, so a removed device is signed out at once
// (a signed cookie cannot be revoked on its own). The first device pairs with a one-time token from the server log;
// later ones with a short-lived code a paired device shows.
package account

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/trick77/ship/kit/auth"
	"github.com/trick77/ship/kit/fsx"
	"github.com/trick77/ship/kit/ratelimit"

	"github.com/trick77/reworm/internal/api"
)

// Avatars are the avatar keys a profile or a guest may pick (the UI's comic music objects).
var Avatars = []string{"vinyl", "cassette", "boombox", "mic", "worm", "disco", "guitar", "drum", "note", "keys",
	"speaker", "jukebox"}

// MaxName is the longest name, in characters.
const MaxName = 24

// DeviceTTL is how long a device stays paired without pairing again.
const DeviceTTL = 365 * 24 * time.Hour

// PairTTL is how long a pairing code works.
const PairTTL = 5 * time.Minute

// seenEvery throttles the last_seen write: a party's requests must not each write the database.
const seenEvery = 5 * time.Minute

// adminGroup marks the admin's cookie; authorization reads the profile row, the group is informational.
const adminGroup = "admin"

// CheckName trims a display name and checks it: 1 to MaxName printable characters.
func CheckName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxName {
		return "", api.Fail(http.StatusBadRequest, "invalid_name", fmt.Sprintf("a name is 1 to %d characters", MaxName))
	}
	for _, r := range name {
		if !unicode.IsPrint(r) {
			return "", api.Fail(http.StatusBadRequest, "invalid_name", "a name is printable characters only")
		}
	}
	return name, nil
}

// CheckAvatar checks an avatar key.
func CheckAvatar(avatar string) error {
	if !slices.Contains(Avatars, avatar) {
		return api.Fail(http.StatusBadRequest, "invalid_avatar", "unknown avatar")
	}
	return nil
}

// Identity is the paired device a request comes from and the profile it plays as.
type Identity struct {
	DeviceID    string
	DeviceLabel string
	ProfileID   string
	Name        string
	Avatar      string
	Admin       bool
}

// Options configures New.
type Options struct {
	DB *sql.DB
	// Key signs the device cookies (LoadKey); its loss signs every device out.
	Key []byte
	// Secure marks cookies Secure: true behind TLS (PUBLIC_FQDN set).
	Secure bool
	// PublicURL is "https://<PUBLIC_FQDN>" for pairing links; "" builds them from the request's host.
	PublicURL string
	// ClientKey is the rate-limit key of a request (the client IP behind the configured proxies).
	ClientKey func(*http.Request) string
	// Now is the clock; nil is time.Now.
	Now func() time.Time
}

// Service is the account API and the identity every other handler asks.
type Service struct {
	db        *sql.DB
	sessions  *auth.Sessions
	publicURL string
	now       func() time.Time
	limit     func(http.Handler) http.Handler

	mu         sync.Mutex
	setupToken string
}

// New returns the service.
func New(o Options) (*Service, error) {
	s, err := auth.NewSessions(auth.SessionConfig{
		Name: "reworm_device", Key: auth.DeriveKey(hex.EncodeToString(o.Key), "device"), TTL: DeviceTTL,
		Secure: o.Secure, KeepGroup: adminGroup,
	})
	if err != nil {
		return nil, err
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.ClientKey == nil {
		o.ClientKey = ratelimit.ClientKey(0)
	}
	// Pairing and setup: a person types a code now and then; ten tries a minute is plenty and stops guessing.
	lim := ratelimit.New(ratelimit.Options{Rate: 10.0 / 60, Burst: 10, Now: o.Now})
	return &Service{db: o.DB, sessions: s, publicURL: o.PublicURL, now: o.Now, limit: lim.Middleware(o.ClientKey)}, nil
}

// LoadKey reads the cookie signing key at path, creating a random one on first boot. It lives in the data dir so
// devices stay paired across restarts.
func LoadKey(path string) ([]byte, error) {
	b, err := os.ReadFile(path) //nolint:gosec // the configured data dir
	if err == nil {
		if len(b) < 32 {
			return nil, fmt.Errorf("session key %s: %d bytes, want 32", path, len(b))
		}
		return b, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	b = auth.RandomKey()
	if err := fsx.WriteFileAtomic(path, b, 0o600); err != nil {
		return nil, err
	}
	return b, nil
}

// SetupToken returns the one-time first-device token, minting it when no profile exists yet; "" once one does.
func (s *Service) SetupToken(ctx context.Context) (string, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM profiles`).Scan(&n); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if n > 0 {
		s.setupToken = ""
		return "", nil
	}
	if s.setupToken == "" {
		s.setupToken = rand.Text()
	}
	return s.setupToken, nil
}

// Identify is the request's paired device, api.ErrUnauthorized without one (no cookie, a bad one, or a device
// removed since).
func (s *Service) Identify(r *http.Request) (Identity, error) {
	c, ok := s.sessions.Read(r)
	if !ok {
		return Identity{}, api.ErrUnauthorized
	}
	var id Identity
	var seen int64
	err := s.db.QueryRowContext(r.Context(), `
SELECT d.id, d.label, d.last_seen, p.id, p.name, p.avatar, p.admin
FROM devices d JOIN profiles p ON p.id = d.profile_id WHERE d.id = ?`, c.Subject).
		Scan(&id.DeviceID, &id.DeviceLabel, &seen, &id.ProfileID, &id.Name, &id.Avatar, &id.Admin)
	if errors.Is(err, sql.ErrNoRows) {
		return Identity{}, api.ErrUnauthorized
	}
	if err != nil {
		return Identity{}, err
	}
	if now := s.now(); now.Sub(time.UnixMilli(seen)) >= seenEvery {
		if _, err := s.db.ExecContext(r.Context(), `UPDATE devices SET last_seen = ? WHERE id = ?`, now.UnixMilli(), id.DeviceID); err != nil {
			return Identity{}, err
		}
	}
	return id, nil
}

// Require runs h for a paired device and answers 401 otherwise.
func (s *Service) Require(h func(http.ResponseWriter, *http.Request, Identity)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := s.Identify(r)
		if err != nil {
			api.WriteError(w, r, err)
			return
		}
		h(w, r, id)
	}
}

// issue writes the device cookie.
func (s *Service) issue(w http.ResponseWriter, deviceID string, admin bool) error {
	c := auth.Claims{Subject: deviceID}
	if admin {
		c.Groups = []string{adminGroup}
	}
	return s.sessions.Issue(w, c)
}

// newDevice adds a device for profileID inside tx.
func (s *Service) newDevice(ctx context.Context, tx *sql.Tx, profileID, label string) (string, error) {
	id := api.ID()
	now := s.now().UnixMilli()
	_, err := tx.ExecContext(ctx, `INSERT INTO devices (id, profile_id, label, created_at, last_seen) VALUES (?, ?, ?, ?, ?)`,
		id, profileID, label, now, now)
	return id, err
}

// newProfile adds a profile inside tx.
func (s *Service) newProfile(ctx context.Context, tx *sql.Tx, name, avatar string, admin bool) (string, error) {
	id := api.ID()
	_, err := tx.ExecContext(ctx, `INSERT INTO profiles (id, name, avatar, admin, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, name, avatar, admin, s.now().UnixMilli())
	return id, err
}

// Label guesses a device's label from its User-Agent.
func Label(userAgent string) string {
	for _, k := range []struct{ needle, label string }{
		{"iPhone", "iPhone"}, {"iPad", "iPad"}, {"Android", "Android"}, {"Macintosh", "Mac"},
		{"Windows", "Windows"}, {"CrOS", "Chromebook"}, {"Linux", "Linux"},
	} {
		if strings.Contains(userAgent, k.needle) {
			return k.label
		}
	}
	return "Browser"
}
