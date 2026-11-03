package train

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/trick77/reworm/internal/account"
	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/engine"
	"github.com/trick77/reworm/internal/preview"
	"github.com/trick77/reworm/internal/store"
)

type env struct {
	t    *testing.T
	db   *sql.DB
	svc  *Service
	acc  *account.Service
	mux  *http.ServeMux
	mu   sync.Mutex
	now  time.Time
	year map[string]int
	ck   []*http.Cookie
}

func newEnv(t *testing.T, previews preview.Source, n int) *env {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "reworm.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	e := &env{t: t, db: db, now: time.Date(2026, 11, 4, 20, 0, 0, 0, time.Local), year: map[string]int{}}
	for i := range n {
		id, y := fmt.Sprintf("s%d", i), 1960+7*i
		e.year[id] = y
		if _, err := db.ExecContext(ctx, `INSERT INTO songs (id, artist, title, year, year_source, sets, cover_url) VALUES (?, 'A', 'T', ?, 'card', '["essentials"]', 'https://x/{w}x{h}.jpg')`, id, y); err != nil {
			t.Fatal(err)
		}
	}
	e.acc, err = account.New(account.Options{DB: db, Key: make([]byte, 32), Now: e.clock})
	if err != nil {
		t.Fatal(err)
	}
	seed := uint64(0)
	e.svc = New(Options{DB: db, Accounts: e.acc, Songs: deck.New(db), Previews: previews, Now: e.clock,
		Rand: func() *rand.Rand { seed++; return rand.New(rand.NewPCG(seed, 99)) }})
	e.mux = http.NewServeMux()
	e.acc.Mount(e.mux)
	e.svc.Mount(e.mux)
	token, _ := e.acc.SetupToken(ctx)
	rec := e.do(http.MethodPost, "/api/setup", map[string]string{"token": token, "name": "Jan", "avatar": "vinyl"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup = %d", rec.Code)
	}
	e.ck = rec.Result().Cookies()
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

func (e *env) do(method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	for _, c := range e.ck {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, r)
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

// songOf is the song a card asks about, read from the stored session (the client never sees it).
func (e *env) songOf(sessionID, cardID string) card {
	e.t.Helper()
	var state string
	if err := e.db.QueryRow(`SELECT state FROM train_sessions WHERE id = ?`, sessionID).Scan(&state); err != nil {
		e.t.Fatal(err)
	}
	var s session
	_ = json.Unmarshal([]byte(state), &s)
	for _, c := range s.Queue {
		if c.ID == cardID {
			return c
		}
	}
	e.t.Fatalf("card %s not in the queue", cardID)
	return card{}
}

func choiceFor(c card, right bool) string {
	var ch string
	switch {
	case c.Year < c.Proposed:
		ch = "earlier"
	case c.Year > c.Proposed:
		ch = "later"
	default:
		ch = "exact"
	}
	if right {
		return ch
	}
	if ch == "earlier" {
		return "later"
	}
	return "earlier"
}

func TestSession_yearsFlow(t *testing.T) {
	e := newEnv(t, &preview.Fake{}, 6)
	rec := e.do(http.MethodPost, "/api/train/sessions", Config{Length: 5})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
	cr := decodeBody[createResponse](t, rec)
	if cr.Card == nil || cr.Card.ProposedYear == nil || cr.Card.Retry || cr.Card.Clip.Length != 20 || cr.Card.Clip.PreviewURL == "" {
		t.Fatalf("first card = %+v", cr.Card)
	}
	// Resume: the same card, the same year.
	g := decodeBody[getResponse](t, e.do(http.MethodGet, "/api/train/sessions/"+cr.ID, nil))
	if g.Card == nil || g.Card.ID != cr.Card.ID || *g.Card.ProposedYear != *cr.Card.ProposedYear || g.Index != 0 || g.Total != 5 {
		t.Fatalf("resume = %+v", g)
	}

	// Miss the first card: it comes back 3 cards later with another year.
	first := e.songOf(cr.ID, cr.Card.ID)
	rec = e.do(http.MethodPost, "/api/train/sessions/"+cr.ID+"/answer", answerRequest{CardID: cr.Card.ID, Choice: choiceFor(first, false)})
	if rec.Code != http.StatusOK {
		t.Fatalf("answer = %d %s", rec.Code, rec.Body)
	}
	ar := decodeBody[answerResponse](t, rec)
	if ar.Correct || ar.XP != 0 || ar.Song.Year != first.Year || ar.Song.Cover == nil || ar.Next == nil || ar.Tier != first.Tier {
		t.Fatalf("miss = %+v", ar)
	}
	if rec := e.do(http.MethodPost, "/api/train/sessions/"+cr.ID+"/answer", answerRequest{CardID: cr.Card.ID, Choice: "later"}); rec.Code != http.StatusConflict {
		t.Errorf("answer to an old card = %d", rec.Code)
	}
	cur := ar.Next
	var retry *cardJSON
	for i := 1; cur != nil; i++ {
		c := e.songOf(cr.ID, cur.ID)
		if cur.Retry {
			retry = cur
			if i != 4 {
				t.Errorf("re-ask at card %d, want 4 (three later)", i)
			}
			if c.SongID != first.SongID || c.Proposed == first.Proposed {
				t.Errorf("re-ask = %+v, first %+v", c, first)
			}
		}
		ar = decodeBody[answerResponse](t, e.do(http.MethodPost, "/api/train/sessions/"+cr.ID+"/answer", answerRequest{CardID: cur.ID, Choice: choiceFor(c, true)}))
		if !ar.Correct || ar.XP < 10 {
			t.Errorf("hit = %+v", ar)
		}
		cur = ar.Next
	}
	if retry == nil {
		t.Fatal("the miss never came back")
	}
	g = decodeBody[getResponse](t, e.do(http.MethodGet, "/api/train/sessions/"+cr.ID, nil))
	if g.Card != nil || g.Index != 6 || g.Total != 6 || g.Stats.Correct != 5 || g.Stats.Answered != 6 || g.Stats.Learning != 0 ||
		g.Stats.BestStreak != 5 || g.Stats.Streak != 5 {
		t.Errorf("done = %+v", g)
	}
	sum := decodeBody[summaryJSON](t, e.do(http.MethodGet, "/api/train/sessions/"+cr.ID+"/summary", nil))
	if sum.Correct != 5 || sum.Total != 6 || sum.XP != g.Stats.XP || sum.BestStreak != 5 || len(sum.Misses) != 1 ||
		sum.Misses[0].Song.ID != first.SongID || *sum.Misses[0].ProposedYear != first.Proposed || sum.Misses[0].Choice != choiceFor(first, false) {
		t.Errorf("summary = %+v", sum)
	}
	n := 0
	for _, d := range sum.ByDecade {
		n += d.N
	}
	if n != 6 {
		t.Errorf("by decade = %+v", sum.ByDecade)
	}

	st := decodeBody[statsResponse](t, e.do(http.MethodGet, "/api/train/stats", nil))
	// Four first-try hits (box 1, learning), the first song missed (box 0, learning), one of six never asked.
	if st.XP != sum.XP || st.Award != "bronze" || st.NextAt == nil || *st.NextAt != 250 || st.Learning != 5 ||
		st.Mastered != 0 || st.New != 1 || st.StreakDays != 1 || len(st.Decades) == 0 {
		t.Errorf("stats = %+v", st)
	}
}

func TestSession_pinMode(t *testing.T) {
	e := newEnv(t, &preview.Fake{}, 3)
	cr := decodeBody[createResponse](t, e.do(http.MethodPost, "/api/train/sessions", Config{Mode: ModePin, Length: 3}))
	if cr.Card.ProposedYear != nil {
		t.Error("pin mode proposes a year")
	}
	c := e.songOf(cr.ID, cr.Card.ID)
	ar := decodeBody[answerResponse](t, e.do(http.MethodPost, "/api/train/sessions/"+cr.ID+"/answer", answerRequest{CardID: c.ID, Year: c.Year}))
	if !ar.Correct || ar.XP != 30 || ar.Tier != engine.TierExact {
		t.Errorf("exact pin = %+v", ar)
	}
	c = e.songOf(cr.ID, ar.Next.ID)
	ar = decodeBody[answerResponse](t, e.do(http.MethodPost, "/api/train/sessions/"+cr.ID+"/answer", answerRequest{CardID: c.ID, Year: c.Year + 1}))
	if !ar.Correct || ar.XP != 20 || ar.Tier != engine.TierTight {
		t.Errorf("±1 pin = %+v", ar)
	}
	c = e.songOf(cr.ID, ar.Next.ID)
	if rec := e.do(http.MethodPost, "/api/train/sessions/"+cr.ID+"/answer", answerRequest{CardID: c.ID, Year: 12}); rec.Code != http.StatusBadRequest {
		t.Errorf("no year = %d", rec.Code)
	}
	ar = decodeBody[answerResponse](t, e.do(http.MethodPost, "/api/train/sessions/"+cr.ID+"/answer", answerRequest{CardID: c.ID, Year: c.Year + 12}))
	if ar.Correct || ar.Tier != engine.TierEasy {
		t.Errorf("12 off = %+v", ar)
	}
	sum := decodeBody[summaryJSON](t, e.do(http.MethodGet, "/api/train/sessions/"+cr.ID+"/summary", nil))
	if len(sum.Misses) != 1 || sum.Misses[0].ProposedYear != nil || sum.Misses[0].Choice != float64(c.Year+12) {
		t.Errorf("pin misses = %+v", sum.Misses)
	}
}

func TestSession_refusals(t *testing.T) {
	e := newEnv(t, &preview.Fake{}, 3)
	for _, c := range []struct {
		body any
		want int
	}{
		{Config{Mode: "quiz"}, http.StatusBadRequest},
		{Config{Difficulty: "x"}, http.StatusBadRequest},
		{Config{Length: 51}, http.StatusBadRequest},
		{Config{Clip: engine.ClipSettings{Length: 1}}, http.StatusBadRequest},
		{Config{Deck: deck.Selector{From: 2000, To: 1990}}, http.StatusBadRequest},
		{Config{Deck: deck.Selector{Sets: []string{"none"}}}, http.StatusUnprocessableEntity},
		{"x", http.StatusBadRequest},
	} {
		if rec := e.do(http.MethodPost, "/api/train/sessions", c.body); rec.Code != c.want {
			t.Errorf("create %+v = %d, want %d", c.body, rec.Code, c.want)
		}
	}
	for _, p := range []string{"/api/train/sessions/nope", "/api/train/sessions/nope/summary"} {
		if rec := e.do(http.MethodGet, p, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d", p, rec.Code)
		}
	}
	if rec := e.do(http.MethodPost, "/api/train/sessions/nope/answer", answerRequest{}); rec.Code != http.StatusNotFound {
		t.Errorf("answer unknown = %d", rec.Code)
	}
	cr := decodeBody[createResponse](t, e.do(http.MethodPost, "/api/train/sessions", Config{}))
	if rec := e.do(http.MethodPost, "/api/train/sessions/"+cr.ID+"/answer", answerRequest{CardID: cr.Card.ID, Choice: "maybe"}); rec.Code != http.StatusBadRequest {
		t.Errorf("bad choice = %d", rec.Code)
	}
	if rec := e.do(http.MethodPost, "/api/train/sessions/"+cr.ID+"/answer", "x"); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed = %d", rec.Code)
	}
	e.ck = nil
	if rec := e.do(http.MethodGet, "/api/train/stats", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("unpaired stats = %d", rec.Code)
	}
}

func TestSession_noPreviews(t *testing.T) {
	e := newEnv(t, nil, 3)
	if rec := e.do(http.MethodPost, "/api/train/sessions", Config{}); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("create without previews = %d, want 503", rec.Code)
	}
}

func TestSession_skipsSongsWithoutPreview(t *testing.T) {
	e := newEnv(t, &preview.Fake{Missing: map[string]bool{"s0": true, "s1": true}}, 3)
	cr := decodeBody[createResponse](t, e.do(http.MethodPost, "/api/train/sessions", Config{Length: 3}))
	g := decodeBody[getResponse](t, e.do(http.MethodGet, "/api/train/sessions/"+cr.ID, nil))
	if g.Total != 1 || e.songOf(cr.ID, cr.Card.ID).SongID != "s2" {
		t.Errorf("session = %+v", g)
	}
	all := newEnv(t, &preview.Fake{Missing: map[string]bool{"s0": true}}, 1)
	if rec := all.do(http.MethodPost, "/api/train/sessions", Config{}); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("nothing playable = %d", rec.Code)
	}
}

func TestProgress_spacedRepetitionAndStreak(t *testing.T) {
	e := newEnv(t, &preview.Fake{}, 3)
	ctx := context.Background()
	var profile string
	_ = e.db.QueryRow(`SELECT id FROM profiles`).Scan(&profile)
	// Answer s0 right three times on three days: mastered, the streak counts days.
	for i := range 3 {
		tx, _ := e.db.BeginTx(ctx, nil)
		if err := saveAnswer(ctx, tx, profile, answer{SongID: "s0", Year: 1960, Correct: true, XP: 10}, e.clock()); err != nil {
			t.Fatal(err)
		}
		_ = tx.Commit()
		if i < 2 {
			e.advance(24 * time.Hour)
		}
	}
	p, _ := loadProgress(ctx, e.db, profile, "s0")
	if p.box != 3 || p.state() != "mastered" || p.attempts != 3 {
		t.Errorf("progress = %+v", p)
	}
	st, _ := e.svc.stats(ctx, profile)
	if st.StreakDays != 3 || st.Mastered != 1 || st.XP != 30 {
		t.Errorf("stats = %+v", st)
	}
	// The next session asks the new songs first; s0 rests a week.
	songs, _ := deck.New(e.db).Select(ctx, deck.Selector{From: 1900, To: 2026})
	order, err := e.svc.order(ctx, profile, songs, rand.New(rand.NewPCG(1, 1)), e.clock())
	if err != nil || order[2].ID != "s0" {
		t.Errorf("order = %v, %v", order, err)
	}
	// A miss sends it back to box 0, due at once; a re-ask does not move the box.
	tx, _ := e.db.BeginTx(ctx, nil)
	_ = saveAnswer(ctx, tx, profile, answer{SongID: "s0", Year: 1960}, e.clock())
	_ = saveAnswer(ctx, tx, profile, answer{SongID: "s0", Year: 1960, Correct: true, Retry: true}, e.clock())
	_ = tx.Commit()
	p, _ = loadProgress(ctx, e.db, profile, "s0")
	if p.box != 0 || p.attempts != 5 || p.correct != 4 {
		t.Errorf("after miss and re-ask = %+v", p)
	}
	order, _ = e.svc.order(ctx, profile, songs, rand.New(rand.NewPCG(1, 1)), e.clock())
	if order[0].ID != "s0" {
		t.Errorf("a missed song is not first: %v", order)
	}
	// Two days off: the streak is gone.
	e.advance(48 * time.Hour)
	if st, _ := e.svc.stats(ctx, profile); st.StreakDays != 0 {
		t.Errorf("streak after a gap = %d", st.StreakDays)
	}
}

func TestSave_conflict(t *testing.T) {
	e := newEnv(t, &preview.Fake{}, 2)
	cr := decodeBody[createResponse](t, e.do(http.MethodPost, "/api/train/sessions", Config{}))
	var profile string
	_ = e.db.QueryRow(`SELECT id FROM profiles`).Scan(&profile)
	ctx := context.Background()
	a, _ := e.svc.load(ctx, cr.ID, profile)
	b, _ := e.svc.load(ctx, cr.ID, profile)
	a.s.XP, b.s.XP = 1, 2
	if err := e.svc.inTx(ctx, func(tx *sql.Tx) error { return e.svc.save(ctx, tx, a) }); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.inTx(ctx, func(tx *sql.Tx) error { return e.svc.save(ctx, tx, b) }); !errors.Is(err, errConflict) {
		t.Errorf("stale save = %v, want a conflict", err)
	}
	if _, err := e.svc.load(ctx, cr.ID, "someone-else"); !errors.Is(err, errNoSession) {
		t.Errorf("foreign session = %v", err)
	}
}

func TestAwardFor(t *testing.T) {
	for _, c := range []struct {
		xp   int
		name string
		next int
	}{{0, "bronze", 250}, {249, "bronze", 250}, {250, "silver", 750}, {1380, "gold", 1500}, {1500, "platinum", 3000}} {
		name, next := AwardFor(c.xp)
		if name != c.name || next == nil || *next != c.next {
			t.Errorf("AwardFor(%d) = %s %v", c.xp, name, next)
		}
	}
	if name, next := AwardFor(5000); name != "diamond" || next != nil {
		t.Errorf("diamond = %s %v", name, next)
	}
}

func TestJudge(t *testing.T) {
	c := card{Year: 1976, Proposed: 1978, Tier: engine.TierTight}
	for _, x := range []struct {
		choice string
		ok     bool
		xp     int
	}{{"earlier", true, 20}, {"later", false, 0}, {"exact", false, 0}} {
		ok, tier, xp, err := judge(c, ModeYears, x.choice, 0)
		if err != nil || ok != x.ok || xp != x.xp || tier != engine.TierTight {
			t.Errorf("%s = %v %s %d %v", x.choice, ok, tier, xp, err)
		}
	}
	if ok, _, xp, _ := judge(card{Year: 1976, Proposed: 1976}, ModeYears, "exact", 0); !ok || xp != 30 {
		t.Errorf("exact = %v %d", ok, xp)
	}
	if ok, _, xp, _ := judge(card{Year: 1976, Proposed: 1990}, ModeYears, "earlier", 0); !ok || xp != 10 {
		t.Errorf("far = %v %d", ok, xp)
	}
}
