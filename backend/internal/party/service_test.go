package party

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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
	e := &env{t: t, now: t0}
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
	if rec := e.browser().do(http.MethodPost, "/api/rooms", Config{}); rec.Code != http.StatusUnauthorized {
		t.Errorf("unpaired create = %d", rec.Code)
	}
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

	if rec := guest.do(http.MethodPost, base+"/start", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("guest start = %d", rec.Code)
	}
	rec = h.do(http.MethodPost, base+"/start", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("start = %d %s", rec.Code, rec.Body)
	}
	s := decodeBody[State](t, rec)
	if s.Phase != PhasePlaying || s.HostID == "" || len(s.Players) != 2 || s.Round.ProposedYear == nil {
		t.Fatalf("started = %+v", s)
	}
	room, _ := e.svc.room(code)
	room.round.proposed = room.round.pick.song.Year + 4 // one point, not the game
	choice := right(room)
	if rec := guest.do(http.MethodPost, base+"/answer", pressRequest{Round: 1, Choice: choice}); rec.Code != http.StatusOK ||
		!decodeBody[acceptedResponse](t, rec).Accepted {
		t.Fatalf("answer = %d %s", rec.Code, rec.Body)
	}
	if rec := e.browser().do(http.MethodPost, base+"/answer", pressRequest{Round: 1, Choice: choice}); rec.Code != http.StatusForbidden {
		t.Errorf("stranger answer = %d", rec.Code)
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

	played, err := e.repo.Played(context.Background(), s.HostID, t0.Add(-time.Hour))
	if err != nil || len(played) != 2 {
		t.Errorf("played history = %v, %v; want the two rounds", played, err)
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
	inv := decodeBody[struct{ Code string }](t, h.do(http.MethodPost, "/api/pair", map[string]string{"kind": "invite"}))
	other := e.browser()
	other.do(http.MethodPost, "/api/pair/claim", map[string]string{"code": inv.Code, "name": "Mia", "avatar": "mic"})
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
	for range MaxRooms {
		if _, err := e.svc.create("p", Config{}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := e.svc.create("p", Config{})
	wantProblem(t, err, "too_many_rooms")
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
