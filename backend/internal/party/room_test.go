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
	return newRoom("K7Q4", host, c, t0, rand.New(rand.NewPCG(1, 2)), f.PreviewURL)
}

// join adds n players and returns their ids.
func join(t *testing.T, r *Room, n int) []string {
	t.Helper()
	ids := make([]string, n)
	for i := range ids {
		id, err := r.Join(t0, "", "", fmt.Sprintf("P%d", i), "mic")
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = id
	}
	return ids
}

func start(t *testing.T, r *Room, n int) {
	t.Helper()
	if _, err := r.Start(context.Background(), t0, host, loaderOf(songs(n))); err != nil {
		t.Fatal(err)
	}
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
	other, _ := r.Join(t0, "", "", "Late", "worm")
	wantProblem(t, r.Answer(t0.Add(3100*time.Millisecond), other, 1, right(r)), "already_answered")
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
	if _, err := r.Next(context.Background(), t0.Add(2*time.Second), host); err != nil {
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

func TestYears_inputChecks(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	p := join(t, r, 1)
	wantProblem(t, r.Answer(t0, p[0], 1, "later"), "wrong_round")
	start(t, r, 3)
	wantProblem(t, r.Answer(t0, "stranger", 1, "later"), "not_a_player")
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
	wantProblem(t, r.Pause(now, host), "wrong_phase")
	now = now.Add(time.Minute)
	if err := r.Resume(now, host); err != nil {
		t.Fatal(err)
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
		if _, err := r.Next(context.Background(), now, host); err != nil {
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
	wantProblem(t, func() error { _, err := r.Next(context.Background(), now, host); return err }(), "wrong_phase")
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
	r := newRoom("K7Q4", host, c, t0, rand.New(rand.NewPCG(1, 2)), f.PreviewURL)
	p := join(t, r, 1)
	if _, err := r.Start(context.Background(), t0, host, loaderOf(songs(3))); err != nil {
		t.Fatal(err)
	}
	if r.next == nil || r.next.song.ID != "s2" {
		t.Fatalf("next = %+v, want s2 (s1 has no preview)", r.next)
	}
	_ = r.Answer(t0, p[0], 1, wrong(r))
	id, err := r.Next(context.Background(), t0, host)
	if err != nil || id != "s2" || r.next != nil {
		t.Fatalf("Next = %q, %v; next %+v", id, err, r.next)
	}
	if s := r.State(t0); s.NextClip != nil {
		t.Error("nextClip with nothing left")
	}
	_ = r.Answer(t0, p[0], 2, wrong(r))
	id, err = r.Next(context.Background(), t0, host)
	if err != nil || id != "" || r.phase != PhaseOver {
		t.Errorf("deck done: %q, %v, phase %s", id, err, r.phase)
	}
}

func TestStart_refusals(t *testing.T) {
	ctx := context.Background()
	r := newTestRoom(t, ModeYears)
	_, err := r.Start(ctx, t0, host, loaderOf(songs(2)))
	wantProblem(t, err, "no_players")
	join(t, r, 1)
	_, err = r.Start(ctx, t0, "x", loaderOf(songs(2)))
	wantProblem(t, err, "not_host")
	_, err = r.Start(ctx, t0, host, loaderOf(nil))
	wantProblem(t, err, "deck_empty")
	if _, err := r.Start(ctx, t0, host, func(context.Context, deck.Selector) ([]deck.Song, error) {
		return nil, errors.New("db down")
	}); err == nil || errors.As(err, new(*api.Problem)) {
		t.Errorf("load failure = %v", err)
	}
	start(t, r, 2)
	_, err = r.Start(ctx, t0, host, loaderOf(songs(2)))
	wantProblem(t, err, "wrong_phase")

	none := newRoom("ABCD", host, Config{Mode: ModeYears}, t0, rand.New(rand.NewPCG(1, 2)), nil)
	join(t, none, 1)
	_, err = none.Start(ctx, t0, host, loaderOf(songs(2)))
	wantProblem(t, err, "previews_unavailable")
}

func TestJoin(t *testing.T) {
	r := newTestRoom(t, ModeYears)
	p := join(t, r, MaxPlayers-1)
	pro, err := r.Join(t0, "", "profile-1", "Jan", "vinyl")
	if err != nil {
		t.Fatal(err)
	}
	if again, err := r.Join(t0, "", "profile-1", "Jan B.", "keys"); err != nil || again != pro {
		t.Errorf("profile rejoin = %q, %v", again, err)
	}
	if r.PlayerOf("profile-1") != pro || r.PlayerOf("nobody") != "" {
		t.Error("PlayerOf")
	}
	if again, err := r.Join(t0, p[0], "", "P0", "mic"); err != nil || again != p[0] {
		t.Errorf("guest rejoin = %q, %v", again, err)
	}
	_, err = r.Join(t0, "", "", "Ninth", "mic")
	wantProblem(t, err, "room_full")

	// Late join mid-round: out for this round, in from the next.
	r2 := newTestRoom(t, ModeYears)
	q := join(t, r2, 1)
	start(t, r2, 3)
	late, _ := r2.Join(t0, "", "", "Late", "worm")
	wantProblem(t, r2.Answer(t0, late, 1, right(r2)), "already_answered")
	_ = r2.Answer(t0, q[0], 1, wrong(r2))
	if _, err := r2.Next(context.Background(), t0, host); err != nil {
		t.Fatal(err)
	}
	if err := r2.Answer(t0, late, 2, right(r2)); err != nil {
		t.Errorf("late joiner in the next round = %v", err)
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
	if _, err := r.Next(context.Background(), t0.Add(RoundTime), host); err != nil {
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
	_, _ = r.Next(context.Background(), at.Add(time.Minute), host)
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
