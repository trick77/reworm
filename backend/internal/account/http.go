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
	mux.HandleFunc("GET /api/profiles", s.Require(s.handleProfiles))
	mux.HandleFunc("POST /api/profiles/switch", s.Require(s.handleSwitch))
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

type profileListJSON struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

func (s *Service) handleProfiles(w http.ResponseWriter, r *http.Request, _ Identity) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT id, name, avatar FROM profiles ORDER BY created_at, rowid`)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	defer func() { _ = rows.Close() }()
	out := []profileListJSON{}
	for rows.Next() {
		var p profileListJSON
		if err := rows.Scan(&p.ID, &p.Name, &p.Avatar); err != nil {
			api.WriteError(w, r, err)
			return
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		api.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

type switchRequest struct {
	ProfileID string `json:"profileId"`
}

func (s *Service) handleSwitch(w http.ResponseWriter, r *http.Request, id Identity) {
	var req switchRequest
	if !api.Decode(w, r, &req) {
		return
	}
	// Switching never grants admin: the admin profile is reached only by pairing from an admin device.
	var admin bool
	err := s.db.QueryRowContext(r.Context(), `SELECT admin FROM profiles WHERE id = ?`, req.ProfileID).Scan(&admin)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		err = api.Fail(http.StatusNotFound, "unknown_profile", "no such profile")
	case err == nil && admin && req.ProfileID != id.ProfileID:
		err = api.Fail(http.StatusForbidden, "admin_profile", "pair this device from an admin device instead")
	case err == nil && req.ProfileID != id.ProfileID:
		err = store.InTx(r.Context(), s.db, func(tx *sql.Tx) error {
			if err := keepAdmin(r.Context(), tx, id.DeviceID); err != nil {
				return err
			}
			_, err := tx.ExecContext(r.Context(), `UPDATE devices SET profile_id = ? WHERE id = ?`, req.ProfileID, id.DeviceID)
			return err
		})
	}
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	s.answerMe(w, r, http.StatusOK, id.DeviceID)
}

var errLastAdmin = api.Fail(http.StatusConflict, "last_admin", "this is the admin profile's only device: pair another one first")

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// lastAdminDevice reports whether deviceID is the only device on an admin profile: moving or removing it would
// leave the server without an admin.
func lastAdminDevice(ctx context.Context, q querier, deviceID string) (bool, error) {
	var last bool
	err := q.QueryRowContext(ctx, `
SELECT p.admin AND (SELECT count(*) FROM devices o WHERE o.profile_id = d.profile_id) = 1
FROM devices d JOIN profiles p ON p.id = d.profile_id WHERE d.id = ?`, deviceID).Scan(&last)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return last, err
}

// keepAdmin refuses to move deviceID off its profile when it is the admin's last device.
func keepAdmin(ctx context.Context, q querier, deviceID string) error {
	last, err := lastAdminDevice(ctx, q, deviceID)
	if err == nil && last {
		err = errLastAdmin
	}
	return err
}

type deviceListJSON struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	LastSeen int64  `json:"lastSeen"`
	Current  bool   `json:"current"`
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
		if err := keepAdmin(r.Context(), tx, target); err != nil {
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
	if req.Kind != "device" && req.Kind != "invite" {
		api.WriteError(w, r, api.Fail(http.StatusBadRequest, "invalid_kind", `kind is "device" or "invite"`))
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
	Code   string `json:"code"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

var errBadCode = api.Fail(http.StatusNotFound, "invalid_code", "the code is wrong, used or expired")

func (s *Service) handleClaim(w http.ResponseWriter, r *http.Request) {
	var req claimRequest
	if !api.Decode(w, r, &req) {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	// The device this browser was paired as, if any: claiming replaces it rather than leave it listed.
	prev, prevErr := s.Identify(r)
	var deviceID string
	err := store.InTx(r.Context(), s.db, func(tx *sql.Tx) error {
		var kind, profileID string
		err := tx.QueryRowContext(r.Context(), `UPDATE pairing_codes SET used_at = ?
WHERE code = ? AND used_at IS NULL AND expires_at > ? RETURNING kind, profile_id`,
			s.now().UnixMilli(), code, s.now().UnixMilli()).Scan(&kind, &profileID)
		if errors.Is(err, sql.ErrNoRows) {
			return errBadCode
		}
		if err != nil {
			return err
		}
		if kind == "invite" {
			name, err := CheckName(req.Name)
			if err != nil {
				return err
			}
			if err := CheckAvatar(req.Avatar); err != nil {
				return err
			}
			if profileID, err = s.newProfile(r.Context(), tx, name, req.Avatar, false); err != nil {
				return err
			}
		}
		if prevErr == nil {
			if profileID != prev.ProfileID {
				if err := keepAdmin(r.Context(), tx, prev.DeviceID); err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(r.Context(), `DELETE FROM devices WHERE id = ?`, prev.DeviceID); err != nil {
				return err
			}
		}
		deviceID, err = s.newDevice(r.Context(), tx, profileID, Label(r.UserAgent()))
		return err
	})
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	s.answerMe(w, r, http.StatusCreated, deviceID)
}
