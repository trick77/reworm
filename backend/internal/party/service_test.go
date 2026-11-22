package party

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/trick77/ship/kit/auth"

	"github.com/trick77/reworm/internal/account"
	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/preview"
	"github.com/trick77/reworm/internal/store"
)

type env struct {
	t    *testing.T
	svc  *Service
	acc  *account.Service
	repo *deck.Repo
	db   *sql.DB
	mux  *http.ServeMux
	mu   sync.Mutex
	now  time.Time
}

func newEnv(t *testing.T, previews preview.Source) *env {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "reworm.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for i, y := range []int{1976, 1975, 1995, 1984} {
		if _, err := db.ExecContext(ctx, `INSERT INTO songs (id, artist, title, year, year_source, sets) VALUES (?, ?, ?, ?, 'card', '["essentials"]')`,
			string(rune('a'+i)), "Artist", "Title", y); err != nil {
			t.Fatal(err)
		}
	}
	e := &env{t: t, now: t0, db: db}
	key := make([]byte, 32)
	e.acc, err = account.New(account.Options{DB: db, Key: key, Now: e.clock})
	if err != nil {
		t.Fatal(err)
	}
	e.repo = deck.New(db)
	e.svc, err = New(Options{Accounts: e.acc, Songs: e.repo, Previews: previews, Key: key, Now: e.clock,
		Rand: func() *rand.Rand { return rand.New(rand.NewPCG(7, 8)) }})
	if err != nil {
		t.Fatal(err)
	}
	if e.svc.countdown != Countdown {
		t.Fatalf("service countdown = %v, want %v", e.svc.countdown, Countdown)
	}
	e.svc.countdown = 0 // the HTTP tests answer at once; TestCountdown covers it
	e.mux = http.NewServeMux()
	e.acc.Mount(e.mux)
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
	e.now = e.now.Add(d)
	e.mu.Unlock()
}

type browser struct {
	e       *env
	cookies map[string]*http.Cookie
}

func (e *env) browser() *browser { return &browser{e: e, cookies: map[string]*http.Cookie{}} }

func (b *browser) do(method, path string, body any) *httptest.ResponseRecorder {
	b.e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	r.RemoteAddr = "192.0.2.1:1"
	for _, c := range b.cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	b.e.mux.ServeHTTP(rec, r)
	for _, c := range rec.Result().Cookies() {
		b.cookies[c.Name] = c
	}
	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

// pairedHost is a browser paired as the server's first profile.
func (e *env) pairedHost() *browser {
	e.t.Helper()
	b := e.browser()
	token, _ := e.acc.SetupToken(context.Background())
	if rec := b.do(http.MethodPost, "/api/setup", map[string]string{"token": token, "name": "Jan", "avatar": "vinyl"}); rec.Code != http.StatusCreated {
		e.t.Fatalf("setup = %d", rec.Code)
	}
	return b
}

func (e *env) createRoom(h *browser, body any) string {
	e.t.Helper()
	rec := h.do(http.MethodPost, "/api/rooms", body)
	if rec.Code != http.StatusCreated {
		e.t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
	return decodeBody[createResponse](e.t, rec).Code
}

func TestHTTP_yearsGame(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	h := e.pairedHost()
	if rec := h.do(http.MethodPost, "/api/rooms", Config{Mode: "quiz"}); rec.Code != http.StatusBadRequest {
		t.Errorf("bad config = %d", rec.Code)
	}
	code := e.createRoom(h, Config{Mode: ModeYears, ToWin: 2, Deck: deck.Selector{Sets: []string{"essentials"}}})
	base := "/api/rooms/" + strings.ToLower(code)

	guest := e.browser()
	if rec := guest.do(http.MethodPost, base+"/join", joinRequest{Name: "Mia", Avatar: "kazoo"}); rec.Code != http.StatusBadRequest {
		t.Errorf("bad avatar join = %d", rec.Code)
	}
	rec := guest.do(http.MethodPost, base+"/join", joinRequest{Name: "Mia", Avatar: "mic"})
	if rec.Code != http.StatusOK {
		t.Fatalf("join = %d %s", rec.Code, rec.Body)
	}
	mia := decodeBody[joinResponse](t, rec).PlayerID
	if again := decodeBody[joinResponse](t, guest.do(http.MethodPost, base+"/join", joinRequest{Name: "Mia", Avatar: "mic"})).PlayerID; again != mia {
		t.Errorf("rejoin with the cookie = %q, want %q", again, mia)
	}
	// The host's own phone joins as the paired profile, no name needed.
	jan := decodeBody[joinResponse](t, h.do(http.MethodPost, base+"/join", nil)).PlayerID

	if rec := guest.do(http.MethodPost, base+"/start", nil); rec.Code != http.StatusForbidden {
		t.Errorf("guest start = %d, want 403 not_host", rec.Code)
	}
	rec = h.do(http.MethodPost, base+"/start", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("start = %d %s", rec.Code, rec.Body)
	}
	s := decodeBody[State](t, rec)
	if s.Phase != PhasePlaying || len(s.Players) != 2 || s.Round.ProposedYear == nil {
		t.Fatalf("started = %+v", s)
	}
	if strings.Contains(rec.Body.String(), "hostId") {
		t.Errorf("state names the host: %s", rec.Body)
	}
	room, _ := e.svc.room(code)
	if room.HistoryProfile() == "" {
		t.Fatal("a paired host's room keeps no history")
	}
	// Start's background prefetch publishes the state (reads the round): wait for it before writing.
	room.bg.Wait()
	room.round.proposed = room.round.pick.song.Year + 4 // one point, not the game
	choice := right(room)
	if rec := guest.do(http.MethodPost, base+"/answer", pressRequest{Round: 1, StartedAt: 1, Choice: choice}); rec.Code != http.StatusConflict {
		t.Errorf("answer for another game's round 1 = %d, want 409 wrong_round", rec.Code)
	}
	if rec := guest.do(http.MethodPost, base+"/answer", pressRequest{Round: 1, Choice: choice}); rec.Code != http.StatusOK ||
		!decodeBody[acceptedResponse](t, rec).Accepted {
		t.Fatalf("answer = %d %s", rec.Code, rec.Body)
	}
	if rec := e.browser().do(http.MethodPost, base+"/answer", pressRequest{Round: 1, Choice: choice}); rec.Code != http.StatusUnauthorized {
		t.Errorf("stranger answer = %d, want 401 not_in_room", rec.Code)
	}
	if got := decodeBody[meResponse](t, guest.do(http.MethodGet, base+"/me", nil)); got.PlayerID != mia {
		t.Errorf("guest /me = %+v", got)
	}
	if got := decodeBody[meResponse](t, h.do(http.MethodGet, base+"/me", nil)); got.PlayerID != jan {
		t.Errorf("profile /me = %+v", got)
	}
	if rec := e.browser().do(http.MethodGet, base+"/me", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("stranger /me = %d", rec.Code)
	}
	s = decodeBody[State](t, guest.do(http.MethodGet, base, nil))
	if s.Round.Result == nil || *s.Round.Result.WinnerID != mia {
		t.Errorf("result = %+v", s.Round.Result)
	}
	if rec := h.do(http.MethodPost, base+"/next", nil); rec.Code != http.StatusOK {
		t.Errorf("next = %d", rec.Code)
	}
	if rec := h.do(http.MethodPost, base+"/answer", pressRequest{Round: 2, Choice: wrong(room)}); rec.Code != http.StatusOK {
		t.Errorf("profile player answer = %d", rec.Code)
	}
	if s := room.State(e.clock()); playerOf(s, jan).State != StateOut {
		t.Errorf("jan = %+v", playerOf(s, jan))
	}
	for _, path := range []string{"/pause", "/resume"} {
		if rec := h.do(http.MethodPost, base+path, nil); rec.Code != http.StatusOK {
			t.Errorf("%s = %d %s", path, rec.Code, rec.Body)
		}
	}
	if rec := h.do(http.MethodPost, base+"/kick", kickRequest{PlayerID: jan}); rec.Code != http.StatusOK {
		t.Errorf("kick = %d", rec.Code)
	}
	if rec := h.do(http.MethodPost, base+"/kick", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("kick without a body = %d", rec.Code)
	}
	if rec := h.do(http.MethodPost, base+"/settings", Config{}); rec.Code != http.StatusConflict {
		t.Errorf("settings mid-game = %d", rec.Code)
	}
	if rec := h.do(http.MethodPost, base+"/settings", Config{Mode: "x"}); rec.Code != http.StatusBadRequest {
		t.Errorf("bad settings = %d", rec.Code)
	}
	if rec := h.do(http.MethodPost, base+"/rematch", nil); rec.Code != http.StatusConflict {
		t.Errorf("rematch mid-game = %d", rec.Code)
	}

	played, err := e.repo.Played(context.Background(), room.HistoryProfile(), t0.Add(-time.Hour))
	if err != nil || len(played) != 2 {
		t.Errorf("played history = %v, %v; want the two rounds", played, err)
	}

	if rec := guest.do(http.MethodPost, base+"/abort", nil); rec.Code != http.StatusForbidden {
		t.Errorf("guest abort = %d, want 403 not_host", rec.Code)
	}
	rec = h.do(http.MethodPost, base+"/abort", nil)
	if s := decodeBody[State](t, rec); rec.Code != http.StatusOK || s.Phase != PhaseLobby || s.Round != nil {
		t.Errorf("abort = %d %+v", rec.Code, s)
	}
	if rec := h.do(http.MethodPost, base+"/abort", nil); rec.Code != http.StatusConflict {
		t.Errorf("abort in the lobby = %d, want 409", rec.Code)
	}
}

// Anyone hosts: a browser without a profile opens a room, its room cookie makes it the host of that room only, and
// no history is kept for it.
func TestHTTP_hostWithoutProfile(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	h := e.browser()
	code := e.createRoom(h, Config{Mode: ModeYears, ToWin: 10, Deck: deck.Selector{Sets: []string{"essentials"}}})
	base := "/api/rooms/" + code
	if h.cookies["reworm_host"] == nil {
		t.Fatalf("no host cookie: %v", h.cookies)
	}
	room, _ := e.svc.room(code)
	if room.HistoryProfile() != "" {
		t.Errorf("history profile = %q, want none", room.HistoryProfile())
	}
	guest := e.browser()
	if rec := guest.do(http.MethodPost, base+"/join", joinRequest{Name: "Mia", Avatar: "mic"}); rec.Code != http.StatusOK {
		t.Fatalf("join = %d %s", rec.Code, rec.Body)
	}
	if rec := guest.do(http.MethodPost, base+"/start", nil); rec.Code != http.StatusForbidden {
		t.Errorf("guest start = %d, want 403", rec.Code)
	}
	// The server, not the UI's storage, says who hosts; asking renews the host's cookie.
	rec := h.do(http.MethodGet, base+"/host", nil)
	if !decodeBody[hostResponse](t, rec).Host || !strings.Contains(rec.Header().Get("Set-Cookie"), "reworm_host=") {
		t.Errorf("host asks = %s %v", rec.Body, rec.Header())
	}
	if decodeBody[hostResponse](t, guest.do(http.MethodGet, base+"/host", nil)).Host {
		t.Error("a guest is told it hosts")
	}
	// Another host's cookie is for its own room only.
	other := e.browser()
	e.createRoom(other, Config{})
	if rec := other.do(http.MethodPost, base+"/start", nil); rec.Code != http.StatusForbidden {
		t.Errorf("other host start = %d, want 403", rec.Code)
	}
	if decodeBody[hostResponse](t, other.do(http.MethodGet, base+"/host", nil)).Host {
		t.Error("another room's host is told it hosts this one")
	}
	if rec := h.do(http.MethodPost, base+"/start", nil); rec.Code != http.StatusOK {
		t.Fatalf("start = %d %s", rec.Code, rec.Body)
	}
	room.bg.Wait()
	var n int
	if err := e.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM played_history`).Scan(&n); err != nil || n != 0 {
		t.Errorf("played history rows = %d, %v; want none", n, err)
	}
	// The same browser hosting again keeps its host id: the earlier room ends.
	again := e.createRoom(h, Config{})
	if _, err := e.svc.room(code); err == nil {
		t.Error("the host's earlier room is still open")
	}
	if rec := h.do(http.MethodPost, "/api/rooms/"+again+"/pause", nil); rec.Code != http.StatusConflict {
		t.Errorf("host pause in the lobby = %d, want 409 (host, wrong phase)", rec.Code)
	}
}

// A paired device whose lookup fails gets an error, not a room without its history.
func TestHTTP_createLookupFails(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	h := e.pairedHost()
	_ = e.db.Close()
	if rec := h.do(http.MethodPost, "/api/rooms", Config{}); rec.Code != http.StatusInternalServerError {
		t.Errorf("create = %d %s, want 500", rec.Code, rec.Body)
	}
	e.svc.mu.Lock()
	n := len(e.svc.rooms)
	e.svc.mu.Unlock()
	if n != 0 {
		t.Errorf("rooms = %d, want none", n)
	}
}

func TestHTTP_countdown(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	e.svc.countdown = Countdown
	h := e.pairedHost()
	code := e.createRoom(h, Config{Mode: ModeYears, Deck: deck.Selector{Sets: []string{"essentials"}}})
	base := "/api/rooms/" + strings.ToLower(code)
	guest := e.browser()
	if rec := guest.do(http.MethodPost, base+"/join", joinRequest{Name: "Mia", Avatar: "mic"}); rec.Code != http.StatusOK {
		t.Fatalf("join = %d %s", rec.Code, rec.Body)
	}
	rec := h.do(http.MethodPost, base+"/start", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("start = %d %s", rec.Code, rec.Body)
	}
	s := decodeBody[State](t, rec)
	if s.Phase != PhaseCountdown || s.Round.StartedAt != s.Now+Countdown.Milliseconds() {
		t.Fatalf("started = %s, startedAt %d, now %d", s.Phase, s.Round.StartedAt, s.Now)
	}
	room, _ := e.svc.room(code)
	room.bg.Wait()
	choice := right(room)
	if rec := guest.do(http.MethodPost, base+"/answer", pressRequest{Round: 1, Choice: choice}); rec.Code != http.StatusConflict ||
		!strings.Contains(rec.Body.String(), "wrong_phase") {
		t.Errorf("answer in the countdown = %d %s, want 409 wrong_phase", rec.Code, rec.Body)
	}
	// At startedAt the press itself starts the round (no tick needed) and counts.
	e.advance(Countdown)
	if rec := guest.do(http.MethodPost, base+"/answer", pressRequest{Round: 1, Choice: choice}); rec.Code != http.StatusOK {
		t.Errorf("answer at the start = %d %s", rec.Code, rec.Body)
	}
}

func TestHTTP_buzzer(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	h := e.pairedHost()
	code := e.createRoom(h, Config{Mode: ModeBuzzer})
	base := "/api/rooms/" + code
	a, b := e.browser(), e.browser()
	pa := decodeBody[joinResponse](t, a.do(http.MethodPost, base+"/join", joinRequest{Name: "A", Avatar: "mic"})).PlayerID
	b.do(http.MethodPost, base+"/join", joinRequest{Name: "B", Avatar: "drum"})
	h.do(http.MethodPost, base+"/start", nil)
	for _, step := range []struct {
		who  *browser
		path string
		body pressRequest
	}{
		{a, "/buzz", pressRequest{Round: 1}},
		{a, "/reveal", pressRequest{Round: 1}},
		{b, "/reveal", pressRequest{Round: 1}},
		{a, "/judge", pressRequest{Round: 1, Points: new(2)}},
		{b, "/judge", pressRequest{Round: 1, Points: new(2)}},
	} {
		if rec := step.who.do(http.MethodPost, base+step.path, step.body); rec.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", step.path, rec.Code, rec.Body)
		}
	}
	s := decodeBody[State](t, a.do(http.MethodGet, base, nil))
	if s.Phase != PhaseResult || *s.Round.Result.WinnerID != pa || s.Round.Result.Points != 2 {
		t.Errorf("result = %s %+v", s.Phase, s.Round.Result)
	}
	if rec := a.do(http.MethodPost, base+"/judge", pressRequest{Round: 1}); rec.Code != http.StatusBadRequest {
		t.Errorf("judge without points = %d", rec.Code)
	}
}

func TestHTTP_refusals(t *testing.T) {
	e := newEnv(t, nil)
	h := e.pairedHost()
	if rec := h.do(http.MethodGet, "/api/rooms/ZZZZ", nil); rec.Code != http.StatusNotFound {
		t.Errorf("unknown room = %d", rec.Code)
	}
	code := e.createRoom(h, Config{})
	base := "/api/rooms/" + code
	h.do(http.MethodPost, base+"/join", nil)
	if rec := h.do(http.MethodPost, base+"/start", nil); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("start without previews = %d, want 503", rec.Code)
	}
	// Another paired profile is not the host.
	other := e.browser()
	if rec := other.do(http.MethodPost, "/api/profiles", map[string]string{"name": "Mia", "avatar": "mic"}); rec.Code != http.StatusCreated {
		t.Fatalf("create profile = %d", rec.Code)
	}
	if rec := other.do(http.MethodPost, base+"/pause", nil); rec.Code != http.StatusForbidden {
		t.Errorf("non-host pause = %d", rec.Code)
	}
	if rec := h.do(http.MethodPost, base+"/answer", "x"); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed answer = %d", rec.Code)
	}
}

func TestTick_expiresIdleRooms(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	h := e.pairedHost()
	code := e.createRoom(h, Config{})
	e.advance(IdleTTL - time.Second)
	e.svc.Tick(e.clock())
	if _, err := e.svc.room(code); err != nil {
		t.Fatal("expired early")
	}
	e.advance(time.Second)
	e.svc.Tick(e.clock())
	if rec := h.do(http.MethodGet, "/api/rooms/"+code, nil); rec.Code != http.StatusNotFound {
		t.Errorf("idle room = %d, want 404", rec.Code)
	}
}

func TestRun_stopsWithTheContext(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.svc.Run(ctx); close(done) }()
	time.Sleep(3 * tickEvery)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
	e.svc.Close()
}

func TestMaxRooms(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	for i := range MaxRooms {
		if _, err := e.svc.create(fmt.Sprint("p", i), "", Config{}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := e.svc.create("p-last", "", Config{})
	wantProblem(t, err, "too_many_rooms")
}

// One room per host: a new room ends the host's earlier one, whatever its phase; other hosts' rooms stay.
func TestCreate_oneRoomPerHost(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	h := e.pairedHost()
	code := e.createRoom(h, Config{})
	other, err := e.svc.create("someone-else", "", Config{})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		e.createRoom(h, Config{})
	}
	if rec := h.do(http.MethodGet, "/api/rooms/"+code, nil); rec.Code != http.StatusNotFound {
		t.Errorf("the host's earlier room = %d, want 404", rec.Code)
	}
	if _, err := e.svc.room(other.Code()); err != nil {
		t.Error("another host's room went")
	}
	e.svc.mu.Lock()
	n := len(e.svc.rooms)
	e.svc.mu.Unlock()
	if n != 2 {
		t.Errorf("rooms = %d, want 2 (one per host)", n)
	}
}

// readEvent reads one SSE event's data line.
func readEvent(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	var event, data string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("stream: %v", err)
		}
		line = strings.TrimRight(line, "\n")
		switch {
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data += strings.TrimPrefix(line, "data: ")
		case line == "" && data != "":
			if event != "state" {
				t.Fatalf("event %q", event)
			}
			return data
		}
	}
}

func TestEvents_stateOnEveryChange(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	h := e.pairedHost()
	code := e.createRoom(h, Config{})
	srv := httptest.NewServer(e.mux)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/rooms/"+code+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("events = %d %v", resp.StatusCode, resp.Header)
	}
	r := bufio.NewReader(resp.Body)
	var s State
	if err := json.Unmarshal([]byte(readEvent(t, r)), &s); err != nil || s.Code != code || s.Phase != PhaseLobby {
		t.Fatalf("first event = %+v, %v", s, err)
	}
	e.browser().do(http.MethodPost, "/api/rooms/"+code+"/join", joinRequest{Name: "Mia", Avatar: "mic"})
	if err := json.Unmarshal([]byte(readEvent(t, r)), &s); err != nil || len(s.Players) != 1 || s.Players[0].Name != "Mia" {
		t.Errorf("join event = %+v, %v", s, err)
	}
	// Shutdown ends the stream.
	e.svc.Close()
	if _, err := r.ReadString('\n'); err == nil {
		for {
			if _, err := r.ReadString('\n'); err != nil {
				break
			}
		}
	}
}

func TestWait_HTTP(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	e.svc.pollHold = 50 * time.Millisecond
	h := e.pairedHost()
	code := e.createRoom(h, Config{})
	pad := e.browser()
	// No after: the state at once, and the pad gets its guest session.
	rec := pad.do(http.MethodGet, "/api/rooms/"+code+"/wait", nil)
	s := decodeBody[State](t, rec)
	if rec.Code != http.StatusOK || s.Code != code || pad.cookies["reworm_guest"] == nil {
		t.Fatalf("wait = %d %+v %v", rec.Code, s, pad.cookies)
	}
	// Up to date: the unchanged state after the hold.
	rec = pad.do(http.MethodGet, fmt.Sprintf("/api/rooms/%s/wait?after=%d", code, s.Seq), nil)
	if got := decodeBody[State](t, rec); rec.Code != http.StatusOK || got.Seq != s.Seq {
		t.Fatalf("held wait = %d seq %d, want %d", rec.Code, got.Seq, s.Seq)
	}
	// A change during the hold answers it.
	e.svc.pollHold = time.Minute
	done := make(chan *httptest.ResponseRecorder)
	go func() { done <- pad.do(http.MethodGet, fmt.Sprintf("/api/rooms/%s/wait?after=%d", code, s.Seq), nil) }()
	room, _ := e.svc.room(code)
	for {
		room.mu.Lock()
		n := len(room.polled)
		room.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond) // the poll subscribes under the same lock it marked itself in
	if rec := e.browser().do(http.MethodPost, "/api/rooms/"+code+"/join", joinRequest{Name: "Mia", Avatar: "mic"}); rec.Code != http.StatusOK {
		t.Fatalf("join = %d %s", rec.Code, rec.Body)
	}
	if got := decodeBody[State](t, <-done); got.Seq <= s.Seq || len(got.Players) != 1 {
		t.Errorf("woken = seq %d players %+v", got.Seq, got.Players)
	}
	if rec := pad.do(http.MethodGet, "/api/rooms/ZZZZ/wait", nil); rec.Code != http.StatusNotFound {
		t.Errorf("unknown room = %d", rec.Code)
	}
}

func TestStreamID(t *testing.T) {
	long := strings.Repeat("x", maxStreamID+1)
	for q, want := range map[string]string{"?sid=a1": "a1", "": "", "?sid=" + long: ""} {
		if got := streamID(httptest.NewRequest(http.MethodGet, "/x"+q, nil), "sid"); got != want {
			t.Errorf("%q = %q, want %q", q, got, want)
		}
	}
}

// Every publish answers every held poll at once: a full room behind one address fits the limiter.
func TestWait_limitFitsAnOffice(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	h := e.pairedHost()
	code := e.createRoom(h, Config{})
	b := e.browser()
	for i := range 100 {
		if rec := b.do(http.MethodGet, "/api/rooms/"+code+"/wait", nil); rec.Code != http.StatusOK {
			t.Fatalf("wait %d = %d", i, rec.Code)
		}
	}
}

func TestWait_HTTPCapReached(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	h := e.pairedHost()
	code := e.createRoom(h, Config{})
	room, _ := e.svc.room(code)
	room.close()
	rec := h.do(http.MethodGet, fmt.Sprintf("/api/rooms/%s/wait?after=%d", code, room.State(t0).Seq), nil)
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Errorf("refused poll = %d %v", rec.Code, rec.Header())
	}
}

func TestRoomLookups_guessingCodesIsLimited(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	h := e.pairedHost()
	code := e.createRoom(h, Config{})
	b := e.browser()
	// A known room costs nothing however often it is read.
	for range 40 {
		if rec := b.do(http.MethodGet, "/api/rooms/"+code, nil); rec.Code != http.StatusOK {
			t.Fatalf("known room = %d", rec.Code)
		}
	}
	if s := decodeBody[State](t, h.do(http.MethodGet, "/api/rooms/"+code, nil)); s.Now != e.clock().UnixMilli() {
		t.Errorf("now = %d", s.Now)
	}
	var last int
	for range 21 {
		last = b.do(http.MethodGet, "/api/rooms/ZZZZ", nil).Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("21st unknown code = %d, want 429", last)
	}
}

func TestSubscribe_perKeyCap(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	for i := range maxStreamsPerKey {
		if _, _, _, ok := r.Subscribe("", "198.51.100.1"); !ok {
			t.Fatalf("stream %d refused", i)
		}
	}
	if _, _, _, ok := r.Subscribe("", "198.51.100.1"); ok {
		t.Error("one address took more than its share")
	}
	if _, _, _, ok := r.Subscribe("", "198.51.100.2"); !ok {
		t.Error("another address refused")
	}
}

func TestEvents_padStreamBeforeJoinIsConnected(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	h := e.pairedHost()
	code := e.createRoom(h, Config{})
	srv := httptest.NewServer(e.mux)
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	pad := &http.Client{Jar: jar}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/rooms/"+code+"/events", nil)
	resp, err := pad.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	r := bufio.NewReader(resp.Body)
	readEvent(t, r)
	// The pad joins on the guest session its stream was given: connected at once.
	body, _ := json.Marshal(joinRequest{Name: "Mia", Avatar: "mic"})
	jr, err := pad.Post(srv.URL+"/api/rooms/"+code+"/join", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	_ = jr.Body.Close()
	var s State
	if err := json.Unmarshal([]byte(readEvent(t, r)), &s); err != nil || len(s.Players) != 1 || !s.Players[0].Connected {
		t.Errorf("after join = %+v, %v", s.Players, err)
	}
}

func TestEvents_capReached(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	h := e.pairedHost()
	code := e.createRoom(h, Config{})
	room, _ := e.svc.room(code)
	room.close() // a closed hub refuses like a full one
	if rec := h.do(http.MethodGet, "/api/rooms/"+code+"/events", nil); rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Errorf("refused stream = %d %v", rec.Code, rec.Header())
	}
}

// Under a path prefix the guest and host cookies stay scoped to the room API below it; the host's also reaches
// POST /api/rooms, so hosting again ends its earlier room.
func TestNew_cookiePaths(t *testing.T) {
	for base, want := range map[string]string{"": "/api/rooms", "/reworm": "/reworm/api/rooms"} {
		s, err := New(Options{Key: make([]byte, 32), BasePath: base})
		if err != nil {
			t.Fatal(err)
		}
		for sessions, path := range map[*auth.Sessions]string{s.guests: want + "/", s.hosts: want} {
			rec := httptest.NewRecorder()
			if err := sessions.Issue(rec, auth.Claims{Subject: "p", Name: "K7Q4"}); err != nil {
				t.Fatal(err)
			}
			if c := rec.Result().Cookies(); len(c) != 1 || c[0].Path != path {
				t.Errorf("base %q: cookies = %v, want path %s", base, c, path)
			}
		}
	}
}
