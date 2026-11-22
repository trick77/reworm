package account

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/trick77/ship/kit/auth"

	"github.com/trick77/reworm/internal/store"
)

type env struct {
	t   *testing.T
	svc *Service
	mux *http.ServeMux
	now time.Time
	mu  sync.Mutex
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "reworm.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	e := &env{t: t, now: time.UnixMilli(1_700_000_000_000)}
	e.svc, err = New(Options{DB: db, Key: make([]byte, 32), PublicURL: "https://reworm.example", Now: e.clock})
	if err != nil {
		t.Fatal(err)
	}
	e.mux = http.NewServeMux()
	e.svc.Mount(e.mux)
	// as at boot: the setup token is minted before the first request
	if _, err := e.svc.SetupToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) clock() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.now
}

func (e *env) advance(d time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.now = e.now.Add(d)
}

// browser is one device's cookie jar.
type browser struct {
	e       *env
	ua      string
	ip      string
	cookies map[string]*http.Cookie
}

func (e *env) browser(ua string) *browser {
	return &browser{e: e, ua: ua, ip: "192.0.2.1:1234", cookies: map[string]*http.Cookie{}}
}

func (b *browser) do(method, path string, body any) *httptest.ResponseRecorder {
	b.e.t.Helper()
	var r *http.Request
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			b.e.t.Fatal(err)
		}
		r = httptest.NewRequest(method, path, bytes.NewReader(buf))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.RemoteAddr = b.ip
	r.Header.Set("User-Agent", b.ua)
	for _, c := range b.cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	b.e.mux.ServeHTTP(rec, r)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c
		}
	}
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var b struct{ Error, Message string }
	_ = json.NewDecoder(rec.Body).Decode(&b)
	return b.Error
}

// setup pairs b as the first device and returns its /api/me.
func (e *env) setup(b *browser) meJSON {
	e.t.Helper()
	token, err := e.svc.SetupToken(context.Background())
	if err != nil || token == "" {
		e.t.Fatalf("setup token = %q, %v", token, err)
	}
	rec := b.do(http.MethodPost, "/api/setup", setupRequest{Token: token, Name: " Jan ", Avatar: "vinyl"})
	if rec.Code != http.StatusCreated {
		e.t.Fatalf("setup = %d %s", rec.Code, rec.Body)
	}
	return decode[meJSON](e.t, rec)
}

func TestSetup(t *testing.T) {
	e := newEnv(t)
	b := e.browser("Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X)")
	if rec := b.do(http.MethodGet, "/api/me", nil); rec.Code != http.StatusUnauthorized || errCode(t, rec) != "setup_pending" {
		t.Fatalf("unpaired /api/me before setup = %d", rec.Code)
	}
	token, _ := e.svc.SetupToken(context.Background())
	again, _ := e.svc.SetupToken(context.Background())
	if token != again {
		t.Error("the setup token changed between calls")
	}
	if rec := b.do(http.MethodPost, "/api/setup", setupRequest{Token: "wrong", Name: "Jan", Avatar: "vinyl"}); rec.Code != http.StatusForbidden {
		t.Errorf("wrong token = %d", rec.Code)
	}
	if rec := b.do(http.MethodPost, "/api/setup", setupRequest{Token: token, Name: "", Avatar: "vinyl"}); rec.Code != http.StatusBadRequest {
		t.Errorf("no name = %d", rec.Code)
	}
	if rec := b.do(http.MethodPost, "/api/setup", setupRequest{Token: token, Name: "Jan", Avatar: "kazoo"}); rec.Code != http.StatusBadRequest {
		t.Errorf("bad avatar = %d", rec.Code)
	}
	me := e.setup(b)
	if me.Profile.Name != "Jan" || me.Profile.Avatar != "vinyl" || !me.Profile.Admin || me.Device.Label != "iPad" {
		t.Errorf("me = %+v", me)
	}
	if rec := b.do(http.MethodGet, "/api/me", nil); rec.Code != http.StatusOK || decode[meJSON](t, rec) != me {
		t.Errorf("/api/me after setup = %d", rec.Code)
	}
	if rec := b.do(http.MethodPost, "/api/setup", setupRequest{Token: token, Name: "Eve", Avatar: "mic"}); rec.Code != http.StatusGone {
		t.Errorf("second setup = %d, want 410", rec.Code)
	}
	if tok, _ := e.svc.SetupToken(context.Background()); tok != "" {
		t.Errorf("token after setup = %q", tok)
	}
	if rec := b.do(http.MethodPost, "/api/setup", "nope"); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed = %d", rec.Code)
	}
}

// Before the first profile a 401 says so (the UI points to the setup link), never with the token; after setup,
// and after a restart with profiles, it is the plain unauthorized again.
func TestRequire_setupPending(t *testing.T) {
	e := newEnv(t)
	token, _ := e.svc.SetupToken(context.Background())
	rec := e.browser("x").do(http.MethodGet, "/api/me", nil)
	if body := rec.Body.String(); strings.Contains(body, token) {
		t.Errorf("401 body leaks the setup token: %s", body)
	}
	if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "setup_pending" {
		t.Errorf("before setup = %d", rec.Code)
	}
	if rec := e.browser("x").do(http.MethodPost, "/api/pair", pairRequest{Kind: "device"}); errCode(t, rec) != "setup_pending" {
		t.Errorf("pair before setup = %d", rec.Code)
	}
	e.setup(e.browser("Mac"))
	if rec := e.browser("x").do(http.MethodGet, "/api/me", nil); rec.Code != http.StatusUnauthorized || errCode(t, rec) != "unauthorized" {
		t.Errorf("after setup = %d", rec.Code)
	}

	// a restart over the same database: the boot finds a profile, no token, no pending setup
	again, err := New(Options{DB: e.svc.db, Key: make([]byte, 32), Now: e.clock})
	if err != nil {
		t.Fatal(err)
	}
	if tok, err := again.SetupToken(context.Background()); tok != "" || err != nil {
		t.Fatalf("token after restart = %q, %v", tok, err)
	}
	mux := http.NewServeMux()
	again.Mount(mux)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/me", nil))
	if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "unauthorized" {
		t.Errorf("after restart = %d", rec.Code)
	}
}

// Unpaired requests read the pending state while a setup clears it (go test -race).
func TestRequire_setupPendingRace(t *testing.T) {
	e := newEnv(t)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 20 {
				if rec := e.browser("x").do(http.MethodGet, "/api/me", nil); rec.Code != http.StatusUnauthorized {
					t.Errorf("unpaired /api/me = %d", rec.Code)
				}
			}
		})
	}
	e.setup(e.browser("Mac"))
	wg.Wait()
}

func TestSetup_raceMakesOneAdmin(t *testing.T) {
	e := newEnv(t)
	token, _ := e.svc.SetupToken(context.Background())
	var wg sync.WaitGroup
	codes := make([]int, 4)
	for i := range codes {
		wg.Go(func() {
			b := e.browser("x")
			codes[i] = b.do(http.MethodPost, "/api/setup", setupRequest{Token: token, Name: "Jan", Avatar: "vinyl"}).Code
		})
	}
	wg.Wait()
	created := 0
	for _, c := range codes {
		if c == http.StatusCreated {
			created++
		}
	}
	if created != 1 {
		t.Errorf("codes = %v, want exactly one 201", codes)
	}
}

// create makes a new profile for b and returns its /api/me.
func (e *env) create(b *browser, name, avatar string) meJSON {
	e.t.Helper()
	rec := b.do(http.MethodPost, "/api/profiles", createRequest{Name: name, Avatar: avatar})
	if rec.Code != http.StatusCreated {
		e.t.Fatalf("create %s = %d %s", name, rec.Code, rec.Body)
	}
	return decode[meJSON](e.t, rec)
}

// Anyone can make a profile once the server is set up; before that only the setup link makes the first (admin).
func TestCreateProfile(t *testing.T) {
	e := newEnv(t)
	friend := e.browser("Mozilla/5.0 (Linux; Android 15)")
	if rec := friend.do(http.MethodPost, "/api/profiles", createRequest{Name: "Mia", Avatar: "mic"}); rec.Code != http.StatusUnauthorized || errCode(t, rec) != "setup_pending" {
		t.Fatalf("create before setup = %d", rec.Code)
	}
	if tok, _ := e.svc.SetupToken(context.Background()); tok == "" {
		t.Fatal("a refused create used up the setup")
	}
	me := e.setup(e.browser("Mac"))

	if rec := friend.do(http.MethodPost, "/api/profiles", createRequest{Name: " ", Avatar: "mic"}); rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_name" {
		t.Errorf("no name = %d", rec.Code)
	}
	if rec := friend.do(http.MethodPost, "/api/profiles", createRequest{Name: "Mia", Avatar: "kazoo"}); rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_avatar" {
		t.Errorf("bad avatar = %d", rec.Code)
	}
	if rec := friend.do(http.MethodPost, "/api/profiles", "nope"); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed = %d", rec.Code)
	}
	fm := e.create(friend, " Mia ", "mic")
	if fm.Profile.ID == me.Profile.ID || fm.Profile.Name != "Mia" || fm.Profile.Avatar != "mic" || fm.Profile.Admin || fm.Device.Label != "Android" {
		t.Errorf("created = %+v", fm)
	}
	if got := decode[meJSON](t, friend.do(http.MethodGet, "/api/me", nil)); got != fm {
		t.Errorf("/api/me after create = %+v", got)
	}

	// The profile's only device leaving it would strand it: no list, no switching leads back. Refused, unless the
	// caller says leave; then the stranded profile goes with its progress.
	if rec := friend.do(http.MethodPost, "/api/profiles", createRequest{Name: "Mia 2", Avatar: "worm"}); rec.Code != http.StatusConflict || errCode(t, rec) != "last_device" {
		t.Fatalf("create over the last device = %d", rec.Code)
	}
	if got := decode[meJSON](t, friend.do(http.MethodGet, "/api/me", nil)); got.Profile.ID != fm.Profile.ID {
		t.Fatalf("a refused create moved the device: %+v", got)
	}
	rec := friend.do(http.MethodPost, "/api/profiles", createRequest{Name: "Mia 2", Avatar: "worm", Leave: true})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create with leave = %d %s", rec.Code, rec.Body)
	}
	if again := decode[meJSON](t, rec); again.Profile.ID == fm.Profile.ID || again.Device.ID == fm.Device.ID {
		t.Errorf("second create = %+v", again)
	}
	if n := e.count(`SELECT count(*) FROM profiles WHERE id = ?`, fm.Profile.ID); n != 0 {
		t.Errorf("stranded profile rows = %d, want 0", n)
	}
	if n := e.count(`SELECT count(*) FROM devices WHERE id = ?`, fm.Device.ID); n != 0 {
		t.Errorf("old device rows = %d, want 0", n)
	}
}

func (e *env) count(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.svc.db.QueryRow(query, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// A profile with another device left keeps it: no question, nothing deleted.
func TestCreateProfile_otherDeviceStays(t *testing.T) {
	e := newEnv(t)
	e.setup(e.browser("Mac"))
	phone, tablet := e.browser("iPhone"), e.browser("iPad")
	fm := e.create(phone, "Mia", "mic")
	tablet.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: decode[pairJSON](t, phone.do(http.MethodPost, "/api/pair", pairRequest{Kind: "device"})).Code})
	e.create(tablet, "Lea", "note")
	if got := decode[meJSON](t, phone.do(http.MethodGet, "/api/me", nil)); got.Profile.ID != fm.Profile.ID {
		t.Errorf("Mia's phone = %+v", got)
	}
}

// Only a refused cookie counts as unpaired: a failed lookup must not skip removing the old device.
func TestCreateProfile_identifyError(t *testing.T) {
	e := newEnv(t)
	e.setup(e.browser("Mac"))
	b := e.browser("x")
	e.create(b, "Mia", "mic")
	_ = e.svc.db.Close()
	if rec := b.do(http.MethodPost, "/api/profiles", createRequest{Name: "Lea", Avatar: "note", Leave: true}); rec.Code != http.StatusInternalServerError {
		t.Errorf("create with the db down = %d", rec.Code)
	}
	if rec := b.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: "AAAAAA"}); rec.Code != http.StatusInternalServerError {
		t.Errorf("claim with the db down = %d", rec.Code)
	}
}

// Profiles are private: no list of them and no switching into another one without its device code.
func TestProfiles_noListNoSwitch(t *testing.T) {
	e := newEnv(t)
	host := e.browser("Mac")
	e.setup(host)
	if rec := host.do(http.MethodGet, "/api/profiles", nil); rec.Code == http.StatusOK {
		t.Errorf("GET /api/profiles = %d, want gone", rec.Code)
	}
	if rec := host.do(http.MethodPost, "/api/profiles/switch", map[string]string{"profileId": "x"}); rec.Code != http.StatusNotFound {
		t.Errorf("switch = %d, want 404", rec.Code)
	}
}

func TestCreateProfileIsRateLimited(t *testing.T) {
	e := newEnv(t)
	e.setup(e.browser("Mac"))
	// a party behind one router (one address, a browser each) makes its profiles at once; refusals cost nothing
	for i := range 10 {
		b := e.browser("x")
		if rec := b.do(http.MethodPost, "/api/profiles", createRequest{Name: "", Avatar: "mic"}); rec.Code != http.StatusBadRequest {
			t.Fatalf("bad name %d = %d", i, rec.Code)
		}
		if rec := b.do(http.MethodPost, "/api/profiles", createRequest{Name: "P", Avatar: "mic"}); rec.Code != http.StatusCreated {
			t.Fatalf("create %d = %d", i, rec.Code)
		}
	}
	b := e.browser("x")
	if rec := b.do(http.MethodPost, "/api/profiles", createRequest{Name: "P", Avatar: "mic"}); rec.Code != http.StatusTooManyRequests {
		t.Errorf("11th create = %d, want 429", rec.Code)
	}
	// the claim and pairing limit is its own
	if rec := b.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: "AAAAAA"}); rec.Code != http.StatusNotFound {
		t.Errorf("claim after the create limit = %d", rec.Code)
	}
	e.advance(time.Hour / 10)
	if rec := b.do(http.MethodPost, "/api/profiles", createRequest{Name: "P", Avatar: "mic"}); rec.Code != http.StatusCreated {
		t.Errorf("create after 6 min = %d", rec.Code)
	}
}

// Keys rotated by the thousand (IPv6 /64s) still meet the server-wide limit.
func TestCreateProfile_serverWideLimit(t *testing.T) {
	e := newEnv(t)
	e.setup(e.browser("Mac"))
	for i := range MaxCreatesPerHour + 1 {
		b := e.browser("x")
		b.ip = fmt.Sprintf("[2001:db8:0:%x::1]:1", i)
		want := http.StatusCreated
		if i == MaxCreatesPerHour {
			want = http.StatusTooManyRequests
		}
		if rec := b.do(http.MethodPost, "/api/profiles", createRequest{Name: "P", Avatar: "mic"}); rec.Code != want {
			t.Fatalf("create %d = %d, want %d", i, rec.Code, want)
		}
	}
	// the server refills one a 2 min; the refused caller gets it, its own budget untouched by the refusal
	e.advance(time.Hour / MaxCreatesPerHour)
	b := e.browser("x")
	b.ip = fmt.Sprintf("[2001:db8:0:%x::1]:1", MaxCreatesPerHour)
	if rec := b.do(http.MethodPost, "/api/profiles", createRequest{Name: "P", Avatar: "mic"}); rec.Code != http.StatusCreated {
		t.Fatalf("create after the refill = %d", rec.Code)
	}
}

func TestPairDevice(t *testing.T) {
	e := newEnv(t)
	host := e.browser("Macintosh")
	me := e.setup(host)

	if rec := e.browser("x").do(http.MethodPost, "/api/pair", pairRequest{Kind: "device"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("unpaired pair = %d", rec.Code)
	}
	if rec := host.do(http.MethodPost, "/api/pair", pairRequest{Kind: "car"}); rec.Code != http.StatusBadRequest {
		t.Errorf("bad kind = %d", rec.Code)
	}
	// invites are gone: anyone makes their own profile
	if rec := host.do(http.MethodPost, "/api/pair", pairRequest{Kind: "invite"}); rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_kind" {
		t.Errorf("invite = %d", rec.Code)
	}
	rec := host.do(http.MethodPost, "/api/pair", pairRequest{Kind: "device"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("pair = %d %s", rec.Code, rec.Body)
	}
	p := decode[pairJSON](t, rec)
	if len(p.Code) != 6 || p.URL != "https://reworm.example/pair/"+p.Code || p.ExpiresAt != e.clock().Add(PairTTL).UnixMilli() {
		t.Errorf("pair = %+v", p)
	}

	phone := e.browser("iPhone")
	rec = phone.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: strings.ToLower(p.Code)})
	if rec.Code != http.StatusCreated {
		t.Fatalf("claim = %d %s", rec.Code, rec.Body)
	}
	pm := decode[meJSON](t, rec)
	if pm.Profile.ID != me.Profile.ID || pm.Device.Label != "iPhone" || pm.Device.ID == me.Device.ID {
		t.Errorf("claimed device = %+v", pm)
	}
	if rec := e.browser("x").do(http.MethodPost, "/api/pair/claim", claimRequest{Code: p.Code}); rec.Code != http.StatusNotFound {
		t.Errorf("reused code = %d, want 404", rec.Code)
	}

	// Expired.
	late := decode[pairJSON](t, host.do(http.MethodPost, "/api/pair", pairRequest{Kind: "device"}))
	e.advance(PairTTL)
	if rec := e.browser("x").do(http.MethodPost, "/api/pair/claim", claimRequest{Code: late.Code}); rec.Code != http.StatusNotFound {
		t.Errorf("expired code = %d", rec.Code)
	}
}

// An invite code made before invites went away claims nothing: it is not a device code.
func TestClaim_leftoverInvite(t *testing.T) {
	e := newEnv(t)
	me := e.setup(e.browser("Mac"))
	if _, err := e.svc.db.Exec(`INSERT INTO pairing_codes (code, kind, profile_id, expires_at) VALUES ('INV123', 'invite', ?, ?)`,
		me.Profile.ID, e.clock().Add(PairTTL).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if rec := e.browser("x").do(http.MethodPost, "/api/pair/claim", claimRequest{Code: "INV123"}); rec.Code != http.StatusNotFound {
		t.Errorf("leftover invite = %d, want 404", rec.Code)
	}
}

func TestClaimReplacesThePreviousDevice(t *testing.T) {
	e := newEnv(t)
	host := e.browser("Mac")
	me := e.setup(host)
	friend := e.browser("Android")
	fm := e.create(friend, "Mia", "mic")
	mias := decode[pairJSON](t, friend.do(http.MethodPost, "/api/pair", pairRequest{Kind: "device"}))
	// The admin's only device cannot be claimed over: the server would lose its admin.
	if rec := host.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: mias.Code}); rec.Code != http.StatusConflict || errCode(t, rec) != "last_admin" {
		t.Fatalf("claim over the last admin device = %d", rec.Code)
	}
	// Mia's only device takes Jan's code: refused (it would strand Mia), and the code stays usable; with leave her
	// device row and her profile go, the new device is Jan's.
	jans := decode[pairJSON](t, host.do(http.MethodPost, "/api/pair", pairRequest{Kind: "device"}))
	if rec := friend.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: jans.Code}); rec.Code != http.StatusConflict || errCode(t, rec) != "last_device" {
		t.Fatalf("claim over Mia's last device = %d", rec.Code)
	}
	rec := friend.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: jans.Code, Leave: true})
	if rec.Code != http.StatusCreated || decode[meJSON](t, rec).Profile.ID != me.Profile.ID {
		t.Fatal(rec.Code)
	}
	if n := e.count(`SELECT count(*) FROM devices WHERE id = ?`, fm.Device.ID); n != 0 {
		t.Errorf("Mia's old device rows = %d; want 0", n)
	}
	if n := e.count(`SELECT count(*) FROM profiles WHERE id = ?`, fm.Profile.ID); n != 0 {
		t.Errorf("Mia's stranded profile rows = %d; want 0", n)
	}
	// a device already on the code's profile just gets a fresh device row, no question
	again := decode[pairJSON](t, host.do(http.MethodPost, "/api/pair", pairRequest{Kind: "device"}))
	if rec := friend.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: again.Code}); rec.Code != http.StatusCreated {
		t.Errorf("claim of its own profile's code = %d", rec.Code)
	}
}

func TestPairURL_neverFromTheHostHeader(t *testing.T) {
	e := newEnv(t)
	e.svc.publicURL = ""
	host := e.browser("Mac")
	e.setup(host)
	p := decode[pairJSON](t, host.do(http.MethodPost, "/api/pair", pairRequest{Kind: "device"}))
	if p.URL != "/pair/"+p.Code {
		t.Errorf("url = %q, want the path only", p.URL)
	}
}

func TestLastAdminDeviceStays(t *testing.T) {
	e := newEnv(t)
	host := e.browser("Mac")
	me := e.setup(host)
	if rec := host.do(http.MethodPost, "/api/profiles", createRequest{Name: "Mia", Avatar: "mic"}); rec.Code != http.StatusConflict || errCode(t, rec) != "last_admin" {
		t.Errorf("new profile on the last admin device = %d", rec.Code)
	}
	if rec := host.do(http.MethodDelete, "/api/devices/"+me.Device.ID, nil); rec.Code != http.StatusConflict {
		t.Errorf("delete the last admin device = %d", rec.Code)
	}
	// Once a second admin device exists, both are allowed.
	phone := e.browser("iPhone")
	pm := decode[meJSON](t, phone.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: decode[pairJSON](t, host.do(http.MethodPost, "/api/pair", pairRequest{Kind: "device"})).Code}))
	if mia := e.create(host, "Mia", "mic"); mia.Profile.Admin {
		t.Errorf("new profile with another admin device = %+v", mia)
	}
	if rec := phone.do(http.MethodDelete, "/api/devices/"+pm.Device.ID, nil); rec.Code != http.StatusConflict {
		t.Errorf("the phone is now the last admin device: delete = %d", rec.Code)
	}
}

func TestDevicesAndRevoke(t *testing.T) {
	e := newEnv(t)
	host := e.browser("Mac")
	me := e.setup(host)
	phone := e.browser("iPhone")
	code := decode[pairJSON](t, host.do(http.MethodPost, "/api/pair", pairRequest{Kind: "device"})).Code
	pm := decode[meJSON](t, phone.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: code}))

	e.advance(10 * time.Minute)
	list := decode[[]deviceListJSON](t, host.do(http.MethodGet, "/api/devices", nil))
	if len(list) != 2 || list[0].ID != me.Device.ID || !list[0].Current || list[0].LastSeen != e.clock().UnixMilli() || list[1].Current {
		t.Errorf("devices = %+v", list)
	}

	// Revoked: the phone's cookie is still validly signed, the row is gone.
	if rec := host.do(http.MethodDelete, "/api/devices/"+pm.Device.ID, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", rec.Code)
	}
	if rec := phone.do(http.MethodGet, "/api/me", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked device /api/me = %d, want 401", rec.Code)
	}
	if rec := host.do(http.MethodDelete, "/api/devices/"+pm.Device.ID, nil); rec.Code != http.StatusNotFound {
		t.Errorf("delete twice = %d", rec.Code)
	}

	// Another profile cannot remove the admin's devices; removing your own current device signs you out.
	friend := e.browser("Android")
	fm := e.create(friend, "Mia", "mic")
	if rec := friend.do(http.MethodDelete, "/api/devices/"+me.Device.ID, nil); rec.Code != http.StatusNotFound {
		t.Errorf("foreign delete = %d", rec.Code)
	}
	// a profile's last device stays, for anyone: removing it would strand the profile (no list, no switching)
	for _, who := range []*browser{friend, host} {
		if rec := who.do(http.MethodDelete, "/api/devices/"+fm.Device.ID, nil); rec.Code != http.StatusConflict || errCode(t, rec) != "last_device" {
			t.Errorf("delete Mia's last device = %d", rec.Code)
		}
	}
	tablet := e.browser("iPad")
	tablet.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: decode[pairJSON](t, friend.do(http.MethodPost, "/api/pair", pairRequest{Kind: "device"})).Code})
	if rec := friend.do(http.MethodDelete, "/api/devices/"+fm.Device.ID, nil); rec.Code != http.StatusNoContent {
		t.Errorf("own delete = %d", rec.Code)
	}
	if _, ok := friend.cookies["reworm_device"]; ok {
		t.Error("cookie kept after removing the current device")
	}
}

func TestUpdateMe(t *testing.T) {
	e := newEnv(t)
	b := e.browser("Mac")
	e.setup(b)
	name := "Jan B."
	rec := b.do(http.MethodPatch, "/api/me", updateMeRequest{Name: &name})
	if got := decode[meJSON](t, rec); rec.Code != http.StatusOK || got.Profile.Name != "Jan B." || got.Profile.Avatar != "vinyl" {
		t.Errorf("patch name = %d %+v", rec.Code, got)
	}
	av := "jukebox"
	if got := decode[meJSON](t, b.do(http.MethodPatch, "/api/me", updateMeRequest{Avatar: &av})); got.Profile.Avatar != "jukebox" || got.Profile.Name != "Jan B." {
		t.Errorf("patch avatar = %+v", got)
	}
	bad := "trumpet"
	if rec := b.do(http.MethodPatch, "/api/me", updateMeRequest{Avatar: &bad}); rec.Code != http.StatusBadRequest {
		t.Errorf("bad avatar = %d", rec.Code)
	}
	long := strings.Repeat("x", MaxName+1)
	if rec := b.do(http.MethodPatch, "/api/me", updateMeRequest{Name: &long}); rec.Code != http.StatusBadRequest {
		t.Errorf("long name = %d", rec.Code)
	}
	ctrl := "a\x07"
	if rec := b.do(http.MethodPatch, "/api/me", updateMeRequest{Name: &ctrl}); rec.Code != http.StatusBadRequest {
		t.Errorf("control char = %d", rec.Code)
	}
}

func TestClaimIsRateLimited(t *testing.T) {
	e := newEnv(t)
	b := e.browser("x")
	var last int
	for range 11 {
		last = b.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: "AAAAAA"}).Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("11th guess = %d, want 429", last)
	}
	// Another address is not affected.
	other := e.browser("x")
	other.ip = "198.51.100.9:1"
	if rec := other.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: "AAAAAA"}); rec.Code != http.StatusNotFound {
		t.Errorf("other caller = %d", rec.Code)
	}
}

func TestLoadKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.key")
	k, err := LoadKey(path)
	if err != nil || len(k) != 32 {
		t.Fatalf("new key = %d bytes, %v", len(k), err)
	}
	again, err := LoadKey(path)
	if err != nil || !bytes.Equal(k, again) {
		t.Error("key changed across loads")
	}
	if err := os.WriteFile(path, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadKey(path); err == nil {
		t.Error("short key accepted")
	}
	if _, err := LoadKey(filepath.Join(path, "x")); err == nil {
		t.Error("key under a file accepted")
	}
}

func TestNew_badKey(t *testing.T) {
	// DeriveKey always yields 32 bytes, so only the name could fail; the zero-length key still derives.
	if _, err := New(Options{Key: nil}); err != nil {
		t.Errorf("New = %v", err)
	}
}

func TestLabel(t *testing.T) {
	for ua, want := range map[string]string{
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0)": "iPhone", "Mozilla/5.0 (Linux; Android 15)": "Android",
		"Mozilla/5.0 (Windows NT 10.0)": "Windows", "curl/8": "Browser", "Mozilla/5.0 (X11; CrOS x86_64)": "Chromebook",
	} {
		if got := Label(ua); got != want {
			t.Errorf("Label(%q) = %q, want %q", ua, got, want)
		}
	}
}

func TestIdentify_dbError(t *testing.T) {
	e := newEnv(t)
	b := e.browser("Mac")
	e.setup(b)
	_ = e.svc.db.Close()
	if rec := b.do(http.MethodGet, "/api/me", nil); rec.Code != http.StatusInternalServerError {
		t.Errorf("db down = %d", rec.Code)
	}
	if _, err := e.svc.SetupToken(context.Background()); err == nil {
		t.Error("SetupToken on a closed db succeeded")
	}
}

// Under a path prefix the device cookie is scoped to it: another app on the host never receives it.
func TestNew_cookiePath(t *testing.T) {
	for base, want := range map[string]string{"": "/", "/reworm": "/reworm/"} {
		s, err := New(Options{Key: make([]byte, 32), BasePath: base})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		if err := s.sessions.Issue(rec, auth.Claims{Subject: "d"}); err != nil {
			t.Fatal(err)
		}
		if c := rec.Result().Cookies(); len(c) != 1 || c[0].Path != want {
			t.Errorf("base %q: cookies = %v, want path %s", base, c, want)
		}
	}
}
