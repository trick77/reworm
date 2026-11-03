package train

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"time"

	"github.com/trick77/ship/kit/httpx"
	"github.com/trick77/ship/kit/logging"

	"github.com/trick77/reworm/internal/account"
	"github.com/trick77/reworm/internal/api"
	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/engine"
	"github.com/trick77/reworm/internal/preview"
	"github.com/trick77/reworm/internal/store"
)

// Options configures New.
type Options struct {
	DB       *sql.DB
	Accounts *account.Service
	Songs    *deck.Repo
	// Previews resolves clip URLs; nil answers 503.
	Previews preview.Source
	// Now is the clock; nil is time.Now.
	Now func() time.Time
	// Rand makes a request's random source; nil seeds one at random.
	Rand func() *rand.Rand
}

// Service is the training API.
type Service struct {
	o Options
}

// New returns the service.
func New(o Options) *Service {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Rand == nil {
		o.Rand = func() *rand.Rand {
			return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) //nolint:gosec // game randomness, not a secret
		}
	}
	return &Service{o: o}
}

// Mount adds the training routes.
func (s *Service) Mount(mux *http.ServeMux) {
	acc := s.o.Accounts
	mux.HandleFunc("POST /api/train/sessions", acc.Require(s.handleCreate))
	mux.HandleFunc("GET /api/train/sessions/{id}", acc.Require(s.handleGet))
	mux.HandleFunc("POST /api/train/sessions/{id}/answer", acc.Require(s.handleAnswer))
	mux.HandleFunc("GET /api/train/sessions/{id}/summary", acc.Require(s.handleSummary))
	mux.HandleFunc("GET /api/train/stats", acc.Require(s.handleStats))
}

var (
	errNoSession = api.Fail(http.StatusNotFound, "unknown_session", "no such session")
	errConflict  = api.Fail(http.StatusConflict, "conflict", "the session changed meanwhile; reload it")
	errNotCard   = api.Fail(http.StatusConflict, "wrong_card", "that card is not the current one")
	errDeckEmpty = api.Fail(http.StatusUnprocessableEntity, "deck_empty", "no playable song in this deck")
	errPreviews  = api.Fail(http.StatusServiceUnavailable, "previews_unavailable", "the preview source failed; try again")
)

// cardJSON is a card as the client sees it: never the year before the answer.
type cardJSON struct {
	ID           string      `json:"id"`
	Retry        bool        `json:"retry"`
	ProposedYear *int        `json:"proposedYear,omitempty"`
	Clip         engine.Clip `json:"clip"`
}

// stored is a loaded session with the text it was loaded as, for the optimistic write.
type stored struct {
	id      string
	profile string
	old     string
	s       session
}

func (s *Service) load(ctx context.Context, id, profile string) (*stored, error) {
	st := &stored{id: id, profile: profile}
	err := s.o.DB.QueryRowContext(ctx, `SELECT state FROM train_sessions WHERE id = ? AND profile_id = ?`, id, profile).Scan(&st.old)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNoSession
	}
	if err != nil {
		return nil, err
	}
	return st, json.Unmarshal([]byte(st.old), &st.s)
}

// save writes the session back in tx, refusing when another request changed it since it was loaded.
func (s *Service) save(ctx context.Context, tx *sql.Tx, st *stored) error {
	b, err := json.Marshal(st.s)
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `UPDATE train_sessions SET state = ?, updated_at = ? WHERE id = ? AND state = ?`,
		string(b), s.o.Now().UnixMilli(), st.id, st.old)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errConflict
	}
	st.old = string(b)
	return nil
}

// present makes the current card ready: its proposal drawn (shifted by the session so far and the song's
// progress) and its preview resolved. A song without a preview leaves the queue. It returns nil when the session is
// done, and whether the session changed.
func (s *Service) present(ctx context.Context, st *stored, rng *rand.Rand) (*cardJSON, bool, error) {
	if s.o.Previews == nil {
		return nil, false, api.ErrNoPreviews
	}
	changed := false
	for {
		c := st.s.current()
		if c == nil {
			return nil, changed, nil
		}
		song, err := s.o.Songs.Get(ctx, c.SongID)
		if err != nil {
			return nil, changed, err
		}
		u, err := s.o.Previews.PreviewURL(ctx, song)
		if errors.Is(err, preview.ErrNone) {
			slog.WarnContext(ctx, "training card skipped: no preview", "song", song.ID)
			st.s.Queue = slices.Delete(st.s.Queue, st.s.Index, st.s.Index+1)
			changed = true
			continue
		}
		if err != nil {
			slog.WarnContext(ctx, "preview lookup failed", "song", song.ID, logging.Err(err))
			return nil, changed, errPreviews
		}
		cfg := st.s.Config
		out := &cardJSON{ID: c.ID, Retry: c.Retry, Clip: engine.Clip{PreviewURL: u, From: c.From, Length: cfg.Clip.Length}}
		if cfg.Mode == ModeYears {
			if c.Proposed == 0 {
				p, err := loadProgress(ctx, s.o.DB, st.profile, c.SongID)
				if err != nil {
					return nil, changed, err
				}
				shift := engine.Shift(engine.Adapt{Index: st.s.Index, Streak: st.s.Streak, LastMiss: st.s.LastMiss,
					SongState: p.state(), Retry: c.Retry})
				// A re-ask shows another year than the miss did: the year itself is learned, not the button.
				for range 20 {
					c.Proposed, c.Tier = engine.Propose(rng, c.Year, cfg.Difficulty, shift, s.o.Now().Year())
					if c.Proposed != c.Avoid {
						break
					}
				}
				changed = true
			}
			y := c.Proposed
			out.ProposedYear = &y
		}
		return out, changed, nil
	}
}

type createResponse struct {
	ID   string    `json:"id"`
	Card *cardJSON `json:"card"`
	// Now is the server clock (unix ms), for the client's clock offset.
	Now int64 `json:"now"`
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request, id account.Identity) {
	var cfg Config
	if !api.Decode(w, r, &cfg) {
		return
	}
	resp, err := s.create(r.Context(), id.ProfileID, cfg)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (s *Service) create(ctx context.Context, profile string, cfg Config) (createResponse, error) {
	now := s.o.Now()
	if err := cfg.Normalize(now.Year()); err != nil {
		return createResponse{}, err
	}
	if s.o.Previews == nil {
		return createResponse{}, api.ErrNoPreviews
	}
	songs, err := s.songs(ctx, cfg)
	if err != nil {
		return createResponse{}, err
	}
	rng := s.o.Rand()
	order, err := s.order(ctx, profile, songs, rng, now)
	if err != nil {
		return createResponse{}, err
	}
	// Fill up to Length with songs that have a preview; one without is passed over, a failing source is not.
	st := &stored{id: api.ID(), profile: profile, s: session{Config: cfg}}
	for _, song := range order {
		if len(st.s.Queue) == cfg.Length {
			break
		}
		_, err := s.o.Previews.PreviewURL(ctx, song)
		if errors.Is(err, preview.ErrNone) {
			continue
		}
		if err != nil {
			slog.WarnContext(ctx, "preview lookup failed", "song", song.ID, logging.Err(err))
			return createResponse{}, errPreviews
		}
		st.s.Queue = append(st.s.Queue, card{ID: api.ID(), SongID: song.ID, Year: song.Year, From: engine.ClipFrom(rng, cfg.Clip)})
	}
	if len(st.s.Queue) == 0 {
		return createResponse{}, errDeckEmpty
	}
	c, _, err := s.present(ctx, st, rng)
	if err != nil {
		return createResponse{}, err
	}
	if c == nil {
		return createResponse{}, errDeckEmpty
	}
	b, err := json.Marshal(st.s)
	if err != nil {
		return createResponse{}, err
	}
	_, err = s.o.DB.ExecContext(ctx, `INSERT INTO train_sessions (id, profile_id, created_at, updated_at, state) VALUES (?, ?, ?, ?, ?)`,
		st.id, profile, now.UnixMilli(), now.UnixMilli(), string(b))
	return createResponse{ID: st.id, Card: c, Now: api.Millis(s.o.Now())}, err
}

// songs is what a session draws from: the drilled songs (a summary's misses) when songIds is set, else the deck.
func (s *Service) songs(ctx context.Context, cfg Config) ([]deck.Song, error) {
	if len(cfg.SongIDs) == 0 {
		return s.o.Songs.Select(ctx, cfg.Deck)
	}
	var out []deck.Song
	seen := map[string]bool{}
	for _, id := range cfg.SongIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		song, err := s.o.Songs.Get(ctx, id)
		if errors.Is(err, deck.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, song)
	}
	return out, nil
}

// order is the play order for a profile: songs due after a miss first, then new ones, then mastered songs that are
// due, then the rest; random within each.
func (s *Service) order(ctx context.Context, profile string, songs []deck.Song, rng *rand.Rand, now time.Time) ([]deck.Song, error) {
	rows, err := s.o.DB.QueryContext(ctx, `SELECT song_id, box, due_at FROM train_progress WHERE profile_id = ?`, profile)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	group := map[string]int{}
	for rows.Next() {
		var id string
		var box int
		var due int64
		if err := rows.Scan(&id, &box, &due); err != nil {
			return nil, err
		}
		switch {
		case due > now.UnixMilli():
			group[id] = 3
		case box >= masteredBox:
			group[id] = 2
		default:
			group[id] = 0
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := slices.Clone(songs)
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	rank := func(id string) int {
		if g, ok := group[id]; ok {
			return g
		}
		return 1 // new
	}
	slices.SortStableFunc(out, func(a, b deck.Song) int { return cmp.Compare(rank(a.ID), rank(b.ID)) })
	return out, nil
}

type statsJSON struct {
	Correct    int `json:"correct"`
	Answered   int `json:"answered"`
	Streak     int `json:"streak"`
	BestStreak int `json:"bestStreak"`
	Learning   int `json:"learning"`
	XP         int `json:"xp"`
}

type getResponse struct {
	ID    string    `json:"id"`
	Index int       `json:"index"`
	Total int       `json:"total"`
	Card  *cardJSON `json:"card"`
	Stats statsJSON `json:"stats"`
	Now   int64     `json:"now"`
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request, id account.Identity) {
	st, err := s.load(r.Context(), r.PathValue("id"), id.ProfileID)
	var c *cardJSON
	var changed bool
	if err == nil {
		c, changed, err = s.present(r.Context(), st, s.o.Rand())
	}
	if err == nil && changed {
		err = store.InTx(r.Context(), s.o.DB, func(tx *sql.Tx) error { return s.save(r.Context(), tx, st) })
	}
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	ss := st.s
	httpx.WriteJSON(w, http.StatusOK, getResponse{ID: st.id, Index: ss.Index, Total: len(ss.Queue), Card: c,
		Stats: statsJSON{Correct: ss.correct(), Answered: len(ss.Answers), Streak: ss.Streak, BestStreak: ss.Best,
			Learning: ss.learning(), XP: ss.XP}, Now: api.Millis(s.o.Now())})
}

type answerRequest struct {
	CardID string `json:"cardId"`
	Choice string `json:"choice"`
	Year   int    `json:"year"`
}

type answerResponse struct {
	Correct bool          `json:"correct"`
	Tier    engine.Tier   `json:"tier"`
	XP      int           `json:"xp"`
	Song    deck.Revealed `json:"song"`
	Next    *cardJSON     `json:"next"`
	Now     int64         `json:"now"`
}

func (s *Service) handleAnswer(w http.ResponseWriter, r *http.Request, id account.Identity) {
	var req answerRequest
	if !api.Decode(w, r, &req) {
		return
	}
	resp, err := s.answer(r.Context(), r.PathValue("id"), id.ProfileID, req)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (s *Service) answer(ctx context.Context, sessionID, profile string, req answerRequest) (answerResponse, error) {
	st, err := s.load(ctx, sessionID, profile)
	if err != nil {
		return answerResponse{}, err
	}
	rng := s.o.Rand()
	c := st.s.current()
	if c == nil || c.ID != req.CardID {
		return answerResponse{}, errNotCard
	}
	if st.s.Config.Mode == ModeYears && c.Proposed == 0 {
		// Answered before it was ever shown: draw what it would have shown.
		if _, _, err := s.present(ctx, st, rng); err != nil {
			return answerResponse{}, err
		}
		if c = st.s.current(); c == nil || c.ID != req.CardID {
			return answerResponse{}, errNotCard
		}
	}
	correct, tier, xp, err := judge(*c, st.s.Config.Mode, req.Choice, req.Year)
	if err != nil {
		return answerResponse{}, err
	}
	song, err := s.o.Songs.Get(ctx, c.SongID)
	if err != nil {
		return answerResponse{}, err
	}
	a := answer{CardID: c.ID, SongID: c.SongID, Year: c.Year, Proposed: c.Proposed, Choice: req.Choice, Guess: req.Year,
		Correct: correct, Tier: tier, XP: xp, Retry: c.Retry}
	if st.s.Config.Mode == ModePin {
		a.Choice = ""
	}
	st.s.record(a, api.ID)
	next, _, err := s.present(ctx, st, rng)
	if err != nil {
		return answerResponse{}, err
	}
	now := s.o.Now()
	err = store.InTx(ctx, s.o.DB, func(tx *sql.Tx) error {
		if err := s.save(ctx, tx, st); err != nil {
			return err
		}
		return saveAnswer(ctx, tx, profile, a, now)
	})
	if err != nil {
		return answerResponse{}, err
	}
	return answerResponse{Correct: correct, Tier: tier, XP: xp, Song: song.Reveal(), Next: next, Now: api.Millis(s.o.Now())}, nil
}

type missJSON struct {
	Song         deck.Revealed `json:"song"`
	ProposedYear *int          `json:"proposedYear"`
	Choice       any           `json:"choice"`
}

type summaryJSON struct {
	Correct    int           `json:"correct"`
	Total      int           `json:"total"`
	XP         int           `json:"xp"`
	BestStreak int           `json:"bestStreak"`
	ByDecade   []decadeCount `json:"byDecade"`
	Misses     []missJSON    `json:"misses"`
}

func (s *Service) handleSummary(w http.ResponseWriter, r *http.Request, id account.Identity) {
	st, err := s.load(r.Context(), r.PathValue("id"), id.ProfileID)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	ss := st.s
	out := summaryJSON{Correct: ss.correct(), Total: len(ss.Answers), XP: ss.XP, BestStreak: ss.Best,
		ByDecade: ss.byDecade(), Misses: []missJSON{}}
	for _, a := range ss.Answers {
		if a.Correct {
			continue
		}
		song, err := s.o.Songs.Get(r.Context(), a.SongID)
		if err != nil {
			api.WriteError(w, r, err)
			return
		}
		m := missJSON{Song: song.Reveal(), Choice: a.Choice}
		if ss.Config.Mode == ModePin {
			m.Choice = a.Guess
		} else {
			p := a.Proposed
			m.ProposedYear = &p
		}
		out.Misses = append(out.Misses, m)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

type decadeAccuracy struct {
	Decade   int     `json:"decade"`
	Accuracy float64 `json:"accuracy"`
}

type statsResponse struct {
	Award      string           `json:"award"`
	XP         int              `json:"xp"`
	NextAt     *int             `json:"nextAt"`
	Mastered   int              `json:"mastered"`
	Learning   int              `json:"learning"`
	New        int              `json:"new"`
	StreakDays int              `json:"streakDays"`
	Decades    []decadeAccuracy `json:"decades"`
}

func (s *Service) handleStats(w http.ResponseWriter, r *http.Request, id account.Identity) {
	out, err := s.stats(r.Context(), id.ProfileID)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (s *Service) stats(ctx context.Context, profile string) (statsResponse, error) {
	var out statsResponse
	var lastDay sql.NullString
	err := s.o.DB.QueryRowContext(ctx, `SELECT xp, streak_days, last_day FROM train_profiles WHERE profile_id = ?`, profile).
		Scan(&out.XP, &out.StreakDays, &lastDay)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	now := s.o.Now()
	if lastDay.String != day(now) && lastDay.String != day(now.AddDate(0, 0, -1)) {
		out.StreakDays = 0 // a day without training broke it
	}
	out.Award, out.NextAt = AwardFor(out.XP)
	var total int
	if err := s.o.DB.QueryRowContext(ctx, `
SELECT (SELECT count(*) FROM songs WHERE active = 1),
       coalesce(sum(p.box >= ?), 0), coalesce(sum(p.box < ?), 0)
FROM train_progress p JOIN songs s ON s.id = p.song_id AND s.active = 1 WHERE p.profile_id = ?`,
		masteredBox, masteredBox, profile).Scan(&total, &out.Mastered, &out.Learning); err != nil {
		return out, err
	}
	out.New = max(total-out.Mastered-out.Learning, 0)
	rows, err := s.o.DB.QueryContext(ctx, `
SELECT s.year / 10 * 10 AS decade, sum(p.correct), sum(p.attempts)
FROM train_progress p JOIN songs s ON s.id = p.song_id WHERE p.profile_id = ? GROUP BY decade ORDER BY decade`, profile)
	if err != nil {
		return out, err
	}
	defer func() { _ = rows.Close() }()
	out.Decades = []decadeAccuracy{}
	for rows.Next() {
		var d, ok, n int
		if err := rows.Scan(&d, &ok, &n); err != nil {
			return out, err
		}
		if n > 0 {
			out.Decades = append(out.Decades, decadeAccuracy{Decade: d, Accuracy: float64(ok) / float64(n)})
		}
	}
	if err := rows.Err(); err != nil {
		slog.WarnContext(ctx, "decade stats", logging.Err(err))
		return out, err
	}
	return out, nil
}
