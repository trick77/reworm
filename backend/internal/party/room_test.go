package party

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/trick77/reworm/internal/api"
	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/engine"
	"github.com/trick77/reworm/internal/preview"
)

const host = "host-profile"

var t0 = time.UnixMilli(1_790_000_000_000)

func songs(n int) []deck.Song {
	out := make([]deck.Song, n)
	for i := range out {
		out[i] = deck.Song{ID: fmt.Sprintf("s%d", i), Artist: fmt.Sprintf("Artist %d", i), Title: fmt.Sprintf("Title %d", i),
			Year: 1960 + i, HasCover: i%2 == 0}
	}
	return out
}

func loaderOf(s []deck.Song) Loader {
	return func(context.Context, deck.Selector) ([]deck.Song, error) { return s, nil }
}

func newTestRoom(t *testing.T, mode string) *Room {
	t.Helper()
	c := Config{Mode: mode, ToWin: 3}
	if err := c.Normalize(2026); err != nil {
		t.Fatal(err)
	}
	f := &preview.Fake{}
	return testRoom("K7Q4", host, c, fixed(t0), rand.New(rand.NewPCG(1, 2)), f.PreviewURL)
}

func fixed(t time.Time) func() time.Time { return func() time.Time { return t } }

// testRoom is newRoom without the countdown: the rule tests answer the moment a round starts. TestCountdown
// covers the countdown itself.
func testRoom(code, hostProfile string, c Config, clock func() time.Time, rng *rand.Rand, resolve Resolver) *Room {
	return newRoom(code, hostProfile, c, 0, clock, rng, resolve)
}

// setClock waits for background prefetches and sets the room's clock to now.
func setClock(r *Room, now time.Time) {
	r.bg.Wait()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clock = fixed(now)
}

// next plays the next round at now and waits for its prefetch.
func next(r *Room, now time.Time) (string, error) {
	setClock(r, now)
	id, err := r.Next(context.Background(), host)
	r.bg.Wait()
	return id, err
}

func joinAs(r *Room, now time.Time, session, profile, name, avatar string) (string, error) {
	return r.Join(now, Joiner{Session: session, ProfileID: profile, Name: name, Avatar: avatar})
}

// join adds n players and returns their ids.
func join(t *testing.T, r *Room, n int) []string {
	t.Helper()
	ids := make([]string, n)
	for i := range ids {
		id, err := joinAs(r, t0, "", "", fmt.Sprintf("P%d", i), "mic")
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = id
	}
	return ids
}

func start(t *testing.T, r *Room, n int) {
	t.Helper()
	if _, err := r.Start(context.Background(), host, loaderOf(songs(n))); err != nil {
		t.Fatal(err)
	}
	r.bg.Wait()
}

func wantProblem(t *testing.T, err error, code string) {
	t.Helper()
	var p *api.Problem
	if !errors.As(err, &p) || p.Code != code {
		t.Errorf("err = %v, want %s", err, code)
	}
}

// right and wrong are the choices that are and are not correct for the round.
func right(r *Room) string {
	y, p := r.round.pick.song.Year, r.round.proposed
	switch {
	case y < p:
		return "earlier"
	case y > p:
		return "later"
	}
	return "exact"
}

func wrong(r *Room) string {
	if right(r) == "earlier" {
		return "later"
	}
	return "earlier"
}

func playerOf(s State, id string) PlayerState {
	for _, p := range s.Players {
		if p.ID == id {
			return p
		}
	}
	return PlayerState{}
}

func TestYears_firstCorrectWinsWithPhotoFinish(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	p := join(t, r, 4)
	start(t, r, 5)
	r.round.proposed = r.round.pick.song.Year + 4 // not exact: a plain point

	s := r.State(t0)
	if s.Phase != PhasePlaying || s.Round.N != 1 || s.Round.ProposedYear == nil || s.Round.Result != nil ||
		s.Round.EndsAt != t0.Add(RoundTime).UnixMilli() || s.NextClip == nil || s.Round.Clip.PreviewURL == "" {
		t.Fatalf("state = %+v", s)
	}
	b, _ := json.Marshal(s)
	if strings.Contains(string(b), "Artist 0") || strings.Contains(string(b), "Title 0") {
		t.Fatalf("the song leaked before the result: %s", b)
	}

	if err := r.Answer(t0.Add(time.Second), p[0], 1, wrong(r)); err != nil {
		t.Fatal(err)
	}
	if err := r.Answer(t0.Add(time.Second), p[0], 1, right(r)); err == nil {
		t.Error("a player out answered again")
	}
	if err := r.Answer(t0.Add(2*time.Second), p[1], 1, right(r)); err != nil {
		t.Fatal(err)
	}
	if err := r.Answer(t0.Add(2*time.Second+30*time.Millisecond), p[2], 1, right(r)); err != nil {
		t.Errorf("photo finish answer = %v", err)
	}
	if err := r.Answer(t0.Add(2*time.Second+40*time.Millisecond), p[3], 1, wrong(r)); err != nil {
		t.Errorf("wrong answer in the window = %v", err)
	}
	s = r.State(t0.Add(3 * time.Second))
	res := s.Round.Result
	if s.Phase != PhaseResult || res == nil || res.WinnerID == nil || *res.WinnerID != p[1] || res.Points != 1 {
		t.Fatalf("result = %+v", res)
	}
	if res.Song.Artist != "Artist 0" || res.Song.Year != 1960 || res.Song.Cover == nil {
		t.Errorf("revealed song = %+v", res.Song)
	}
	if len(res.PhotoFinish) != 1 || res.PhotoFinish[0] != (PhotoFinish{PlayerID: p[2], DeltaMs: 30}) {
		t.Errorf("photo finish = %+v (wrong answers never count)", res.PhotoFinish)
	}
	if playerOf(s, p[1]).Score != 1 || playerOf(s, p[2]).Score != 0 || playerOf(s, p[0]).State != StateOut ||
		playerOf(s, p[2]).State != StateAnswered || playerOf(s, p[3]).State != StateOut {
		t.Errorf("players = %+v", s.Players)
	}
	// After the window nothing more counts.
	other, _ := joinAs(r, t0, "", "", "Late", "worm")
	wantProblem(t, r.Answer(t0.Add(3100*time.Millisecond), other, 1, right(r)), "late")
	wantProblem(t, r.Answer(t0.Add(3100*time.Millisecond), p[2], 1, right(r)), "already_answered")
}

func TestYears_exactScoresTwoAndTheWindowCloses(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	p := join(t, r, 2)
	start(t, r, 5)
	r.round.proposed = r.round.pick.song.Year
	if err := r.Answer(t0.Add(time.Second), p[0], 1, "exact"); err != nil {
		t.Fatal(err)
	}
	wantProblem(t, r.Answer(t0.Add(2001*time.Millisecond), p[1], 1, "exact"), "wrong_round")
	s := r.State(t0.Add(3 * time.Second))
	if s.Round.Result.Points != 2 || playerOf(s, p[0]).Score != 2 {
		t.Errorf("exact = %+v", s.Round.Result)
	}
}

func TestYears_allOutAndTimer(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	p := join(t, r, 2)
	start(t, r, 5)
	for _, id := range p {
		if err := r.Answer(t0.Add(time.Second), id, 1, wrong(r)); err != nil {
			t.Fatal(err)
		}
	}
	if s := r.State(t0.Add(time.Second)); s.Phase != PhaseResult || s.Round.Result.WinnerID != nil || s.Round.Result.Points != 0 {
		t.Errorf("all out = %+v", s.Round)
	}
	if _, err := next(r, t0.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	// Nobody answers: the timer reveals.
	r.Tick(t0.Add(2*time.Second + RoundTime - time.Millisecond))
	if r.phase != PhasePlaying {
		t.Fatal("ended before the timer")
	}
	r.Tick(t0.Add(2*time.Second + RoundTime))
	if s := r.State(t0.Add(20 * time.Second)); s.Phase != PhaseResult || s.Round.Result.WinnerID != nil {
		t.Errorf("after the timer = %+v", s)
	}
}

func TestCountdown(t *testing.T) {
	for _, mode := range []string{ModeYears, ModeBuzzer} {
		t.Run(mode, func(t *testing.T) {
			r := newTestRoom(t, mode)
			r.countdown = Countdown
			p := join(t, r, 1)
			start(t, r, 5)
			go1 := t0.Add(Countdown)
			s := r.State(t0)
			if s.Phase != PhaseCountdown || s.Round.StartedAt != go1.UnixMilli() || s.Round.EndsAt != go1.Add(RoundTime).UnixMilli() {
				t.Fatalf("started = %s %+v", s.Phase, s.Round)
			}
			// Nothing counts before the clip: no answer, no buzz, no pause.
			before := t0.Add(Countdown - time.Millisecond)
			if mode == ModeYears {
				wantProblem(t, r.Answer(before, p[0], 1, right(r)), "wrong_phase")
			} else {
				wantProblem(t, r.Buzz(before, p[0], 1), "wrong_phase")
			}
			wantProblem(t, r.Pause(before, host), "wrong_phase")
			// Who joins during the countdown plays this round.
			late, err := joinAs(r, before, "", "", "Late", "mic")
			if err != nil {
				t.Fatal(err)
			}
			if st := playerOf(r.State(before), late).State; st != StateThinking {
				t.Errorf("joined in the countdown = %s, want thinking", st)
			}
			// The tick at startedAt starts the round; the press counts from there.
			r.Tick(go1)
			if r.phase != PhasePlaying {
				t.Fatalf("after the countdown = %s", r.phase)
			}
			at := go1.Add(1500 * time.Millisecond)
			if mode == ModeYears {
				r.round.proposed = r.round.pick.song.Year + 4
				err = r.Answer(at, p[0], 1, right(r))
			} else {
				err = r.Buzz(at, p[0], 1)
			}
			if err != nil {
				t.Fatal(err)
			}
			if ms := r.round.pressMs; ms != 1500 {
				t.Errorf("pressMs = %d, want 1500 (from the clip's start)", ms)
			}
		})
	}
}

func TestCountdown_kickingTheLastPlayerEndsTheRound(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	r.countdown = Countdown
	p := join(t, r, 1)
	start(t, r, 5)
	if err := r.Kick(t0.Add(time.Second), host, p[0]); err != nil {
		t.Fatal(err)
	}
	if s := r.State(t0.Add(time.Second)); s.Phase != PhaseResult || s.Round.Result.WinnerID != nil {
		t.Errorf("after the kick = %s %+v", s.Phase, s.Round)
	}
}

func TestCountdown_endedByARefusedPressStillPublishes(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	r.countdown = Countdown
	p := join(t, r, 1)
	start(t, r, 5)
	seq := r.State(t0).Seq
	// A stale press after startedAt starts the round in touch, then fails: nothing published yet.
	wantProblem(t, r.Answer(t0.Add(Countdown), p[0], 7, "later"), "wrong_round")
	if r.phase != PhasePlaying || r.seq != seq {
		t.Fatalf("after the refused press = %s, seq %d (was %d)", r.phase, r.seq, seq)
	}
	r.Tick(t0.Add(Countdown + time.Millisecond))
	if r.seq != seq+1 {
		t.Errorf("the tick did not publish the start: seq %d, want %d", r.seq, seq+1)
	}
}

func TestCountdown_everyRound(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	r.countdown = Countdown
	p := join(t, r, 1)
	start(t, r, 5)
	r.Tick(t0.Add(Countdown))
	r.round.proposed = r.round.pick.song.Year + 4
	if err := r.Answer(t0.Add(Countdown+time.Second), p[0], 1, right(r)); err != nil {
		t.Fatal(err)
	}
	at := t0.Add(10 * time.Second)
	if _, err := next(r, at); err != nil {
		t.Fatal(err)
	}
	if s := r.State(at); s.Phase != PhaseCountdown || s.Round.N != 2 || s.Round.StartedAt != at.Add(Countdown).UnixMilli() {
		t.Errorf("next round = %s %+v", s.Phase, s.Round)
	}
}

func TestYears_inputChecks(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	p := join(t, r, 1)
	wantProblem(t, r.Answer(t0, p[0], 1, "later"), "wrong_round")
	start(t, r, 3)
	wantProblem(t, r.Answer(t0, "stranger", 1, "later"), "not_in_room")
	wantProblem(t, r.Answer(t0, "", 1, "later"), "not_in_room")
	wantProblem(t, r.Answer(t0, p[0], 2, "later"), "wrong_round")
	wantProblem(t, r.Answer(t0, p[0], 1, "sideways"), "invalid_choice")
	wantProblem(t, r.Buzz(t0, p[0], 1), "wrong_mode")
}

func TestGame_toWinPauseNextRematch(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	p := join(t, r, 2)
	start(t, r, 4)
	now := t0
	seen := map[string]bool{r.round.pick.song.ID: true}

	// Pause during a round: the deadline moves by the pause.
	now = now.Add(5 * time.Second)
	wantProblem(t, r.Pause(now, "not-host"), "not_host")
	if err := r.Pause(now, host); err != nil {
		t.Fatal(err)
	}
	wantProblem(t, r.Answer(now, p[0], 1, right(r)), "wrong_phase")
	r.Tick(now.Add(time.Hour))
	if r.phase != PhasePaused {
		t.Fatal("a paused round timed out")
	}
	if got := r.state().PausedFrom; got != PhasePlaying {
		t.Fatalf("pausedFrom while paused = %q, want %q", got, PhasePlaying)
	}
	wantProblem(t, r.Pause(now, host), "wrong_phase")
	now = now.Add(time.Minute)
	if err := r.Resume(now, host); err != nil {
		t.Fatal(err)
	}
	if got := r.state().PausedFrom; got != "" {
		t.Fatalf("pausedFrom after resume = %q, want none", got)
	}
	if got := r.round.endsAt; !got.Equal(t0.Add(RoundTime + time.Minute)) {
		t.Errorf("endsAt after pause = %v, want +1 min", got)
	}
	wantProblem(t, r.Resume(now, host), "wrong_phase")

	// Three wins of p[0] end the game (toWin 3).
	for n := 1; ; n++ {
		r.round.proposed = r.round.pick.song.Year + 5
		if err := r.Answer(now, p[0], n, right(r)); err != nil {
			t.Fatal(err)
		}
		if r.phase == PhaseOver {
			break
		}
		wantProblem(t, r.Rematch(now, host), "wrong_phase")
		if _, err := next(r, now); err != nil {
			t.Fatal(err)
		}
		if seen[r.round.pick.song.ID] {
			t.Fatalf("song %s repeated", r.round.pick.song.ID)
		}
		seen[r.round.pick.song.ID] = true
	}
	if s := r.State(now); playerOf(s, p[0]).Score != 3 || s.Round.Result == nil {
		t.Errorf("game over = %+v", s)
	}
	wantProblem(t, func() error { _, err := next(r, now); return err }(), "wrong_phase")
	if err := r.Rematch(now, host); err != nil {
		t.Fatal(err)
	}
	if s := r.State(now); s.Phase != PhaseLobby || s.Round != nil || playerOf(s, p[0]).Score != 0 {
		t.Errorf("rematch = %+v", s)
	}
}

func TestNext_deckRunsOutAndSkipsMissingPreviews(t *testing.T) {
	c := Config{}
	_ = c.Normalize(2026)
	f := &preview.Fake{Missing: map[string]bool{"s1": true}}
	r := testRoom("K7Q4", host, c, fixed(t0), rand.New(rand.NewPCG(1, 2)), f.PreviewURL)
	p := join(t, r, 1)
	if _, err := r.Start(context.Background(), host, loaderOf(songs(3))); err != nil {
		t.Fatal(err)
	}
	r.bg.Wait()
	if r.next == nil || r.next.song.ID != "s2" {
		t.Fatalf("next = %+v, want s2 (s1 has no preview)", r.next)
	}
	_ = r.Answer(t0, p[0], 1, wrong(r))
	id, err := next(r, t0)
	if err != nil || id != "s2" || r.next != nil {
		t.Fatalf("Next = %q, %v; next %+v", id, err, r.next)
	}
	if s := r.State(t0); s.NextClip != nil {
		t.Error("nextClip with nothing left")
	}
	_ = r.Answer(t0, p[0], 2, wrong(r))
	id, err = next(r, t0)
	if err != nil || id != "" || r.phase != PhaseOver {
		t.Errorf("deck done: %q, %v, phase %s", id, err, r.phase)
	}
}

// failing is a preview source that fails (not "no preview") for the ids in down, ErrNone for none.
type failing struct {
	mu   sync.Mutex
	down map[string]bool
	none map[string]bool
}

func (f *failing) set(id string, down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down[id] = down
}

func (f *failing) PreviewURL(_ context.Context, s deck.Song) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case f.down[s.ID]:
		return "", errors.New("apple: 503")
	case f.none[s.ID]:
		return "", preview.ErrNone
	}
	return "https://previews.invalid/" + s.ID, nil
}

func TestPrefetch_onlyNoPreviewDropsASong(t *testing.T) {
	f := &failing{down: map[string]bool{"s0": true}, none: map[string]bool{"s2": true}}
	r := testRoom("K7Q4", host, Config{Mode: ModeYears, ToWin: 10}, fixed(t0), rand.New(rand.NewPCG(1, 2)), f.PreviewURL)
	p := join(t, r, 1)
	// The source is down for the first song: Start fails, the queue keeps it.
	_, err := r.Start(context.Background(), host, loaderOf(songs(4)))
	wantProblem(t, err, "previews_unavailable")
	if r.phase != PhaseLobby || len(r.queue) != 4 || r.queue[0].ID != "s0" {
		t.Fatalf("after a failed lookup: phase %s queue %d", r.phase, len(r.queue))
	}
	f.set("s0", false)
	f.set("s1", true)
	if _, err := r.Start(context.Background(), host, loaderOf(songs(4))); err != nil {
		t.Fatal(err)
	}
	r.bg.Wait()
	// s1 is down while prefetching: no nextClip, s1 still queued; Next tries again and fails cleanly.
	if r.next != nil || r.queue[0].ID != "s1" {
		t.Fatalf("next = %+v, queue head %s", r.next, r.queue[0].ID)
	}
	_ = r.Answer(t0, p[0], 1, wrong(r))
	_, err = next(r, t0)
	wantProblem(t, err, "previews_unavailable")
	if r.phase != PhaseResult {
		t.Errorf("phase after a failed Next = %s", r.phase)
	}
	// Back up: s1 plays; s2 has no preview and leaves; s3 is prefetched.
	f.set("s1", false)
	if id, err := next(r, t0); err != nil || id != "s1" || r.next == nil || r.next.song.ID != "s3" {
		t.Errorf("Next = %q, %v, next %+v", id, err, r.next)
	}
}

// blocking is a preview source that waits until release is closed.
type blocking struct {
	entered chan struct{}
	release chan struct{}
}

func (b *blocking) PreviewURL(ctx context.Context, s deck.Song) (string, error) {
	select {
	case b.entered <- struct{}{}:
	default:
	}
	select {
	case <-b.release:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return "https://previews.invalid/" + s.ID, nil
}

func TestStart_slowPreviewsHoldNothingUp(t *testing.T) {
	e := newEnv(t, &preview.Fake{})
	slow := &blocking{entered: make(chan struct{}, 1), release: make(chan struct{})}
	busy := testRoom("SLOW", host, Config{Mode: ModeYears, ToWin: 10}, fixed(t0), rand.New(rand.NewPCG(1, 2)), slow.PreviewURL)
	join(t, busy, 1)
	other, err := e.svc.create(host, Config{Mode: ModeYears, ToWin: 10})
	if err != nil {
		t.Fatal(err)
	}
	e.svc.mu.Lock()
	e.svc.rooms[busy.code] = busy
	e.svc.mu.Unlock()
	join(t, other, 1)
	if _, err := other.Start(context.Background(), host, loaderOf(songs(3))); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := busy.Start(context.Background(), host, loaderOf(songs(3)))
		done <- err
	}()
	<-slow.entered
	// While busy waits on its preview: the other room's timer still fires, and busy itself answers.
	e.svc.Tick(e.clock().Add(RoundTime))
	if s := other.State(e.clock().Add(RoundTime)); s.Phase != PhaseResult {
		t.Errorf("other room = %s, want result", s.Phase)
	}
	if s := busy.State(t0); s.Phase != PhaseLobby {
		t.Errorf("busy room = %s", s.Phase)
	}
	close(slow.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestStart_settingsChangedWhileLoadingDiscardsTheOldDeck(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	join(t, r, 1)
	loading, release := make(chan struct{}), make(chan struct{})
	slowLoad := func(context.Context, deck.Selector) ([]deck.Song, error) {
		close(loading)
		<-release
		return songs(3), nil
	}
	done := make(chan error, 1)
	go func() { _, err := r.Start(context.Background(), host, slowLoad); done <- err }()
	<-loading
	c := Config{Mode: ModeYears, ToWin: 3, Deck: deck.Selector{Sets: []string{"other"}}}
	_ = c.Normalize(2026)
	if err := r.Configure(t0, host, c); err != nil {
		t.Fatal(err)
	}
	close(release)
	wantProblem(t, <-done, "wrong_phase")
	if r.phase != PhaseLobby || r.queue != nil {
		t.Errorf("phase %s, queue %d", r.phase, len(r.queue))
	}
}

func TestStart_answersBeforeTheNextClipResolves(t *testing.T) {
	slow := &blocking{entered: make(chan struct{}, 1), release: make(chan struct{})}
	first := true
	var mu sync.Mutex
	resolve := func(ctx context.Context, s deck.Song) (string, error) {
		mu.Lock()
		isFirst := first
		first = false
		mu.Unlock()
		if isFirst {
			return "https://previews.invalid/" + s.ID, nil
		}
		return slow.PreviewURL(ctx, s)
	}
	r := testRoom("K7Q4", host, Config{Mode: ModeYears, ToWin: 3}, fixed(t0), rand.New(rand.NewPCG(1, 2)), resolve)
	join(t, r, 1)
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := r.Start(ctx, host, loaderOf(songs(3))); err != nil {
		t.Fatal(err)
	}
	cancel() // the host's request is over; the prefetch is not tied to it
	<-slow.entered
	if s := r.State(t0); s.Phase != PhasePlaying || s.NextClip != nil {
		t.Fatalf("state = %s, nextClip %v", s.Phase, s.NextClip)
	}
	close(slow.release)
	r.bg.Wait()
	if s := r.State(t0); s.NextClip == nil {
		t.Error("nextClip never arrived")
	}
}

func TestState_snapshotsDoNotRace(t *testing.T) {
	r := newTestRoom(t, ModeBuzzer)
	p := join(t, r, MaxPlayers)
	start(t, r, 3)
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, _ = json.Marshal(r.State(t0))
			}
		})
	}
	_ = r.Buzz(t0, p[0], 1)
	for _, id := range p[1:] {
		_ = r.Buzz(t0.Add(10*time.Millisecond), id, 1)
	}
	_ = r.Reveal(t0, p[0], 1)
	_ = r.Reveal(t0, p[1], 1)
	for _, id := range p {
		_ = r.Judge(t0, id, 1, 1)
	}
	r.Tick(t0.Add(time.Hour))
	close(stop)
	wg.Wait()
	if s := r.State(t0); s.Round.Result == nil || len(s.Round.Result.PhotoFinish) != MaxPlayers-1 {
		t.Errorf("result = %+v", s.Round.Result)
	}
}

func TestSubscribe_streamOpenedBeforeJoinCounts(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	_, stop, _, _ := r.Subscribe("guest-session", "")
	defer stop()
	_, stopDev, _, _ := r.Subscribe(deviceSession("dev-1"), "")
	defer stopDev()
	id, err := joinAs(r, t0, "guest-session", "", "Mia", "mic")
	if err != nil || id != "guest-session" {
		t.Fatalf("join = %q, %v", id, err)
	}
	pro, _ := r.Join(t0, Joiner{ProfileID: "profile-1", DeviceID: "dev-1", Name: "Jan", Avatar: "vinyl"})
	s := r.State(t0)
	if !playerOf(s, id).Connected || !playerOf(s, pro).Connected {
		t.Errorf("players = %+v", s.Players)
	}
}

func TestStart_refusals(t *testing.T) {
	ctx := context.Background()
	r := newTestRoom(t, ModeYears)
	_, err := r.Start(ctx, host, loaderOf(songs(2)))
	wantProblem(t, err, "no_players")
	join(t, r, 1)
	_, err = r.Start(ctx, "x", loaderOf(songs(2)))
	wantProblem(t, err, "not_host")
	_, err = r.Start(ctx, host, loaderOf(nil))
	wantProblem(t, err, "deck_empty")
	if _, err := r.Start(ctx, host, func(context.Context, deck.Selector) ([]deck.Song, error) {
		return nil, errors.New("db down")
	}); err == nil || errors.As(err, new(*api.Problem)) {
		t.Errorf("load failure = %v", err)
	}
	start(t, r, 2)
	_, err = r.Start(ctx, host, loaderOf(songs(2)))
	wantProblem(t, err, "wrong_phase")

	none := testRoom("ABCD", host, Config{Mode: ModeYears}, fixed(t0), rand.New(rand.NewPCG(1, 2)), nil)
	join(t, none, 1)
	_, err = none.Start(ctx, host, loaderOf(songs(2)))
	wantProblem(t, err, "previews_unavailable")
}

func TestJoin(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	p := join(t, r, MaxPlayers-1)
	pro, err := joinAs(r, t0, "", "profile-1", "Jan", "vinyl")
	if err != nil {
		t.Fatal(err)
	}
	if again, err := joinAs(r, t0, "", "profile-1", "Jan B.", "keys"); err != nil || again != pro {
		t.Errorf("profile rejoin = %q, %v", again, err)
	}
	if r.PlayerOf("profile-1") != pro || r.PlayerOf("nobody") != "" {
		t.Error("PlayerOf")
	}
	if again, err := joinAs(r, t0, p[0], "", "P0", "mic"); err != nil || again != p[0] {
		t.Errorf("guest rejoin = %q, %v", again, err)
	}
	_, err = joinAs(r, t0, "", "", "Ninth", "mic")
	wantProblem(t, err, "room_full")

	// Late join mid-round: late for this round, in from the next.
	r2 := newTestRoom(t, ModeYears)
	q := join(t, r2, 1)
	start(t, r2, 3)
	late, _ := joinAs(r2, t0, "", "", "Late", "worm")
	if s := r2.State(t0); playerOf(s, late).State != StateLate {
		t.Errorf("late joiner state = %q", playerOf(s, late).State)
	}
	wantProblem(t, r2.Answer(t0, late, 1, right(r2)), "late")
	_ = r2.Answer(t0, q[0], 1, wrong(r2))
	if _, err := next(r2, t0); err != nil {
		t.Fatal(err)
	}
	if err := r2.Answer(t0, late, 2, right(r2)); err != nil {
		t.Errorf("late joiner in the next round = %v", err)
	}
}

func TestKick_bansForTheRoomsLife(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	guest := join(t, r, 1)[0]
	pro, _ := joinAs(r, t0, "", "profile-1", "Jan", "vinyl")
	for _, id := range []string{guest, pro} {
		if err := r.Kick(t0, host, id); err != nil {
			t.Fatal(err)
		}
	}
	_, err := joinAs(r, t0, guest, "", "Again", "mic")
	wantProblem(t, err, "kicked")
	_, err = joinAs(r, t0, "", "profile-1", "Jan", "vinyl")
	wantProblem(t, err, "kicked")
	wantProblem(t, r.Member(guest), "kicked")
	wantProblem(t, r.Member("never"), "not_in_room")
	_, _ = joinAs(r, t0, "", "", "Mia", "mic")
	start(t, r, 3)
	wantProblem(t, r.Answer(t0, guest, 1, right(r)), "kicked")
}

func TestKick_whilePausedSettlesOnResume(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	p := join(t, r, 2)
	start(t, r, 3)
	_ = r.Answer(t0, p[0], 1, wrong(r))
	_ = r.Pause(t0, host)
	if err := r.Kick(t0, host, p[1]); err != nil {
		t.Fatal(err)
	}
	if r.phase != PhasePaused {
		t.Fatalf("kick ended the pause: %s", r.phase)
	}
	if err := r.Resume(t0.Add(time.Second), host); err != nil {
		t.Fatal(err)
	}
	if r.phase != PhaseResult {
		t.Errorf("after resume = %s, want the round settled", r.phase)
	}

	// The buzzer kicked while paused: still paused, and the round ends without points on resume.
	b := newTestRoom(t, ModeBuzzer)
	q := join(t, b, 2)
	start(t, b, 3)
	_ = b.Buzz(t0, q[0], 1)
	_ = b.Pause(t0, host)
	_ = b.Kick(t0, host, q[0])
	if b.phase != PhasePaused {
		t.Fatalf("phase %s", b.phase)
	}
	_ = b.Resume(t0, host)
	if b.phase != PhaseResult || b.round.result.Points != 0 {
		t.Errorf("after resume = %s %+v", b.phase, b.round.result)
	}
}

func TestState_seqWinnerMsAndJudgeEndsAt(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	p := join(t, r, 2)
	s0 := r.State(t0).Seq
	start(t, r, 3)
	s1 := r.State(t0).Seq
	if s1 <= s0 {
		t.Errorf("seq %d after start, %d before", s1, s0)
	}
	r.round.proposed = r.round.pick.song.Year + 5
	_ = r.Pause(t0.Add(time.Second), host)
	_ = r.Resume(t0.Add(time.Minute), host)
	_ = r.Answer(t0.Add(time.Minute+2*time.Second), p[0], 1, right(r))
	s := r.State(t0.Add(2 * time.Minute))
	// 1 s before the pause, 2 s after it: 3000 ms of round time.
	if s.Seq <= s1 || s.Round.Result.WinnerMs == nil || *s.Round.Result.WinnerMs != 3000 {
		t.Errorf("seq %d, winnerMs %v", s.Seq, s.Round.Result.WinnerMs)
	}
	if s.Round.JudgeEndsAt != nil {
		t.Error("judgeEndsAt in years mode")
	}

	b := newTestRoom(t, ModeBuzzer)
	q := join(t, b, 2)
	start(t, b, 3)
	_ = b.Buzz(t0.Add(4*time.Second), q[0], 1)
	_ = b.Reveal(t0.Add(5*time.Second), q[0], 1)
	_ = b.Reveal(t0.Add(5*time.Second), q[1], 1)
	st := b.State(t0.Add(5 * time.Second))
	if st.Round.JudgeEndsAt == nil || *st.Round.JudgeEndsAt != t0.Add(5*time.Second+JudgeTime).UnixMilli() {
		t.Errorf("judgeEndsAt = %v", st.Round.JudgeEndsAt)
	}
	_ = b.Judge(t0.Add(6*time.Second), q[0], 1, 1)
	_ = b.Judge(t0.Add(6*time.Second), q[1], 1, 1)
	if ms := b.State(t0.Add(6 * time.Second)).Round.Result.WinnerMs; ms == nil || *ms != 4000 {
		t.Errorf("buzzer winnerMs = %v", ms)
	}
}

func TestKickAndSettings(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	p := join(t, r, 2)
	c := Config{Mode: ModeBuzzer, ToWin: 5}
	_ = c.Normalize(2026)
	wantProblem(t, r.Configure(t0, "x", c), "not_host")
	if err := r.Configure(t0, host, c); err != nil {
		t.Fatal(err)
	}
	if s := r.State(t0); s.Mode != ModeBuzzer || s.ToWin != 5 {
		t.Errorf("settings = %+v", s)
	}
	c.Mode = ModeYears
	_ = r.Configure(t0, host, c)
	start(t, r, 3)
	wantProblem(t, r.Configure(t0, host, c), "wrong_phase")
	wantProblem(t, r.Kick(t0, "x", p[0]), "not_host")
	wantProblem(t, r.Kick(t0, host, "nobody"), "unknown_player")
	_ = r.Answer(t0, p[0], 1, wrong(r))
	// The only player still thinking leaves: the round ends.
	if err := r.Kick(t0, host, p[1]); err != nil {
		t.Fatal(err)
	}
	if s := r.State(t0); s.Phase != PhaseResult || len(s.Players) != 1 {
		t.Errorf("after kick = %+v", s)
	}
}

func TestConfig_Normalize(t *testing.T) {
	var c Config
	if err := c.Normalize(2026); err != nil || c.Mode != ModeYears || c.ToWin != 10 || c.Difficulty != "medium" ||
		c.Clip.Length != 20 || c.Deck.To != 2026 {
		t.Errorf("defaults = %+v, %v", c, err)
	}
	for _, bad := range []Config{{Mode: "quiz"}, {ToWin: 51}, {Difficulty: "brutal"}, {Clip: engine.ClipSettings{Length: 1}},
		{Deck: deck.Selector{From: 2000, To: 1990}}} {
		if err := bad.Normalize(2026); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}
}

func TestBuzzer_buzzRevealJudge(t *testing.T) {
	r := newTestRoom(t, ModeBuzzer)
	p := join(t, r, 3)
	start(t, r, 4)
	if s := r.State(t0); s.Round.ProposedYear != nil {
		t.Error("buzzer mode shows a year")
	}
	wantProblem(t, r.Answer(t0, p[0], 1, "later"), "wrong_mode")
	wantProblem(t, r.Reveal(t0, p[0], 1), "wrong_phase")
	at := t0.Add(4 * time.Second)
	if err := r.Buzz(at, p[0], 1); err != nil {
		t.Fatal(err)
	}
	if err := r.Buzz(at.Add(45*time.Millisecond), p[1], 1); err != nil {
		t.Errorf("photo-finish buzz = %v", err)
	}
	wantProblem(t, r.Buzz(at.Add(50*time.Millisecond), p[1], 1), "already_answered")
	wantProblem(t, r.Buzz(at.Add(50*time.Millisecond), p[0], 1), "already_answered")
	wantProblem(t, r.Buzz(at.Add(PhotoWindow), p[2], 1), "already_buzzed")
	s := r.State(at)
	if s.Phase != PhaseBuzzed || s.Round.Buzz == nil || s.Round.Buzz.PlayerID != p[0] ||
		s.Round.Buzz.AnswerEndsAt != at.Add(AnswerTime).UnixMilli() || s.Round.Result != nil ||
		playerOf(s, p[0]).State != StateBuzzed {
		t.Fatalf("buzzed = %+v", s.Round)
	}
	wantProblem(t, r.Judge(at, p[0], 1, 2), "wrong_phase")

	// Another player alone cannot reveal; the buzzer plus one other does.
	if err := r.Reveal(at.Add(time.Second), p[1], 1); err != nil {
		t.Fatal(err)
	}
	if r.phase != PhaseBuzzed {
		t.Fatal("revealed without the buzzer")
	}
	_ = r.Reveal(at.Add(time.Second), p[1], 1) // a second press is the same vote
	if err := r.Reveal(at.Add(2*time.Second), p[0], 1); err != nil {
		t.Fatal(err)
	}
	s = r.State(at.Add(2 * time.Second))
	if s.Phase != PhaseJudging || s.Round.Result == nil || s.Round.Result.WinnerID != nil || len(s.Round.RevealVotes) != 2 {
		t.Fatalf("judging = %+v", s.Round)
	}
	if pf := s.Round.Result.PhotoFinish; len(pf) != 1 || pf[0] != (PhotoFinish{PlayerID: p[1], DeltaMs: 45}) {
		t.Errorf("photo finish = %+v", pf)
	}

	wantProblem(t, r.Judge(at, p[1], 1, 3), "invalid_points")
	_ = r.Judge(at.Add(3*time.Second), p[0], 1, 2)
	_ = r.Judge(at.Add(3*time.Second), p[1], 1, 1) // disagrees: wait for the rest
	if r.phase != PhaseJudging {
		t.Fatal("decided on a disagreement")
	}
	_ = r.Judge(at.Add(4*time.Second), p[2], 1, 2) // agrees with the buzzer
	s = r.State(at.Add(4 * time.Second))
	if s.Phase != PhaseResult || *s.Round.Result.WinnerID != p[0] || s.Round.Result.Points != 2 || playerOf(s, p[0]).Score != 2 {
		t.Errorf("judged = %+v", s.Round.Result)
	}
}

func TestBuzzer_tieBrokenByTheRoom(t *testing.T) {
	cases := []struct {
		name  string
		n     int
		votes []int // buzzer first
		want  int
	}{
		{"others overrule", 3, []int{2, 0, 0}, 0},
		{"median of three", 3, []int{2, 0, 1}, 1},
		{"two players, lower counts", 2, []int{2, 1}, 1},
		{"room of one", 1, []int{2}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newTestRoom(t, ModeBuzzer)
			p := join(t, r, c.n)
			start(t, r, 3)
			_ = r.Buzz(t0, p[0], 1)
			_ = r.Reveal(t0, p[0], 1)
			if c.n > 1 {
				_ = r.Reveal(t0, p[1], 1)
			}
			for i, v := range c.votes {
				if err := r.Judge(t0, p[i], 1, v); err != nil {
					t.Fatal(err)
				}
			}
			if r.phase != PhaseResult || r.round.result.Points != c.want {
				t.Errorf("phase %s points %d, want %d", r.phase, r.round.result.Points, c.want)
			}
			if c.want == 0 && r.round.result.WinnerID != nil {
				t.Error("winner with 0 points")
			}
		})
	}
}

func TestBuzzer_deadlines(t *testing.T) {
	r := newTestRoom(t, ModeBuzzer)
	p := join(t, r, 3)
	start(t, r, 4)
	// Nobody buzzes: reveal, nobody scores.
	r.Tick(t0.Add(RoundTime))
	if r.phase != PhaseResult || r.round.result.WinnerID != nil {
		t.Fatalf("no buzz = %s %+v", r.phase, r.round.result)
	}
	wantProblem(t, r.Buzz(t0.Add(RoundTime), p[0], 1), "wrong_round")

	// The answer window ends: the stage reveals; judging time ends: the votes so far decide.
	if _, err := next(r, t0.Add(RoundTime)); err != nil {
		t.Fatal(err)
	}
	at := t0.Add(RoundTime + time.Second)
	_ = r.Buzz(at, p[0], 2)
	r.Tick(at.Add(AnswerTime))
	if r.phase != PhaseJudging {
		t.Fatalf("after the answer window: %s", r.phase)
	}
	_ = r.Judge(at.Add(AnswerTime), p[1], 2, 1)
	r.Tick(at.Add(AnswerTime + JudgeTime))
	if r.phase != PhaseResult || r.round.result.Points != 1 {
		t.Errorf("judging timeout: %s %+v", r.phase, r.round.result)
	}

	// No vote at all in time: nothing.
	_, _ = next(r, at.Add(time.Minute))
	at = at.Add(time.Minute)
	_ = r.Buzz(at, p[0], 3)
	r.Tick(at.Add(AnswerTime + JudgeTime))
	if r.phase != PhaseResult || r.round.result.Points != 0 {
		t.Errorf("no votes: %s %+v", r.phase, r.round.result)
	}
}

func TestBuzzer_kickAndPause(t *testing.T) {
	r := newTestRoom(t, ModeBuzzer)
	p := join(t, r, 3)
	start(t, r, 4)
	_ = r.Buzz(t0, p[0], 1)
	_ = r.Reveal(t0, p[0], 1)
	// The one other who would have revealed leaves; another one does.
	if err := r.Kick(t0, host, p[2]); err != nil {
		t.Fatal(err)
	}
	if err := r.Pause(t0, host); err != nil {
		t.Fatal(err)
	}
	if err := r.Resume(t0.Add(time.Minute), host); err != nil {
		t.Fatal(err)
	}
	if got := r.round.buzz.answerEndsAt; !got.Equal(t0.Add(AnswerTime + time.Minute)) {
		t.Errorf("answerEndsAt after pause = %v", got)
	}
	_ = r.Reveal(t0.Add(time.Minute), p[1], 1)
	if r.phase != PhaseJudging {
		t.Fatalf("phase %s", r.phase)
	}
	_ = r.Judge(t0.Add(time.Minute), p[0], 1, 2)
	// p[1] is the only other; kicking them leaves the buzzer alone, whose vote stands.
	if err := r.Kick(t0.Add(time.Minute), host, p[1]); err != nil {
		t.Fatal(err)
	}
	if r.phase != PhaseResult || r.round.result.Points != 2 {
		t.Errorf("after kick in judging: %s %+v", r.phase, r.round.result)
	}

	// The buzzer leaves mid-answer: the round ends without points.
	r2 := newTestRoom(t, ModeBuzzer)
	q := join(t, r2, 2)
	start(t, r2, 3)
	_ = r2.Buzz(t0, q[0], 1)
	if err := r2.Kick(t0, host, q[0]); err != nil {
		t.Fatal(err)
	}
	if r2.phase != PhaseResult || r2.round.result == nil || r2.round.result.Points != 0 {
		t.Errorf("buzzer kicked: %s %+v", r2.phase, r2.round.result)
	}
}

func TestSubscribe_connectedAndEvents(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	p := join(t, r, 1)
	stage, stopStage, first, ok := r.Subscribe("", "")
	if !ok {
		t.Fatal("subscribe refused")
	}
	var s State
	if err := json.Unmarshal([]byte(first), &s); err != nil || s.Code != "K7Q4" || s.Players[0].Connected {
		t.Fatalf("first = %s, %v", first, err)
	}
	_, stopPad, _, _ := r.Subscribe(p[0], "")
	if ev := <-stage; !strings.Contains(ev, `"connected":true`) {
		t.Errorf("pad connect event = %s", ev)
	}
	stopPad()
	stopPad()
	if ev := <-stage; !strings.Contains(ev, `"connected":false`) {
		t.Errorf("pad leave event = %s", ev)
	}
	stopStage()
	r.close()
	if _, _, _, ok := r.Subscribe("", ""); ok {
		t.Error("subscribe on a closed room")
	}
}

func TestRoom_concurrentAnswers(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	p := join(t, r, MaxPlayers)
	start(t, r, 3)
	r.round.proposed = r.round.pick.song.Year + 3
	choice := right(r)
	_, stop, _, _ := r.Subscribe("", "")
	defer stop()
	var wg sync.WaitGroup
	for i, id := range p {
		wg.Go(func() {
			_ = r.Answer(t0.Add(time.Duration(i)*time.Millisecond), id, 1, choice)
			r.Tick(t0.Add(time.Second))
			_ = r.State(t0)
		})
	}
	wg.Wait()
	s := r.State(t0.Add(2 * time.Second))
	if s.Round.Result == nil || s.Round.Result.WinnerID == nil {
		t.Fatalf("no winner: %+v", s.Round)
	}
	total := 0
	for _, pl := range s.Players {
		total += pl.Score
	}
	if total != 1 || len(s.Round.Result.PhotoFinish) > MaxPlayers-1 {
		t.Errorf("total score %d, photo %d", total, len(s.Round.Result.PhotoFinish))
	}
}
