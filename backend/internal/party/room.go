// Package party is party mode: rooms in memory, a kit/sse hub per room that pushes the whole RoomState on every
// change, and the rules of the two modes (plan D11, D14, D15). The server's clock decides every order; every time
// limit is a deadline the room checks when it is touched or ticked, so a test drives a whole game with a fake clock.
package party

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/trick77/ship/kit/logging"
	"github.com/trick77/ship/kit/sse"

	"github.com/trick77/reworm/internal/api"
	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/engine"
	"github.com/trick77/reworm/internal/preview"
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

// Stream caps per room. A party shares one address behind the home router: eight pads, the stage and a reconnect
// or two must fit under one key, while one caller still cannot take every slot.
const (
	maxStreams       = 64
	maxStreamsPerKey = 12
)

// Errors a room answers with.
var (
	errNotInRoom   = api.Fail(http.StatusUnauthorized, "not_in_room", "join the room first")
	errKicked      = api.Fail(http.StatusForbidden, "kicked", "the host removed you from this room")
	errNotHost     = api.Fail(http.StatusForbidden, "not_host", "only the host can do that")
	errFull        = api.Fail(http.StatusConflict, "room_full", "the room is full")
	errPhase       = api.Fail(http.StatusConflict, "wrong_phase", "not now")
	errMode        = api.Fail(http.StatusConflict, "wrong_mode", "not in this mode")
	errRound       = api.Fail(http.StatusConflict, "wrong_round", "that round is over")
	errAnswered    = api.Fail(http.StatusConflict, "already_answered", "you already answered this round")
	errLate        = api.Fail(http.StatusConflict, "late", "you joined mid-round: you play from the next round")
	errNoPlayers   = api.Fail(http.StatusConflict, "no_players", "nobody has joined yet")
	errDeckEmpty   = api.Fail(http.StatusUnprocessableEntity, "deck_empty", "no playable song in this deck")
	errUnknownUser = api.Fail(http.StatusNotFound, "unknown_player", "no such player")
	errPreviews    = api.Fail(http.StatusServiceUnavailable, "previews_unavailable", "the preview source failed; try again")
)

// Resolver is a song's preview URL (preview.Source); nil means none is configured.
type Resolver func(ctx context.Context, s deck.Song) (string, error)

// Loader is the play order of a deck: the songs to come, best first.
type Loader func(ctx context.Context, sel deck.Selector) ([]deck.Song, error)

type player struct {
	id        string
	profileID string
	// device is the paired device a profile player joined from; its streams are the player's connection.
	device string
	name   string
	avatar string
	score  int
	state  string
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
	// paused is how long the round stood still before its decisive press.
	paused    time.Duration
	buzz      *buzz
	reveal    []string
	judge     map[string]int
	judgeEnds time.Time
	result    *ResultState
	// decidedAt is the decisive press: the first correct answer, or the buzz; the photo window runs from it.
	decidedAt time.Time
	// pressMs is the decisive press in ms of running round time (pauses left out).
	pressMs int64
	photo   []PhotoFinish
}

// Room is one party. Every rule runs under mu, so the rules see one change at a time; preview lookups run outside
// it, serialized by fetch, so a slow preview source never holds up the room or the ticker. Lock order: fetch, then
// mu; never fetch while holding mu.
type Room struct {
	mu    sync.Mutex
	fetch sync.Mutex

	code        string
	hostProfile string
	mode        string
	toWin       int
	settings    Settings
	phase       string
	pausedFrom  string
	pausedAt    time.Time
	players     []*player
	// banned are the player and profile ids the host kicked, for the room's lifetime.
	banned map[string]bool
	round  *round
	queue  []deck.Song
	next   *pick
	// gen changes whenever the queue is replaced: a lookup started on the old one is stale.
	gen        int
	seq        int64
	lastActive time.Time
	// streams counts the open streams per session (Subscribe).
	streams map[string]int
	// bg tracks background prefetches (tests wait on it).
	bg sync.WaitGroup
	// idle mirrors lastActive (unix ms) for the reaper, which must not wait for a busy room.
	idle atomic.Int64

	clock   func() time.Time
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

func newRoom(code, hostProfile string, c Config, clock func() time.Time, rng *rand.Rand, resolve Resolver) *Room {
	r := &Room{code: code, hostProfile: hostProfile, phase: PhaseLobby, banned: map[string]bool{}, streams: map[string]int{}, clock: clock,
		rng: rng, resolve: resolve, hub: sse.NewHub[string](sse.HubOptions{Max: maxStreams, MaxPerKey: maxStreamsPerKey})}
	r.lastActive = clock()
	r.idle.Store(r.lastActive.UnixMilli())
	r.configure(c)
	return r
}

// configure applies c; a Start still loading the old deck is discarded (gen).
func (r *Room) configure(c Config) {
	r.gen++
	r.mode, r.toWin = c.Mode, c.ToWin
	r.settings = Settings{Deck: c.Deck, Clip: c.Clip, Difficulty: c.Difficulty}
}

// Code is the room's join code.
func (r *Room) Code() string { return r.code }

// --- snapshot and publishing -----------------------------------------------------------------------------------

func millisOrNil(t time.Time) *int64 {
	if t.IsZero() {
		return nil
	}
	ms := api.Millis(t)
	return &ms
}

// clonePtr is a copy of *p, nil for nil.
func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// cloneResult is a deep copy: a snapshot is marshalled after the lock is gone, while the room goes on changing.
func cloneResult(res *ResultState) *ResultState {
	if res == nil {
		return nil
	}
	c := *res
	c.Song.Cover = clonePtr(res.Song.Cover)
	c.WinnerID, c.WinnerMs = clonePtr(res.WinnerID), clonePtr(res.WinnerMs)
	c.PhotoFinish = append([]PhotoFinish{}, res.PhotoFinish...)
	return &c
}

// connected reports whether a stream is open for p: on its guest session, or on the device it joined from.
func (r *Room) connected(p *player) bool {
	return r.streams[p.id] > 0 || (p.device != "" && r.streams[deviceSession(p.device)] > 0)
}

// deviceSession is the stream session of a paired device.
func deviceSession(deviceID string) string { return "device:" + deviceID }

// state is a snapshot sharing nothing with the room.
func (r *Room) state() State {
	set := r.settings
	set.Deck.Sets, set.Deck.Categories, set.Deck.Genres = slices.Clone(set.Deck.Sets), slices.Clone(set.Deck.Categories),
		slices.Clone(set.Deck.Genres)
	s := State{Code: r.code, Seq: r.seq, Now: api.Millis(r.clock()), Mode: r.mode, Phase: r.phase, HostID: r.hostProfile,
		ToWin: r.toWin, Settings: set, Players: make([]PlayerState, len(r.players))}
	for i, p := range r.players {
		s.Players[i] = PlayerState{ID: p.id, Name: p.name, Avatar: p.avatar, Score: p.score, Connected: r.connected(p), State: p.state}
	}
	if rd := r.round; rd != nil {
		rs := &RoundState{N: rd.n, StartedAt: api.Millis(rd.startedAt), EndsAt: api.Millis(rd.endsAt), Clip: rd.pick.clip,
			RevealVotes: slices.Clone(rd.reveal), JudgeVotes: map[string]int{}, JudgeEndsAt: millisOrNil(rd.judgeEnds),
			Result: cloneResult(rd.result)}
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

// publish numbers the change and sends the state to every stream, under the lock so streams see the changes in
// order; seq lets a client drop a snapshot older than an event it already has.
func (r *Room) publish() {
	r.seq++
	b, err := json.Marshal(r.state())
	if err != nil {
		slog.Error("room state", logging.Err(err)) // plain structs; a programmer error
		return
	}
	r.hub.PublishLatest(string(b))
}

// Subscribe opens a stream for the room on session: a guest session id (the player id it joins as) or a paired
// device's deviceSession. The stream counts as the player's connection whenever a player is on that session, also
// one who joins after the stream opened. It returns the current state as the stream's first event; ok is false
// when a stream cap is reached.
func (r *Room) Subscribe(session, key string) (ch <-chan string, cancel func(), first string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, stop, ok := r.hub.Subscribe(key)
	if !ok {
		return nil, stop, "", false
	}
	r.streams[session]++
	if r.onSession(session) {
		r.publish()
	}
	b, _ := json.Marshal(r.state()) // plain structs
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			stop()
			r.mu.Lock()
			defer r.mu.Unlock()
			if r.streams[session]--; r.streams[session] <= 0 {
				delete(r.streams, session)
			}
			if r.onSession(session) {
				r.publish()
			}
		})
	}, string(b), true
}

// onSession reports whether a player is on session.
func (r *Room) onSession(session string) bool {
	return session != "" && slices.ContainsFunc(r.players, func(p *player) bool {
		return p.id == session || (p.device != "" && deviceSession(p.device) == session)
	})
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

// Member checks that playerID is in the room: errNotInRoom when it is not (no session, or a room it never joined),
// errKicked when the host removed it.
func (r *Room) Member(playerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.member(playerID)
}

func (r *Room) member(playerID string) error {
	switch {
	case playerID != "" && r.banned[playerID]:
		return errKicked
	case r.player(playerID) == nil:
		return errNotInRoom
	}
	return nil
}

// HostProfile is the profile that created the room.
func (r *Room) HostProfile() string { return r.hostProfile }

// Joiner is who joins: a guest by its session id (the player id its cookie carries), or a paired profile from a
// device.
type Joiner struct {
	Session   string
	ProfileID string
	DeviceID  string
	Name      string
	Avatar    string
}

// Join adds a player, or returns the one rejoining: a guest by its session, a paired profile by its profile. A new
// guest plays under its session id, so a stream it opened before joining counts for it at once. A player who joins
// mid-round is "late" and plays from the next round. The kicked stay out.
func (r *Room) Join(now time.Time, j Joiner) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.touch(now)
	if (j.Session != "" && r.banned[j.Session]) || (j.ProfileID != "" && r.banned[j.ProfileID]) {
		return "", errKicked
	}
	for _, p := range r.players {
		if (j.ProfileID != "" && p.profileID == j.ProfileID) || (j.ProfileID == "" && j.Session != "" && p.id == j.Session) {
			p.name, p.avatar = j.Name, j.Avatar
			if j.DeviceID != "" {
				p.device = j.DeviceID
			}
			r.publish()
			return p.id, nil
		}
	}
	if len(r.players) >= MaxPlayers {
		return "", errFull
	}
	id := j.Session
	if id == "" || j.ProfileID != "" || r.player(id) != nil {
		id = api.ID()
	}
	p := &player{id: id, profileID: j.ProfileID, device: j.DeviceID, name: j.Name, avatar: j.Avatar, state: StateThinking}
	if r.phase != PhaseLobby {
		p.state = StateLate
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
	r.idle.Store(now.UnixMilli())
	r.advance(now)
	return now
}

// idleSince is the last activity, read without the lock.
func (r *Room) idleSince() time.Time { return time.UnixMilli(r.idle.Load()) }

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

// Kick removes a player for the rest of the room's life (player and profile). A running round is settled at once;
// a paused one when it resumes.
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
	r.banned[playerID] = true
	if pid := r.players[i].profileID; pid != "" {
		r.banned[pid] = true
	}
	r.players = slices.Delete(r.players, i, i+1)
	if rd := r.round; rd != nil {
		rd.reveal = slices.DeleteFunc(rd.reveal, func(id string) bool { return id == playerID })
		delete(rd.judge, playerID)
		if r.phase != PhasePaused {
			r.settle(now)
		}
	}
	r.publish()
	return nil
}

// settle re-runs the round's end conditions after the players changed: years with nobody left thinking ends; a
// buzzer who left ends the round without points; otherwise reveal and judging are counted again.
func (r *Room) settle(now time.Time) {
	rd := r.round
	switch r.phase {
	case PhasePlaying:
		if r.mode == ModeYears && !r.anyThinking() {
			r.finish(now, nil, 0)
		}
	case PhaseBuzzed, PhaseJudging:
		switch {
		case r.player(rd.buzz.playerID) == nil:
			r.finish(now, nil, 0)
		case r.phase == PhaseBuzzed:
			r.tryReveal(now)
		default:
			r.tryDecide(now, false)
		}
	}
}

func (r *Room) canStart(profileID string) error {
	switch {
	case r.host(profileID) != nil:
		return errNotHost
	case r.phase != PhaseLobby:
		return errPhase
	case r.resolve == nil:
		return api.ErrNoPreviews
	case len(r.players) == 0:
		return errNoPlayers
	}
	return nil
}

// Start begins the game: loads the deck's play order, resolves the first song and plays it, then prefetches the
// next clip. The deck query and every preview lookup run outside the room lock. It returns the first song's id.
func (r *Room) Start(ctx context.Context, profileID string, load Loader) (string, error) {
	r.mu.Lock()
	r.touch(r.clock())
	err := r.canStart(profileID)
	sel, gen := r.settings.Deck, r.gen
	r.mu.Unlock()
	if err != nil {
		return "", err
	}
	q, err := load(ctx, sel)
	if err != nil {
		return "", err
	}
	r.mu.Lock()
	if err := r.canStart(profileID); err != nil || r.gen != gen {
		r.mu.Unlock()
		return "", errPhase
	}
	r.gen++
	r.queue, r.next, gen = q, nil, r.gen
	r.mu.Unlock()
	if err := r.prefetch(ctx); err != nil {
		return "", err
	}
	r.mu.Lock()
	now := r.touch(r.clock())
	// The settings changed meanwhile (configure bumps gen): this queue is for the old deck.
	if err := r.canStart(profileID); err != nil || r.gen != gen {
		r.mu.Unlock()
		return "", errPhase
	}
	first := r.next
	if first == nil {
		r.mu.Unlock()
		return "", errDeckEmpty
	}
	r.next = nil
	for _, p := range r.players {
		p.score = 0
	}
	r.play(now, *first, 0)
	r.publish()
	r.mu.Unlock()
	r.prefetchNext(ctx)
	return first.song.ID, nil
}

// Next plays the next round after a result; with no song left the game is over. It returns the song's id, "" when
// none.
func (r *Room) Next(ctx context.Context, profileID string) (string, error) {
	r.mu.Lock()
	r.touch(r.clock())
	err := r.host(profileID)
	if err == nil && r.phase != PhaseResult {
		err = errPhase
	}
	r.mu.Unlock()
	if err != nil {
		return "", err
	}
	// Usually prefetched during the last round already; else (it failed then) now.
	if err := r.prefetch(ctx); err != nil {
		return "", err
	}
	r.mu.Lock()
	now := r.touch(r.clock())
	if r.phase != PhaseResult {
		r.mu.Unlock()
		return "", errPhase
	}
	cur := r.next
	if cur == nil {
		r.phase = PhaseOver
		r.publish()
		r.mu.Unlock()
		return "", nil
	}
	r.next = nil
	r.play(now, *cur, r.round.n)
	r.publish()
	r.mu.Unlock()
	r.prefetchNext(ctx)
	return cur.song.ID, nil
}

// prefetchTimeout bounds a background prefetch.
const prefetchTimeout = 30 * time.Second

// prefetchNext resolves the next clip for the stage in the background, so the host's Start or Next answers at once;
// it outlives the request. A failure only leaves nextClip empty until Next tries again; a lookup for a queue that
// was replaced meanwhile is dropped (prefetch's gen check).
func (r *Room) prefetchNext(ctx context.Context) {
	r.bg.Add(1)
	go func() {
		defer r.bg.Done()
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), prefetchTimeout)
		defer cancel()
		if err := r.prefetch(ctx); err != nil {
			slog.WarnContext(ctx, "next clip not prefetched", "room", r.code, logging.Err(err))
		}
	}()
}

// prefetch resolves the head of the queue into r.next, outside the room lock. A song without a preview
// (preview.ErrNone) leaves the queue; any other failure (a cancelled request, the source down) leaves the queue as
// it is and answers errPreviews.
func (r *Room) prefetch(ctx context.Context) error {
	r.fetch.Lock()
	defer r.fetch.Unlock()
	for {
		r.mu.Lock()
		if r.next != nil || len(r.queue) == 0 {
			r.mu.Unlock()
			return nil
		}
		s, gen := r.queue[0], r.gen
		r.mu.Unlock()
		u, err := r.resolve(ctx, s)
		r.mu.Lock()
		switch {
		case gen != r.gen:
			// The queue was replaced meanwhile: look again.
		case errors.Is(err, preview.ErrNone):
			slog.WarnContext(ctx, "song skipped: no preview", "song", s.ID)
			r.queue = r.queue[1:]
		case err != nil:
			r.mu.Unlock()
			slog.WarnContext(ctx, "preview lookup failed", "song", s.ID, logging.Err(err))
			return errPreviews
		default:
			r.queue = r.queue[1:]
			r.next = &pick{song: s, clip: engine.Clip{PreviewURL: u, From: engine.ClipFrom(r.rng, r.settings.Clip),
				Length: r.settings.Clip.Length}}
			if r.round != nil {
				r.publish() // the stage prefetches nextClip
			}
		}
		r.mu.Unlock()
	}
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

// Resume continues a paused round and settles what changed while it stood still (a kick).
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
	if !rd.judgeEnds.IsZero() {
		rd.judgeEnds = rd.judgeEnds.Add(d)
	}
	if rd.decidedAt.IsZero() {
		rd.paused += d
	} else {
		rd.decidedAt = rd.decidedAt.Add(d)
	}
	if rd.buzz != nil {
		rd.buzz.answerEndsAt = rd.buzz.answerEndsAt.Add(d)
	}
	r.phase = r.pausedFrom
	r.settle(now)
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
	r.gen++
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
	rd := r.round
	res := &ResultState{Song: rd.pick.song.Reveal(), PhotoFinish: rd.photo}
	if res.PhotoFinish == nil {
		res.PhotoFinish = []PhotoFinish{}
	}
	return res
}

// finish ends the round: winner (nil: nobody) scores points; reaching toWin ends the game.
func (r *Room) finish(now time.Time, winner *player, points int) {
	rd := r.round
	if rd.result == nil {
		rd.result = r.newResult()
	}
	rd.result.Points = points
	r.phase = PhaseResult
	if winner == nil || points == 0 {
		return
	}
	id, ms := winner.id, rd.pressMs
	rd.result.WinnerID, rd.result.WinnerMs = &id, &ms
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

// decide marks now as the round's decisive press.
func (r *Room) decide(now time.Time) {
	rd := r.round
	rd.decidedAt = now
	rd.pressMs = (now.Sub(rd.startedAt) - rd.paused).Milliseconds()
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

// press is the checks every player action shares: the player, the round, the room not paused. It returns the
// player and the time the press counts at (touch).
func (r *Room) press(now time.Time, playerID string, n int, mode string) (*player, time.Time, error) {
	now = r.touch(now)
	if err := r.member(playerID); err != nil {
		return nil, now, err
	}
	p := r.player(playerID)
	switch {
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
	rd := r.round
	correct, valid := engine.Correct(choice, rd.pick.song.Year, rd.proposed)
	if !valid {
		return api.Fail(http.StatusBadRequest, "invalid_choice", `choice is "earlier", "exact" or "later"`)
	}
	switch p.state {
	case StateThinking:
	case StateLate:
		return errLate
	default:
		return errAnswered
	}
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
	r.decide(now)
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
	case p.state == StateLate:
		return errLate
	case p.state != StateThinking:
		return errAnswered
	case r.phase == PhasePlaying:
		rd.buzz = &buzz{playerID: p.id, at: now, answerEndsAt: now.Add(AnswerTime)}
		r.decide(now)
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
