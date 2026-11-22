package account

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/trick77/ship/kit/httpx"
	"github.com/trick77/ship/kit/secret"

	"github.com/trick77/reworm/internal/api"
	"github.com/trick77/reworm/internal/store"
)

// Mount adds the profile, device and pairing routes.
func (s *Service) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/me", s.Require(s.handleMe))
	mux.HandleFunc("PATCH /api/me", s.Require(s.handleUpdateMe))
	mux.Handle("POST /api/setup", s.limit(http.HandlerFunc(s.handleSetup)))
	mux.HandleFunc("POST /api/profiles", s.handleCreate)
	mux.HandleFunc("GET /api/devices", s.Require(s.handleDevices))
	mux.HandleFunc("DELETE /api/devices/{id}", s.Require(s.handleDeleteDevice))
	mux.Handle("POST /api/pair", s.limit(s.Require(s.handlePair)))
	mux.Handle("POST /api/pair/claim", s.limit(http.HandlerFunc(s.handleClaim)))
}

type profileJSON struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Admin  bool   `json:"admin"`
}

type deviceJSON struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type meJSON struct {
	Profile profileJSON `json:"profile"`
	Device  deviceJSON  `json:"device"`
}

func meOf(id Identity) meJSON {
	return meJSON{
		Profile: profileJSON{ID: id.ProfileID, Name: id.Name, Avatar: id.Avatar, Admin: id.Admin},
		Device:  deviceJSON{ID: id.DeviceID, Label: id.DeviceLabel},
	}
}

func (s *Service) handleMe(w http.ResponseWriter, _ *http.Request, id Identity) {
	httpx.WriteJSON(w, http.StatusOK, meOf(id))
}

// identity is the Identity of a device by id, after a write that changed it.
func (s *Service) identity(ctx context.Context, deviceID string) (Identity, error) {
	var id Identity
	err := s.db.QueryRowContext(ctx, `
SELECT d.id, d.label, p.id, p.name, p.avatar, p.admin
FROM devices d JOIN profiles p ON p.id = d.profile_id WHERE d.id = ?`, deviceID).
		Scan(&id.DeviceID, &id.DeviceLabel, &id.ProfileID, &id.Name, &id.Avatar, &id.Admin)
	return id, err
}

// answerMe issues the device cookie and answers /api/me's body for deviceID.
func (s *Service) answerMe(w http.ResponseWriter, r *http.Request, status int, deviceID string) {
	id, err := s.identity(r.Context(), deviceID)
	if err == nil {
		err = s.issue(w, id.DeviceID, id.Admin)
	}
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, status, meOf(id))
}

type setupRequest struct {
	Token  string `json:"token"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

var errSetupDone = api.Fail(http.StatusGone, "setup_done", "this server is set up; pair from a paired device")

func (s *Service) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req setupRequest
	if !api.Decode(w, r, &req) {
		return
	}
	token, err := s.SetupToken(r.Context())
	switch {
	case err != nil:
	case token == "":
		err = errSetupDone
	case !secret.Match(req.Token, token):
		err = api.Fail(http.StatusForbidden, "invalid_token", "wrong setup token")
	}
	var name string
	if err == nil {
		name, err = CheckName(req.Name)
	}
	if err == nil {
		err = CheckAvatar(req.Avatar)
	}
	var deviceID string
	if err == nil {
		err = store.InTx(r.Context(), s.db, func(tx *sql.Tx) error {
			// Inside the write transaction: two setups at once cannot both make an admin.
			var n int
			if err := tx.QueryRowContext(r.Context(), `SELECT count(*) FROM profiles`).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return errSetupDone
			}
			pid, err := s.newProfile(r.Context(), tx, name, req.Avatar, true)
			if err != nil {
				return err
			}
			deviceID, err = s.newDevice(r.Context(), tx, pid, Label(r.UserAgent()))
			return err
		})
	}
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	s.mu.Lock()
	s.setupToken = ""
	s.mu.Unlock()
	s.answerMe(w, r, http.StatusCreated, deviceID)
}

type updateMeRequest struct {
	Name   *string `json:"name"`
	Avatar *string `json:"avatar"`
}

func (s *Service) handleUpdateMe(w http.ResponseWriter, r *http.Request, id Identity) {
	var req updateMeRequest
	if !api.Decode(w, r, &req) {
		return
	}
	var err error
	if req.Name != nil {
		id.Name, err = CheckName(*req.Name)
	}
	if err == nil && req.Avatar != nil {
		err = CheckAvatar(*req.Avatar)
		id.Avatar = *req.Avatar
	}
	if err == nil {
		_, err = s.db.ExecContext(r.Context(), `UPDATE profiles SET name = ?, avatar = ? WHERE id = ?`, id.Name, id.Avatar, id.ProfileID)
	}
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, meOf(id))
}

type createRequest struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	// Leave: this browser may leave its profile although it is that profile's last device; the profile goes.
	Leave bool `json:"leave,omitempty"`
}

// handleCreate makes a new profile for this browser: anyone may, once the setup link has made the admin. Profiles
// are private from then on: no list, no switching; another device reaches one only with its device code.
// The limit counts only profiles made: a typo or a refusal costs a party behind one router nothing.
func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) {
	key := s.clientKey(r)
	if ok, wait := s.creates.Take(key, 1, 0); !ok {
		httpx.TooManyRequests(w, wait)
		return
	}
	if ok, wait := s.allCreates.Take(allKey, 1, 0); !ok {
		s.creates.Charge(key, -1)
		httpx.TooManyRequests(w, wait)
		return
	}
	refund := func() {
		s.creates.Charge(key, -1)
		s.allCreates.Charge(allKey, -1)
	}
	var req createRequest
	if !api.Decode(w, r, &req) {
		refund()
		return
	}
	name, err := CheckName(req.Name)
	if err == nil {
		err = CheckAvatar(req.Avatar)
	}
	var prev *Identity
	if err == nil {
		prev, err = s.previous(r)
	}
	var deviceID string
	if err == nil {
		err = store.InTx(r.Context(), s.db, func(tx *sql.Tx) error {
			// Inside the write transaction: a profile made before setup would leave the server without an admin.
			var n int
			if err := tx.QueryRowContext(r.Context(), `SELECT count(*) FROM profiles`).Scan(&n); err != nil {
				return err
			}
			if n == 0 {
				return errSetupPending
			}
			pid, err := s.newProfile(r.Context(), tx, name, req.Avatar, false)
			if err != nil {
				return err
			}
			deviceID, err = s.attach(r, tx, prev, pid, req.Leave)
			return err
		})
	}
	if err != nil {
		refund()
		api.WriteError(w, r, err)
		return
	}
	s.answerMe(w, r, http.StatusCreated, deviceID)
}

// allKey is the one bucket of allCreates.
const allKey = "*"

// previous is the device this browser is paired as, nil when it is not. Only a refused cookie counts as unpaired: a
// failed lookup must not skip removing the old device.
func (s *Service) previous(r *http.Request) (*Identity, error) {
	id, err := s.Identify(r)
	if errors.Is(err, api.ErrUnauthorized) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

var (
	errLastAdmin  = api.Fail(http.StatusConflict, "last_admin", "this is the admin profile's only device: pair another one first")
	errLastDevice = api.Fail(http.StatusConflict, "last_device", "this is the profile's only device: leaving deletes the profile and its progress")
)

// attach adds this browser as a device of profileID inside tx. A browser paired before drops its old device row
// rather than leave it listed. Its profile's last device leaving strands the profile (no list, no switching leads
// back): refused (last_device) unless leave, then the profile goes too; the admin's last device never leaves
// (last_admin).
func (s *Service) attach(r *http.Request, tx *sql.Tx, prev *Identity, profileID string, leave bool) (string, error) {
	ctx := r.Context()
	if prev != nil {
		var strand bool
		if profileID != prev.ProfileID {
			last, admin, err := lastDevice(ctx, tx, prev.DeviceID)
			switch {
			case err != nil:
				return "", err
			case last && admin:
				return "", errLastAdmin
			case last && !leave:
				return "", errLastDevice
			}
			strand = last
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM devices WHERE id = ?`, prev.DeviceID); err != nil {
			return "", err
		}
		if strand {
			// cascades to its codes, training and history
			if _, err := tx.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, prev.ProfileID); err != nil {
				return "", err
			}
		}
	}
	return s.newDevice(ctx, tx, profileID, Label(r.UserAgent()))
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// lastDevice reports whether deviceID is the only device of its profile, and whether that profile is the admin.
func lastDevice(ctx context.Context, q querier, deviceID string) (last, admin bool, err error) {
	err = q.QueryRowContext(ctx, `
SELECT (SELECT count(*) FROM devices o WHERE o.profile_id = d.profile_id) = 1, p.admin
FROM devices d JOIN profiles p ON p.id = d.profile_id WHERE d.id = ?`, deviceID).Scan(&last, &admin)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	return last, admin, err
}

// keepLast refuses to remove deviceID when it is its profile's last device: the server would lose its admin
// (last_admin), any other profile would be stranded (last_device; leaving it is attach's, with the profile).
func keepLast(ctx context.Context, q querier, deviceID string) error {
	last, admin, err := lastDevice(ctx, q, deviceID)
	switch {
	case err != nil:
		return err
	case last && admin:
		return errLastAdmin
	case last:
		return errLastDevice
	}
	return nil
}

func (s *Service) handleDevices(w http.ResponseWriter, r *http.Request, id Identity) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT id, label, last_seen FROM devices WHERE profile_id = ?
ORDER BY last_seen DESC, id`, id.ProfileID)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	defer func() { _ = rows.Close() }()
	out := []deviceListJSON{}
	for rows.Next() {
		var d deviceListJSON
		if err := rows.Scan(&d.ID, &d.Label, &d.LastSeen); err != nil {
			api.WriteError(w, r, err)
			return
		}
		d.Current = d.ID == id.DeviceID
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		api.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// handleDeleteDevice removes one of the profile's devices (the admin: any device), signing it out.
func (s *Service) handleDeleteDevice(w http.ResponseWriter, r *http.Request, id Identity) {
	target := r.PathValue("id")
	err := store.InTx(r.Context(), s.db, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(r.Context(), `SELECT count(*) FROM devices WHERE id = ? AND (profile_id = ? OR ?)`,
			target, id.ProfileID, id.Admin).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return api.Fail(http.StatusNotFound, "unknown_device", "no such device")
		}
		if err := keepLast(r.Context(), tx, target); err != nil {
			return err
		}
		_, err := tx.ExecContext(r.Context(), `DELETE FROM devices WHERE id = ?`, target)
		return err
	})
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	if target == id.DeviceID {
		s.sessions.Clear(w)
	}
	w.WriteHeader(http.StatusNoContent)
}

type pairRequest struct {
	Kind string `json:"kind"`
}

type pairJSON struct {
	Code      string `json:"code"`
	URL       string `json:"url"`
	ExpiresAt int64  `json:"expiresAt"`
}

func (s *Service) handlePair(w http.ResponseWriter, r *http.Request, id Identity) {
	var req pairRequest
	if !api.Decode(w, r, &req) {
		return
	}
	if req.Kind != "device" {
		api.WriteError(w, r, api.Fail(http.StatusBadRequest, "invalid_kind", `kind is "device"`))
		return
	}
	now := s.now()
	code, exp := api.Code(6), now.Add(PairTTL)
	err := store.InTx(r.Context(), s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `DELETE FROM pairing_codes WHERE expires_at <= ? OR used_at IS NOT NULL`, now.UnixMilli()); err != nil {
			return err
		}
		_, err := tx.ExecContext(r.Context(), `INSERT INTO pairing_codes (code, kind, profile_id, expires_at) VALUES (?, ?, ?, ?)`,
			code, req.Kind, id.ProfileID, exp.UnixMilli())
		return err
	})
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	// Never from the Host header, which the caller picks: absolute only with the configured PUBLIC_FQDN, else the
	// path, which the client prefixes with its own origin.
	httpx.WriteJSON(w, http.StatusCreated, pairJSON{Code: code, URL: s.publicURL + "/pair/" + code, ExpiresAt: exp.UnixMilli()})
}

type claimRequest struct {
	Code string `json:"code"`
	// Leave: as in createRequest; a refusal rolls back, the code stays usable.
	Leave bool `json:"leave,omitempty"`
}

var errBadCode = api.Fail(http.StatusNotFound, "invalid_code", "the code is wrong, used or expired")

func (s *Service) handleClaim(w http.ResponseWriter, r *http.Request) {
	var req claimRequest
	if !api.Decode(w, r, &req) {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	// The device this browser was paired as, if any: claiming replaces it rather than leave it listed.
	prev, err := s.previous(r)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	var deviceID string
	err = store.InTx(r.Context(), s.db, func(tx *sql.Tx) error {
		// kind: an invite code left from before invites went away is no device code
		var profileID string
		err := tx.QueryRowContext(r.Context(), `UPDATE pairing_codes SET used_at = ?
WHERE code = ? AND kind = 'device' AND used_at IS NULL AND expires_at > ? RETURNING profile_id`,
			s.now().UnixMilli(), code, s.now().UnixMilli()).Scan(&profileID)
		if errors.Is(err, sql.ErrNoRows) {
			return errBadCode
		}
		if err != nil {
			return err
		}
		deviceID, err = s.attach(r, tx, prev, profileID, req.Leave)
		return err
	})
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	s.answerMe(w, r, http.StatusCreated, deviceID)
}
