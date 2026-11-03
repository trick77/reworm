package account

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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
	if rec := b.do(http.MethodGet, "/api/me", nil); rec.Code != http.StatusUnauthorized || errCode(t, rec) != "unauthorized" {
		t.Fatalf("unpaired /api/me = %d", rec.Code)
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

func TestPairDeviceAndInvite(t *testing.T) {
	e := newEnv(t)
	host := e.browser("Macintosh")
	me := e.setup(host)

	if rec := e.browser("x").do(http.MethodPost, "/api/pair", pairRequest{Kind: "device"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("unpaired pair = %d", rec.Code)
	}
	if rec := host.do(http.MethodPost, "/api/pair", pairRequest{Kind: "car"}); rec.Code != http.StatusBadRequest {
		t.Errorf("bad kind = %d", rec.Code)
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

	// Invite: a new profile; name and avatar required, a bad one leaves the code usable.
	inv := decode[pairJSON](t, host.do(http.MethodPost, "/api/pair", pairRequest{Kind: "invite"}))
	friend := e.browser("Android")
	if rec := friend.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: inv.Code}); rec.Code != http.StatusBadRequest {
		t.Errorf("invite without a name = %d", rec.Code)
	}
	if rec := friend.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: inv.Code, Name: "Mia", Avatar: "nope"}); rec.Code != http.StatusBadRequest {
		t.Errorf("invite with a bad avatar = %d", rec.Code)
	}
	rec = friend.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: inv.Code, Name: "Mia", Avatar: "mic"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite claim = %d %s", rec.Code, rec.Body)
	}
	fm := decode[meJSON](t, rec)
	if fm.Profile.ID == me.Profile.ID || fm.Profile.Name != "Mia" || fm.Profile.Admin {
		t.Errorf("invited profile = %+v", fm)
	}

	// Expired.
	late := decode[pairJSON](t, host.do(http.MethodPost, "/api/pair", pairRequest{Kind: "device"}))
	e.advance(PairTTL)
	if rec := e.browser("x").do(http.MethodPost, "/api/pair/claim", claimRequest{Code: late.Code}); rec.Code != http.StatusNotFound {
		t.Errorf("expired code = %d", rec.Code)
	}

	// Who's playing: both profiles; the phone switches to Mia's.
	list := decode[[]profileListJSON](t, phone.do(http.MethodGet, "/api/profiles", nil))
	if len(list) != 2 || list[0].Name != "Jan" || list[1].Name != "Mia" {
		t.Errorf("profiles = %+v", list)
	}
	rec = phone.do(http.MethodPost, "/api/profiles/switch", switchRequest{ProfileID: fm.Profile.ID})
	if rec.Code != http.StatusOK || decode[meJSON](t, rec).Profile.ID != fm.Profile.ID {
		t.Errorf("switch = %d", rec.Code)
	}
	if rec := phone.do(http.MethodPost, "/api/profiles/switch", switchRequest{ProfileID: "nope"}); rec.Code != http.StatusNotFound {
		t.Errorf("switch to unknown = %d", rec.Code)
	}
	if got := decode[meJSON](t, phone.do(http.MethodGet, "/api/me", nil)); got.Profile.Name != "Mia" || got.Device.ID != pm.Device.ID {
		t.Errorf("after switch = %+v", got)
	}
}

func TestClaimReplacesThePreviousDevice(t *testing.T) {
	e := newEnv(t)
	host := e.browser("Mac")
	e.setup(host)
	inv := decode[pairJSON](t, host.do(http.MethodPost, "/api/pair", pairRequest{Kind: "invite"}))
	// The host's own browser claims an invite: its old device row goes, the new one is Mia's.
	rec := host.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: inv.Code, Name: "Mia", Avatar: "mic"})
	if rec.Code != http.StatusCreated {
		t.Fatal(rec.Code)
	}
	var n int
	if err := e.svc.db.QueryRow(`SELECT count(*) FROM devices`).Scan(&n); err != nil || n != 1 {
		t.Errorf("devices = %d, %v; want 1", n, err)
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
	inv := decode[pairJSON](t, host.do(http.MethodPost, "/api/pair", pairRequest{Kind: "invite"}))
	friend := e.browser("Android")
	fm := decode[meJSON](t, friend.do(http.MethodPost, "/api/pair/claim", claimRequest{Code: inv.Code, Name: "Mia", Avatar: "mic"}))
	if rec := friend.do(http.MethodDelete, "/api/devices/"+me.Device.ID, nil); rec.Code != http.StatusNotFound {
		t.Errorf("foreign delete = %d", rec.Code)
	}
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
