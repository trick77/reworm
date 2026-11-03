// Package party is party mode: rooms in memory, a kit/sse hub per room that pushes the whole RoomState on every
// change, and the rules of the two modes (plan D11, D14, D15). The server's clock decides every order; every time
// limit is a deadline the room checks when it is touched or ticked, so a test drives a whole game with a fake clock.
package party

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/trick77/ship/kit/logging"
	"github.com/trick77/ship/kit/sse"

	"github.com/trick77/reworm/internal/api"
	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/engine"
)

// Rule constants (D11, D14, D15).
const (
	// RoundTime is the answer window of a round.
	RoundTime = 15 * time.Second
	// AnswerTime is how long a buzzer has to say artist and title.
	AnswerTime = 10 * time.Second
	// JudgeTime is how long the room has to judge before the votes so far decide.
	JudgeTime = 20 * time.Second
	// PhotoWindow is how long after the decisive press later presses are still recorded.
	PhotoWindow = time.Second
	// MaxPlayers is a room's cap.
	MaxPlayers = 8
	// DefaultToWin is the points a game is played to.
	DefaultToWin = 10
)

// Errors a room answers with.
var (
	errNotPlayer   = api.Fail(http.StatusForbidden, "not_a_player", "join the room first")
	errNotHost     = api.Fail(http.StatusForbidden, "not_host", "only the host can do that")
	errFull        = api.Fail(http.StatusConflict, "room_full", "the room is full")
	errPhase       = api.Fail(http.StatusConflict, "wrong_phase", "not now")
	errMode        = api.Fail(http.StatusConflict, "wrong_mode", "not in this mode")
	errRound       = api.Fail(http.StatusConflict, "wrong_round", "that round is over")
	errAnswered    = api.Fail(http.StatusConflict, "already_answered", "you already answered this round")
	errNoPlayers   = api.Fail(http.StatusConflict, "no_players", "nobody has joined yet")
	errDeckEmpty   = api.Fail(http.StatusUnprocessableEntity, "deck_empty", "no playable song in this deck")
	errUnknownUser = api.Fail(http.StatusNotFound, "unknown_player", "no such player")
)

// Resolver is a song's preview URL (preview.Source); nil means none is configured.
type Resolver func(ctx context.Context, s deck.Song) (string, error)

// Loader is the play order of a deck: the songs to come, best first.
type Loader func(ctx context.Context, sel deck.Selector) ([]deck.Song, error)

type player struct {
	id        string
	profileID string
	name      string
	avatar    string
	score     int
	state     string
	streams   int
}

// pick is a song with its clip, ready to play.
type pick struct {
	song deck.Song
	clip engine.Clip
}

type buzz struct {
	playerID     string
	at           time.Time
	answerEndsAt time.Time
}

type round struct {
	n         int
	pick      pick
	proposed  int
	startedAt time.Time
	endsAt    time.Time
	buzz      *buzz
	reveal    []string
	judge     map[string]int
	judgeEnds time.Time
	result    *ResultState
	// decidedAt is the decisive press: the first correct answer, or the buzz; the photo window runs from it.
	decidedAt time.Time
	photo     []PhotoFinish
}

// Room is one party. Every method takes the room's lock, so the rules see one change at a time.
type Room struct {
	mu sync.Mutex

	code        string
	hostProfile string
	mode        string
	toWin       int
	settings    Settings
	phase       string
	pausedFrom  string
	pausedAt    time.Time
	players     []*player
	round       *round
	queue       []deck.Song
	next        *pick
	lastActive  time.Time

	rng     *rand.Rand
	resolve Resolver
	hub     *sse.Hub[string]
}

// Config is what a room is created or reconfigured with.
type Config struct {
	Mode       string              `json:"mode"`
	Deck       deck.Selector       `json:"deck"`
	ToWin      int                 `json:"toWin"`
	Clip       engine.ClipSettings `json:"clip"`
	Difficulty string              `json:"difficulty"`
}

// Normalize fills the defaults and checks the values.
func (c *Config) Normalize(thisYear int) error {
	if c.Mode == "" {
		c.Mode = ModeYears
	}
	if c.ToWin == 0 {
		c.ToWin = DefaultToWin
	}
	if c.Difficulty == "" {
		c.Difficulty = "medium"
	}
	bad := func(msg string) error { return api.Fail(http.StatusBadRequest, "invalid_settings", msg) }
	if c.Mode != ModeYears && c.Mode != ModeBuzzer {
		return bad(`mode is "years" or "buzzer"`)
	}
	if c.ToWin < 1 || c.ToWin > 50 {
		return bad("toWin is 1 to 50")
	}
	if _, ok := engine.Mixes[c.Difficulty]; !ok {
		return bad(`difficulty is "easy", "medium" or "hard"`)
	}
	if err := c.Clip.Normalize(); err != nil {
		return bad(err.Error())
	}
	if err := c.Deck.Normalize(thisYear); err != nil {
		return bad(err.Error())
	}
	return nil
}

func newRoom(code, hostProfile string, c Config, now time.Time, rng *rand.Rand, resolve Resolver) *Room {
	r := &Room{code: code, hostProfile: hostProfile, phase: PhaseLobby, lastActive: now, rng: rng, resolve: resolve,
		hub: sse.NewHub[string](sse.HubOptions{Max: 64})}
	r.configure(c)
	return r
}

func (r *Room) configure(c Config) {
	r.mode, r.toWin = c.Mode, c.ToWin
	r.settings = Settings{Deck: c.Deck, Clip: c.Clip, Difficulty: c.Difficulty}
}

// Code is the room's join code.
func (r *Room) Code() string { return r.code }

// --- snapshot and publishing -----------------------------------------------------------------------------------

func (r *Room) state() State {
	s := State{Code: r.code, Mode: r.mode, Phase: r.phase, HostID: r.hostProfile, ToWin: r.toWin, Settings: r.settings,
		Players: make([]PlayerState, len(r.players))}
	for i, p := range r.players {
		s.Players[i] = PlayerState{ID: p.id, Name: p.name, Avatar: p.avatar, Score: p.score, Connected: p.streams > 0, State: p.state}
	}
	if rd := r.round; rd != nil {
		rs := &RoundState{N: rd.n, StartedAt: api.Millis(rd.startedAt), EndsAt: api.Millis(rd.endsAt), Clip: rd.pick.clip,
			RevealVotes: slices.Clone(rd.reveal), JudgeVotes: map[string]int{}, Result: rd.result}
		if rs.RevealVotes == nil {
			rs.RevealVotes = []string{}
		}
		for k, v := range rd.judge {
			rs.JudgeVotes[k] = v
		}
		if r.mode == ModeYears {
			y := rd.proposed
			rs.ProposedYear = &y
		}
		if b := rd.buzz; b != nil {
			rs.Buzz = &BuzzState{PlayerID: b.playerID, At: api.Millis(b.at), AnswerEndsAt: api.Millis(b.answerEndsAt)}
		}
		s.Round = rs
	}
	if r.next != nil {
		c := r.next.clip
		s.NextClip = &c
	}
	return s
}

// State is a snapshot of the room after any deadline that passed by now.
func (r *Room) State(now time.Time) State {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.advance(now) {
		r.publish()
	}
	return r.state()
}

// publish sends the state to every stream, under the lock so streams see the changes in order.
func (r *Room) publish() {
	b, err := json.Marshal(r.state())
	if err != nil {
		slog.Error("room state", logging.Err(err)) // plain structs; a programmer error
		return
	}
	r.hub.PublishLatest(string(b))
}

// Subscribe opens a stream for the room, counting playerID ("" for the stage) as connected while it is open. It
// returns the current state as the stream's first event; ok is false when the room's stream cap is reached.
func (r *Room) Subscribe(playerID string, key string) (ch <-chan string, cancel func(), first string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, stop, ok := r.hub.Subscribe(key)
	if !ok {
		return nil, stop, "", false
	}
	p := r.player(playerID)
	if p != nil {
		p.streams++
		r.publish()
	}
	b, _ := json.Marshal(r.state()) // plain structs
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			stop()
			r.mu.Lock()
			defer r.mu.Unlock()
			if p != nil && p.streams > 0 {
				p.streams--
				r.publish()
			}
		})
	}, string(b), true
}

// close ends every stream of the room.
func (r *Room) close() { r.hub.Close() }

// --- players ---------------------------------------------------------------------------------------------------

func (r *Room) player(id string) *player {
	if id == "" {
		return nil
	}
	for _, p := range r.players {
		if p.id == id {
			return p
		}
	}
	return nil
}

// PlayerOf is the player a paired profile plays as, "" when it has not joined.
func (r *Room) PlayerOf(profileID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.players {
		if p.profileID != "" && p.profileID == profileID {
			return p.id
		}
	}
	return ""
}

// HostProfile is the profile that created the room.
func (r *Room) HostProfile() string { return r.hostProfile }

// Join adds a player, or returns the one rejoining: a guest by the player id its cookie carries, a paired profile by
// its profile. A player who joins mid-round sits that round out.
func (r *Room) Join(now time.Time, playerID, profileID, name, avatar string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.touch(now)
	for _, p := range r.players {
		if (playerID != "" && p.id == playerID) || (profileID != "" && p.profileID == profileID) {
			p.name, p.avatar = name, avatar
			r.publish()
			return p.id, nil
		}
	}
	if len(r.players) >= MaxPlayers {
		return "", errFull
	}
	p := &player{id: api.ID(), profileID: profileID, name: name, avatar: avatar, state: StateThinking}
	if r.phase != PhaseLobby {
		p.state = StateOut
	}
	r.players = append(r.players, p)
	r.publish()
	return p.id, nil
}

// touch notes activity, fires the deadlines that passed and returns the time the action counts at: never before
// the room's last action, so the order the lock admits requests in and their timestamps agree even when a request
// read the clock before another overtook it to the lock.
func (r *Room) touch(now time.Time) time.Time {
	if now.Before(r.lastActive) {
		now = r.lastActive
	}
	r.lastActive = now
	r.advance(now)
	return now
}

// idleSince is the last activity.
func (r *Room) idleSince() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastActive
}

// --- host actions ----------------------------------------------------------------------------------------------

func (r *Room) host(profileID string) error {
	if profileID == "" || profileID != r.hostProfile {
		return errNotHost
	}
	return nil
}

// Configure changes the settings, in the lobby only.
func (r *Room) Configure(now time.Time, profileID string, c Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.touch(now)
	if err := r.host(profileID); err != nil {
		return err
	}
	if r.phase != PhaseLobby {
		return errPhase
	}
	r.configure(c)
	r.publish()
	return nil
}

// Kick removes a player.
func (r *Room) Kick(now time.Time, profileID, playerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now = r.touch(now)
	if err := r.host(profileID); err != nil {
		return err
	}
	i := slices.IndexFunc(r.players, func(p *player) bool { return p.id == playerID })
	if i < 0 {
		return errUnknownUser
	}
	r.players = slices.Delete(r.players, i, i+1)
	if rd := r.round; rd != nil {
		rd.reveal = slices.DeleteFunc(rd.reveal, func(id string) bool { return id == playerID })
		delete(rd.judge, playerID)
		phase := r.phase
		if phase == PhasePaused {
			phase = r.pausedFrom
		}
		switch {
		case phase == PhasePlaying && r.mode == ModeYears && !r.anyThinking():
			r.finish(now, nil, 0)
		case (phase == PhaseBuzzed || phase == PhaseJudging) && rd.buzz.playerID == playerID:
			if rd.result == nil {
				rd.result = r.newResult()
			}
			r.finish(now, nil, 0)
		case phase == PhaseBuzzed:
			r.tryReveal(now)
		case phase == PhaseJudging:
			r.tryDecide(now, false)
		}
	}
	r.publish()
	return nil
}

// Start begins the game: loads the deck's play order and plays the first round. It returns the first song's id.
func (r *Room) Start(ctx context.Context, now time.Time, profileID string, load Loader) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now = r.touch(now)
	if err := r.host(profileID); err != nil {
		return "", err
	}
	if r.phase != PhaseLobby {
		return "", errPhase
	}
	if r.resolve == nil {
		return "", api.ErrNoPreviews
	}
	if len(r.players) == 0 {
		return "", errNoPlayers
	}
	q, err := load(ctx, r.settings.Deck)
	if err != nil {
		return "", err
	}
	r.queue = q
	first := r.take(ctx)
	if first == nil {
		return "", errDeckEmpty
	}
	r.next = r.take(ctx)
	for _, p := range r.players {
		p.score = 0
	}
	r.play(now, *first, 0)
	r.publish()
	return first.song.ID, nil
}

// Next plays the next round after a result; with no song left the game is over. It returns the song's id, "" when
// none.
func (r *Room) Next(ctx context.Context, now time.Time, profileID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now = r.touch(now)
	if err := r.host(profileID); err != nil {
		return "", err
	}
	if r.phase != PhaseResult {
		return "", errPhase
	}
	cur := r.next
	if cur == nil {
		r.phase = PhaseOver
		r.publish()
		return "", nil
	}
	r.next = r.take(ctx)
	r.play(now, *cur, r.round.n)
	r.publish()
	return cur.song.ID, nil
}

// take is the next song of the queue whose preview resolves, nil when none is left.
func (r *Room) take(ctx context.Context) *pick {
	for len(r.queue) > 0 {
		s := r.queue[0]
		r.queue = r.queue[1:]
		u, err := r.resolve(ctx, s)
		if err != nil {
			slog.WarnContext(ctx, "song skipped: no preview", "song", s.ID, logging.Err(err))
			continue
		}
		return &pick{song: s, clip: engine.Clip{PreviewURL: u, From: engine.ClipFrom(r.rng, r.settings.Clip), Length: r.settings.Clip.Length}}
	}
	return nil
}

// play starts round n+1 with p.
func (r *Room) play(now time.Time, p pick, n int) {
	rd := &round{n: n + 1, pick: p, startedAt: now, endsAt: now.Add(RoundTime), judge: map[string]int{}}
	if r.mode == ModeYears {
		rd.proposed, _ = engine.Propose(r.rng, p.song.Year, r.settings.Difficulty, 0, now.Year())
	}
	r.round = rd
	r.phase = PhasePlaying
	for _, pl := range r.players {
		pl.state = StateThinking
	}
}

// Pause freezes a running round; Resume moves its deadlines by the time it stood still.
func (r *Room) Pause(now time.Time, profileID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now = r.touch(now)
	if err := r.host(profileID); err != nil {
		return err
	}
	switch r.phase {
	case PhasePlaying, PhaseBuzzed, PhaseJudging:
	default:
		return errPhase
	}
	r.pausedFrom, r.pausedAt, r.phase = r.phase, now, PhasePaused
	r.publish()
	return nil
}

// Resume continues a paused round.
func (r *Room) Resume(now time.Time, profileID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now = r.touch(now)
	if err := r.host(profileID); err != nil {
		return err
	}
	if r.phase != PhasePaused {
		return errPhase
	}
	d := now.Sub(r.pausedAt)
	rd := r.round
	rd.endsAt = rd.endsAt.Add(d)
	rd.judgeEnds = rd.judgeEnds.Add(d)
	rd.decidedAt = rd.decidedAt.Add(d)
	if rd.buzz != nil {
		rd.buzz.answerEndsAt = rd.buzz.answerEndsAt.Add(d)
	}
	r.phase = r.pausedFrom
	r.publish()
	return nil
}

// Rematch takes a finished game back to the lobby with the same players.
func (r *Room) Rematch(now time.Time, profileID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.touch(now)
	if err := r.host(profileID); err != nil {
		return err
	}
	if r.phase != PhaseOver {
		return errPhase
	}
	r.phase, r.round, r.next, r.queue = PhaseLobby, nil, nil, nil
	for _, p := range r.players {
		p.score, p.state = 0, StateThinking
	}
	r.publish()
	return nil
}

// --- rounds ----------------------------------------------------------------------------------------------------

// advance fires every deadline that passed by now and reports whether anything changed.
func (r *Room) advance(now time.Time) bool {
	changed := false
	for {
		rd := r.round
		switch {
		case rd == nil:
			return changed
		case r.phase == PhasePlaying && !now.Before(rd.endsAt):
			// Nobody got it (years) or nobody buzzed (buzzer): reveal, no points.
			r.finish(rd.endsAt, nil, 0)
		case r.phase == PhaseBuzzed && !now.Before(rd.buzz.answerEndsAt):
			r.revealNow(rd.buzz.answerEndsAt)
		case r.phase == PhaseJudging && !now.Before(rd.judgeEnds):
			r.tryDecide(rd.judgeEnds, true)
		default:
			return changed
		}
		changed = true
	}
}

// Tick fires the deadlines that passed by now and publishes if that changed anything. A room busy with a request
// is skipped: it advances itself when that request is done, and the next tick comes soon.
func (r *Room) Tick(now time.Time) {
	if !r.mu.TryLock() {
		return
	}
	defer r.mu.Unlock()
	if r.advance(now) {
		r.publish()
	}
}

func (r *Room) newResult() *ResultState {
	return &ResultState{Song: r.round.pick.song.Reveal(), PhotoFinish: []PhotoFinish{}}
}

// finish ends the round: winner (nil: nobody) scores points; reaching toWin ends the game.
func (r *Room) finish(now time.Time, winner *player, points int) {
	rd := r.round
	if rd.result == nil {
		rd.result = r.newResult()
		rd.result.PhotoFinish = rd.photo
		if rd.result.PhotoFinish == nil {
			rd.result.PhotoFinish = []PhotoFinish{}
		}
	}
	rd.result.Points = points
	r.phase = PhaseResult
	if winner == nil || points == 0 {
		return
	}
	id := winner.id
	rd.result.WinnerID = &id
	if rd.decidedAt.IsZero() {
		rd.decidedAt = now
	}
	winner.score += points
	if winner.score >= r.toWin {
		r.phase = PhaseOver
	}
}

func (r *Room) anyThinking() bool {
	return slices.ContainsFunc(r.players, func(p *player) bool { return p.state == StateThinking })
}

// inPhotoWindow reports whether now is within the window after the round's decisive press.
func (r *Room) inPhotoWindow(now time.Time) bool {
	rd := r.round
	return !rd.decidedAt.IsZero() && now.Before(rd.decidedAt.Add(PhotoWindow))
}

func (r *Room) photo(p *player, now time.Time) {
	r.round.photo = append(r.round.photo, PhotoFinish{PlayerID: p.id, DeltaMs: now.Sub(r.round.decidedAt).Milliseconds()})
	if res := r.round.result; res != nil {
		res.PhotoFinish = r.round.photo
	}
}

// press is the checks every player action shares: the round, the player, the room not paused.
// It returns the player and the time the press counts at (touch).
func (r *Room) press(now time.Time, playerID string, n int, mode string) (*player, time.Time, error) {
	now = r.touch(now)
	p := r.player(playerID)
	switch {
	case p == nil:
		return nil, now, errNotPlayer
	case r.mode != mode:
		return nil, now, errMode
	case r.round == nil || r.round.n != n:
		return nil, now, errRound
	case r.phase == PhasePaused:
		return nil, now, errPhase
	}
	return p, now, nil
}

// Answer is a years-mode answer: "earlier", "exact" or "later" than the proposed year. The first correct one takes
// the round (+1 for a right "exact"); a wrong one puts that player out. Correct answers within the photo window
// after the winner are recorded, nothing else.
func (r *Room) Answer(now time.Time, playerID string, n int, choice string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, now, err := r.press(now, playerID, n, ModeYears)
	if err != nil {
		return err
	}
	if choice != "earlier" && choice != "exact" && choice != "later" {
		return api.Fail(http.StatusBadRequest, "invalid_choice", `choice is "earlier", "exact" or "later"`)
	}
	if p.state != StateThinking {
		return errAnswered
	}
	rd := r.round
	y, prop := rd.pick.song.Year, rd.proposed
	correct := (choice == "earlier" && y < prop) || (choice == "later" && y > prop) || (choice == "exact" && y == prop)
	switch {
	case r.phase == PhasePlaying:
	case (r.phase == PhaseResult || r.phase == PhaseOver) && r.inPhotoWindow(now):
		if correct {
			p.state = StateAnswered
			r.photo(p, now)
		} else {
			p.state = StateOut
		}
		r.publish()
		return nil
	default:
		return errRound
	}
	if !correct {
		p.state = StateOut
		if !r.anyThinking() {
			r.finish(now, nil, 0)
		}
		r.publish()
		return nil
	}
	p.state = StateAnswered
	rd.decidedAt = now
	points := 1
	if choice == "exact" {
		points++
	}
	r.finish(now, p, points)
	r.publish()
	return nil
}

// Buzz is a buzzer-mode buzz: the first pauses the round and gives that player AnswerTime to say it; later buzzes
// within the photo window are recorded, one buzz a round.
func (r *Room) Buzz(now time.Time, playerID string, n int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, now, err := r.press(now, playerID, n, ModeBuzzer)
	if err != nil {
		return err
	}
	rd := r.round
	switch {
	case p.state != StateThinking:
		return errAnswered
	case r.phase == PhasePlaying:
		rd.buzz = &buzz{playerID: p.id, at: now, answerEndsAt: now.Add(AnswerTime)}
		rd.decidedAt = now
		p.state = StateBuzzed
		r.phase = PhaseBuzzed
	case r.inPhotoWindow(now) && slices.ContainsFunc(rd.photo, func(f PhotoFinish) bool { return f.PlayerID == p.id }):
		return errAnswered
	case r.inPhotoWindow(now):
		r.photo(p, now)
	case rd.buzz == nil:
		return errRound
	default:
		return api.Fail(http.StatusConflict, "already_buzzed", "someone buzzed first")
	}
	r.publish()
	return nil
}

// Reveal is a vote to show the song: the buzzer and one other (or the buzzer alone in a room of one).
func (r *Room) Reveal(now time.Time, playerID string, n int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, now, err := r.press(now, playerID, n, ModeBuzzer)
	if err != nil {
		return err
	}
	if r.phase != PhaseBuzzed {
		return errPhase
	}
	if !slices.Contains(r.round.reveal, p.id) {
		r.round.reveal = append(r.round.reveal, p.id)
	}
	r.tryReveal(now)
	r.publish()
	return nil
}

func (r *Room) tryReveal(now time.Time) {
	rd := r.round
	buzzer := rd.buzz.playerID
	if !slices.Contains(rd.reveal, buzzer) {
		return
	}
	if len(r.players) == 1 || slices.ContainsFunc(rd.reveal, func(id string) bool { return id != buzzer }) {
		r.revealNow(now)
	}
}

// revealNow shows the song for judging; points follow the judges.
func (r *Room) revealNow(now time.Time) {
	rd := r.round
	rd.result = r.newResult()
	rd.result.PhotoFinish = rd.photo
	if rd.result.PhotoFinish == nil {
		rd.result.PhotoFinish = []PhotoFinish{}
	}
	rd.judgeEnds = now.Add(JudgeTime)
	r.phase = PhaseJudging
}

// Judge is a vote on the buzzer's answer: 2 artist and title, 1 one of them, 0 neither.
func (r *Room) Judge(now time.Time, playerID string, n, points int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, now, err := r.press(now, playerID, n, ModeBuzzer)
	if err != nil {
		return err
	}
	if points < 0 || points > 2 {
		return api.Fail(http.StatusBadRequest, "invalid_points", "points are 0, 1 or 2")
	}
	if r.phase != PhaseJudging {
		return errPhase
	}
	r.round.judge[p.id] = points
	r.tryDecide(now, false)
	r.publish()
	return nil
}

// tryDecide settles the judging: the buzzer and one other agreeing decide; failing that, once everyone voted (or
// the time is up, force) the lower median of the votes does, so the rest of the room breaks a tie. No votes at all
// score nothing.
func (r *Room) tryDecide(now time.Time, force bool) {
	rd := r.round
	buzzer := r.player(rd.buzz.playerID)
	if b, ok := rd.judge[rd.buzz.playerID]; ok {
		if len(r.players) == 1 {
			r.finish(now, buzzer, b)
			return
		}
		for id, v := range rd.judge {
			if id != rd.buzz.playerID && v == b {
				r.finish(now, buzzer, b)
				return
			}
		}
	}
	if !force && len(rd.judge) < len(r.players) {
		return
	}
	votes := make([]int, 0, len(rd.judge))
	for _, v := range rd.judge {
		votes = append(votes, v)
	}
	points := 0
	if len(votes) > 0 {
		slices.Sort(votes)
		points = votes[(len(votes)-1)/2]
	}
	r.finish(now, buzzer, points)
}
