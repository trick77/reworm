package party

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/trick77/ship/kit/auth"
	"github.com/trick77/ship/kit/httpx"
	"github.com/trick77/ship/kit/logging"
	"github.com/trick77/ship/kit/ratelimit"
	"github.com/trick77/ship/kit/sse"

	"github.com/trick77/reworm/internal/account"
	"github.com/trick77/reworm/internal/api"
	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/engine"
	"github.com/trick77/reworm/internal/preview"
)

// IdleTTL is how long a room lives without any action.
const IdleTTL = 2 * time.Hour

// GuestTTL is how long a guest's room cookie lasts: a long game night.
const GuestTTL = 12 * time.Hour

// MaxRooms bounds the rooms held at once.
const MaxRooms = 200

// historyWindow is how far back a host's played songs go to the back of the deck.
const historyWindow = 90 * 24 * time.Hour

// tickEvery is how often Run fires the rooms' deadlines: well inside the photo window.
const tickEvery = 100 * time.Millisecond

// heartbeat keeps an idle stream open through proxies. It is a named event ("ping"), not a comment: the client
// watches its stream for silence (a buffering proxy passes the 200, then nothing) and EventSource hides comments.
// The client's watchdog is twice this plus slack, so it bounds how long a stream that stalls mid-game freezes it.
const heartbeat = 10 * time.Second

// PollHold is how long a long poll (/wait) waits for a change before it answers the unchanged state: well under the
// 30-60 s after which proxies cut a quiet request.
const PollHold = 20 * time.Second

// pollSlack is how long after its last poll a long-polling player still counts as connected: the gap between one
// answer and the next request, and a slow phone's turn-around.
const pollSlack = 10 * time.Second

// Options configures New.
type Options struct {
	Accounts *account.Service
	Songs    *deck.Repo
	// Previews resolves clip URLs; nil answers 503 where a round needs one.
	Previews preview.Source
	// Key signs the guest cookies.
	Key    []byte
	Secure bool
	// BasePath is the path prefix the app is served under ("" at the root): the guest cookie is scoped below it.
	BasePath string
	// ClientKey is the rate-limit key of a request.
	ClientKey func(*http.Request) string
	// Now is the clock; nil is time.Now.
	Now func() time.Time
	// Rand makes each room's random source; nil seeds one at random.
	Rand func() *rand.Rand
}

// Service holds the rooms and serves their API.
type Service struct {
	o      Options
	guests *auth.Sessions
	// hosts is the host's room cookie: hosting needs no profile, the cookie names the host of one room.
	hosts   *auth.Sessions
	join    func(http.Handler) http.Handler
	actions func(http.Handler) http.Handler
	reads   func(http.Handler) http.Handler
	waits   func(http.Handler) http.Handler
	// misses meters unknown room codes per client: guessing codes costs, playing does not.
	misses *ratelimit.Limiter

	// countdown is each new room's lead-in before a round (Countdown; tests set 0).
	countdown time.Duration
	// pollHold is a long poll's wait (PollHold; tests shorten it).
	pollHold time.Duration

	mu    sync.Mutex
	rooms map[string]*Room
}

// New returns the service.
func New(o Options) (*Service, error) {
	g, err := auth.NewSessions(auth.SessionConfig{
		Name: "reworm_guest", Key: auth.DeriveKey(hex.EncodeToString(o.Key), "guest"), TTL: GuestTTL,
		Path: o.BasePath + "/api/rooms/", Secure: o.Secure,
	})
	if err != nil {
		return nil, err
	}
	h, err := auth.NewSessions(auth.SessionConfig{
		Name: "reworm_host", Key: auth.DeriveKey(hex.EncodeToString(o.Key), "host"), TTL: GuestTTL,
		Path: o.BasePath + "/api/rooms", Secure: o.Secure,
	})
	if err != nil {
		return nil, err
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Rand == nil {
		o.Rand = func() *rand.Rand {
			return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) //nolint:gosec // game randomness, not a secret
		}
	}
	if o.ClientKey == nil {
		o.ClientKey = ratelimit.ClientKey(0)
	}
	// A party shares one address behind the home router: limits per address allow eight phones at once.
	joinLim := ratelimit.New(ratelimit.Options{Rate: 30.0 / 60, Burst: 30, Now: o.Now})
	actLim := ratelimit.New(ratelimit.Options{Rate: 10, Burst: 60, Now: o.Now})
	readLim := ratelimit.New(ratelimit.Options{Rate: 5, Burst: 60, Now: o.Now})
	// Every publish answers every held long poll at once, and behind an office's NAT eight pads and the stage
	// poll from one address: reads' 5/s would refuse a busy round.
	waitLim := ratelimit.New(ratelimit.Options{Rate: 20, Burst: 120, Now: o.Now})
	missLim := ratelimit.New(ratelimit.Options{Rate: 10.0 / 60, Burst: 20, Now: o.Now})
	return &Service{o: o, guests: g, hosts: h, join: joinLim.Middleware(o.ClientKey), actions: actLim.Middleware(o.ClientKey),
		reads: readLim.Middleware(o.ClientKey), waits: waitLim.Middleware(o.ClientKey), misses: missLim,
		rooms: map[string]*Room{}, countdown: Countdown, pollHold: PollHold}, nil
}

// Run fires the rooms' deadlines and drops idle rooms until ctx ends.
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(tickEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Tick(s.o.Now())
		}
	}
}

// Tick fires every room's deadlines and drops the rooms idle for IdleTTL.
// Neither a busy room nor the registry is waited on: the rooms are collected under s.mu, then each is checked
// (its idle time is atomic, its tick skips a locked room).
func (s *Service) Tick(now time.Time) {
	s.mu.Lock()
	rooms := make([]*Room, 0, len(s.rooms))
	for _, r := range s.rooms {
		rooms = append(rooms, r)
	}
	s.mu.Unlock()
	for _, r := range rooms {
		if now.Sub(r.idleSince()) < IdleTTL {
			r.Tick(now)
			continue
		}
		s.mu.Lock()
		if s.rooms[r.Code()] == r {
			delete(s.rooms, r.Code())
		}
		s.mu.Unlock()
		r.close()
	}
}

// Close ends every room's streams (on shutdown, so draining does not wait for them).
func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rooms {
		r.close()
	}
}

func (s *Service) resolver() Resolver {
	if s.o.Previews == nil {
		return nil
	}
	return s.o.Previews.PreviewURL
}

// create adds a room for hostID under a fresh code; historyProfile ("" for none) orders its deck. One room per
// host: a new room ends the host's earlier one (its players see it gone). Answering the earlier one instead would
// trap a host who left a game mid-round in it; the host's own device resumes its room without asking here (the
// UI's hosting).
func (s *Service) create(hostID, historyProfile string, c Config) (*Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for code, r := range s.rooms {
		if r.HostID() == hostID {
			delete(s.rooms, code)
			r.close()
		}
	}
	if len(s.rooms) >= MaxRooms {
		return nil, api.Fail(http.StatusServiceUnavailable, "too_many_rooms", "too many rooms are open")
	}
	code := api.Code(4)
	for s.rooms[code] != nil {
		code = api.Code(4)
	}
	r := newRoom(code, hostID, c, s.countdown, s.o.Now, s.o.Rand(), s.resolver())
	r.historyProfile = historyProfile
	s.rooms[code] = r
	return r, nil
}

var errNoRoom = api.Fail(http.StatusNotFound, "unknown_room", "no such room")

func (s *Service) room(code string) (*Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[strings.ToUpper(code)]
	if r == nil {
		return nil, errNoRoom
	}
	return r, nil
}

// loader is the play order of a deck: shuffled, the profile's recently played songs last ("": no history).
func (s *Service) loader(profile string) Loader {
	return func(ctx context.Context, sel deck.Selector) ([]deck.Song, error) {
		songs, err := s.o.Songs.Select(ctx, sel)
		if err != nil {
			return nil, err
		}
		var played map[string]time.Time
		if profile != "" {
			if played, err = s.o.Songs.Played(ctx, profile, s.o.Now().Add(-historyWindow)); err != nil {
				return nil, err
			}
		}
		byID := make(map[string]deck.Song, len(songs))
		ids := make([]string, len(songs))
		for i, song := range songs {
			byID[song.ID], ids[i] = song, song.ID
		}
		rng := s.o.Rand()
		out := make([]deck.Song, 0, len(songs))
		for _, id := range engine.Order(rng, ids, played) {
			out = append(out, byID[id])
		}
		return out, nil
	}
}

// recordPlayed notes a round's song in the profile's history ("": none kept); a failure costs only the ordering
// of a later game.
func (s *Service) recordPlayed(ctx context.Context, profile, songID string) {
	if songID == "" || profile == "" {
		return
	}
	if err := s.o.Songs.RecordPlayed(ctx, profile, songID, s.o.Now()); err != nil {
		slog.WarnContext(ctx, "played history not written", "song", songID, logging.Err(err))
	}
}

// --- HTTP ------------------------------------------------------------------------------------------------------

// Mount adds the room routes.
func (s *Service) Mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/rooms", s.handleCreate)
	mux.Handle("GET /api/rooms/{code}", s.reads(s.withRoom(s.handleState)))
	mux.Handle("GET /api/rooms/{code}/events", s.reads(s.withRoom(s.handleEvents)))
	mux.Handle("GET /api/rooms/{code}/wait", s.waits(s.withRoom(s.handleWait)))
	mux.Handle("GET /api/rooms/{code}/me", s.reads(s.withRoom(s.handleMe)))
	mux.Handle("GET /api/rooms/{code}/host", s.reads(s.withRoom(s.handleHost)))
	mux.Handle("POST /api/rooms/{code}/join", s.join(s.withRoom(s.handleJoin)))
	for action, f := range map[string]func(hostCall) error{
		"start": s.start, "next": s.next, "pause": s.pause, "resume": s.resume, "rematch": s.rematch,
		"kick": s.kick, "settings": s.settings,
	} {
		mux.HandleFunc("POST /api/rooms/{code}/"+action, s.hostAction(f))
	}
	for action, f := range map[string]func(*Room, string, pressRequest) error{
		"answer": s.answer, "buzz": s.buzz, "reveal": s.reveal, "judge": s.judge,
	} {
		mux.Handle("POST /api/rooms/{code}/"+action, s.actions(s.playerAction(f)))
	}
}

// withRoom looks the room up and runs h. Every lookup reserves a token of the client's miss budget, refunded when
// the room exists: a guesser runs out after a few dozen unknown codes, a party never pays.
func (s *Service) withRoom(h func(http.ResponseWriter, *http.Request, *Room)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := s.o.ClientKey(r)
		if ok, wait := s.misses.Take(key, 1, 1); !ok {
			httpx.TooManyRequests(w, wait)
			return
		}
		room, err := s.room(r.PathValue("code"))
		if err != nil {
			api.WriteError(w, r, err)
			return
		}
		s.misses.Charge(key, -1)
		h(w, r, room)
	}
}

type meResponse struct {
	PlayerID string `json:"playerId"`
}

// handleMe is the player the request's guest or device session plays as in the room.
func (s *Service) handleMe(w http.ResponseWriter, r *http.Request, room *Room) {
	id := s.playerID(r, room)
	if err := room.Member(id); err != nil {
		api.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, meResponse{PlayerID: id})
}

type createResponse struct {
	Code string `json:"code"`
}

// handleCreate opens a room for anyone: the host is whoever holds the room cookie it answers with (the earlier
// host id when the browser hosted before, so its earlier room ends). A paired device adds its profile's history.
func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) {
	var c Config
	if !api.Decode(w, r, &c) {
		return
	}
	if err := c.Normalize(s.o.Now().Year()); err != nil {
		api.WriteError(w, r, err)
		return
	}
	hostID := api.ID()
	if hc, ok := s.hosts.Read(r); ok && hc.Subject != "" {
		hostID = hc.Subject
	}
	var history string
	id, err := s.o.Accounts.Identify(r)
	switch {
	case err == nil:
		history = id.ProfileID
	case !errors.Is(err, api.ErrUnauthorized):
		// a paired device must not lose its history to a failed lookup
		api.WriteError(w, r, err)
		return
	}
	room, err := s.create(hostID, history, c)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	if err := s.hosts.Issue(w, auth.Claims{Subject: hostID, Name: room.Code()}); err != nil {
		s.drop(room) // nobody could host it
		api.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, createResponse{Code: room.Code()})
}

// drop removes room and ends its streams.
func (s *Service) drop(room *Room) {
	s.mu.Lock()
	if s.rooms[room.Code()] == room {
		delete(s.rooms, room.Code())
	}
	s.mu.Unlock()
	room.close()
}

// hostOf is the host id the request acts as in room: its host cookie for this room, else "".
func (s *Service) hostOf(r *http.Request, room *Room) string {
	if c, ok := s.hosts.Read(r); ok && c.Name == room.Code() {
		return c.Subject
	}
	return ""
}

// isHost reports whether the request hosts room, and renews its host cookie when it does: a game night kept going
// past the cookie's 12 h keeps its host.
func (s *Service) isHost(w http.ResponseWriter, r *http.Request, room *Room) bool {
	id := s.hostOf(r, room)
	if id == "" || id != room.HostID() {
		return false
	}
	if err := s.hosts.Issue(w, auth.Claims{Subject: id, Name: room.Code()}); err != nil {
		slog.WarnContext(r.Context(), "host cookie not renewed", logging.Err(err))
	}
	return true
}

type hostResponse struct {
	Host bool `json:"host"`
}

// handleHost tells the UI whether this browser hosts the room: the server's word, not the UI's own storage.
func (s *Service) handleHost(w http.ResponseWriter, r *http.Request, room *Room) {
	httpx.WriteJSON(w, http.StatusOK, hostResponse{Host: s.isHost(w, r, room)})
}

func (s *Service) handleState(w http.ResponseWriter, _ *http.Request, room *Room) {
	httpx.WriteJSON(w, http.StatusOK, room.State(s.o.Now()))
}

// playerID is the player a request acts as in room: its guest cookie for this room, else its paired profile's
// player; "" for neither.
func (s *Service) playerID(r *http.Request, room *Room) string {
	if c, ok := s.guests.Read(r); ok && c.Name == room.Code() {
		return c.Subject
	}
	if id, err := s.o.Accounts.Identify(r); err == nil {
		return room.PlayerOf(id.ProfileID)
	}
	return ""
}

// streamSession is the session a stream counts for: a paired device's, else the guest cookie's for this room. A
// browser with neither gets a guest cookie now, before the stream opens, and joins later under that same id, so
// the stream it already has counts for it.
func (s *Service) streamSession(w http.ResponseWriter, r *http.Request, room *Room) (string, error) {
	if id, err := s.o.Accounts.Identify(r); err == nil {
		return deviceSession(id.DeviceID), nil
	}
	if c, ok := s.guests.Read(r); ok && c.Name == room.Code() {
		return c.Subject, nil
	}
	id := api.ID()
	return id, s.guests.Issue(w, auth.Claims{Subject: id, Name: room.Code()})
}

func (s *Service) handleEvents(w http.ResponseWriter, r *http.Request, room *Room) {
	session, err := s.streamSession(w, r, room)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	ch, cancel, first, ok := room.OpenStream(session, s.o.ClientKey(r), streamID(r, "sid"))
	if !ok {
		httpx.SetRetryAfter(w, 5*time.Second)
		api.WriteError(w, r, errTooManyStreams)
		return
	}
	defer cancel()
	sw, err := sse.Open(w)
	if err != nil {
		if errors.Is(err, sse.ErrNoFlush) {
			httpx.ServerError(w, r, err, "streaming unsupported")
		}
		return
	}
	defer sw.Close()
	_ = sw.HeartbeatEvent(heartbeat, "ping") // a constant, valid name
	if sw.Send("state", first) != nil {
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-sw.Done():
			return
		case v, ok := <-ch:
			if !ok || sw.Send("state", v) != nil {
				return
			}
		}
	}
}

// errTooManyStreams is a refused stream or long poll: the room's or the address's cap is reached.
var errTooManyStreams = api.Fail(http.StatusServiceUnavailable, "too_many_streams", "too many open streams")

// streamID is a page's id for its stream (?sid on events, ?cut on wait), "" when missing or too long to be one.
func streamID(r *http.Request, name string) string {
	if id := r.URL.Query().Get(name); len(id) <= maxStreamID {
		return id
	}
	return ""
}

// handleWait is the long poll a client falls back to when a proxy buffers its event stream: every answer is a whole
// response, which any proxy passes. ?after is the last seq the client has; an older room answers at once, else
// the next change does, or the unchanged state after pollHold. A missing after answers at once. ?cut names the
// stream the page gave up on, ended here.
func (s *Service) handleWait(w http.ResponseWriter, r *http.Request, room *Room) {
	session, err := s.streamSession(w, r, room)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	after, err := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	if err != nil {
		after = -1
	}
	ch, cancel, cur, ok := room.Wait(s.o.Now(), session, s.o.ClientKey(r), streamID(r, "cut"), after)
	if !ok {
		httpx.SetRetryAfter(w, 5*time.Second)
		api.WriteError(w, r, errTooManyStreams)
		return
	}
	defer cancel()
	if ch == nil {
		httpx.WriteJSON(w, http.StatusOK, json.RawMessage(cur))
		return
	}
	t := time.NewTimer(s.pollHold)
	defer t.Stop()
	select {
	case <-r.Context().Done():
		return
	case <-ch: // a change, or the room closed
	case <-t.C:
	}
	httpx.WriteJSON(w, http.StatusOK, room.State(s.o.Now()))
}

type joinRequest struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type joinResponse struct {
	PlayerID string `json:"playerId"`
}

func (s *Service) handleJoin(w http.ResponseWriter, r *http.Request, room *Room) {
	var req joinRequest
	if !api.DecodeOptional(w, r, &req) {
		return
	}
	var j Joiner
	if id, err := s.o.Accounts.Identify(r); err == nil {
		// A paired device plays as its profile.
		j.ProfileID, j.DeviceID, req.Name, req.Avatar = id.ProfileID, id.DeviceID, id.Name, id.Avatar
	}
	name, err := account.CheckName(req.Name)
	if err == nil {
		err = account.CheckAvatar(req.Avatar)
	}
	if c, ok := s.guests.Read(r); ok && c.Name == room.Code() {
		j.Session = c.Subject
	}
	j.Name, j.Avatar = name, req.Avatar
	var playerID string
	if err == nil {
		playerID, err = room.Join(s.o.Now(), j)
	}
	if err == nil {
		err = s.guests.Issue(w, auth.Claims{Subject: playerID, Name: room.Code()})
	}
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, joinResponse{PlayerID: playerID})
}

// hostCall is one host action: the room, the request, the caller's host id and the body (nil when empty).
type hostCall struct {
	room *Room
	ctx  context.Context
	host string
	body json.RawMessage
}

// into decodes the body into dst.
func (c hostCall) into(dst any) error {
	if len(c.body) == 0 || json.Unmarshal(c.body, dst) != nil {
		return api.Fail(http.StatusBadRequest, "malformed_request", "the body does not parse")
	}
	return nil
}

// hostAction runs f for the room's host and answers the new state.
func (s *Service) hostAction(f func(hostCall) error) http.HandlerFunc {
	return s.withRoom(func(w http.ResponseWriter, r *http.Request, room *Room) {
		var body json.RawMessage
		if !api.DecodeOptional(w, r, &body) {
			return
		}
		host := ""
		if s.isHost(w, r, room) {
			host = room.HostID()
		}
		if err := f(hostCall{room: room, ctx: r.Context(), host: host, body: body}); err != nil {
			api.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, room.State(s.o.Now()))
	})
}

func (s *Service) start(c hostCall) error {
	song, err := c.room.Start(c.ctx, c.host, s.loader(c.room.HistoryProfile()))
	s.recordPlayed(c.ctx, c.room.HistoryProfile(), song)
	return err
}

func (s *Service) next(c hostCall) error {
	song, err := c.room.Next(c.ctx, c.host)
	s.recordPlayed(c.ctx, c.room.HistoryProfile(), song)
	return err
}

func (s *Service) pause(c hostCall) error { return c.room.Pause(s.o.Now(), c.host) }

func (s *Service) resume(c hostCall) error { return c.room.Resume(s.o.Now(), c.host) }

func (s *Service) rematch(c hostCall) error { return c.room.Rematch(s.o.Now(), c.host) }

type kickRequest struct {
	PlayerID string `json:"playerId"`
}

func (s *Service) kick(c hostCall) error {
	var req kickRequest
	if err := c.into(&req); err != nil {
		return err
	}
	return c.room.Kick(s.o.Now(), c.host, req.PlayerID)
}

func (s *Service) settings(c hostCall) error {
	var cfg Config
	if err := c.into(&cfg); err != nil {
		return err
	}
	if err := cfg.Normalize(s.o.Now().Year()); err != nil {
		return err
	}
	return c.room.Configure(s.o.Now(), c.host, cfg)
}

type pressRequest struct {
	Round  int    `json:"round"`
	Choice string `json:"choice"`
	Points *int   `json:"points"`
}

type acceptedResponse struct {
	Accepted bool `json:"accepted"`
}

// playerAction decodes a press and runs f for the request's player.
func (s *Service) playerAction(f func(*Room, string, pressRequest) error) http.HandlerFunc {
	return s.withRoom(func(w http.ResponseWriter, r *http.Request, room *Room) {
		var req pressRequest
		if !api.Decode(w, r, &req) {
			return
		}
		if err := f(room, s.playerID(r, room), req); err != nil {
			api.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, acceptedResponse{Accepted: true})
	})
}

func (s *Service) answer(room *Room, player string, req pressRequest) error {
	return room.Answer(s.o.Now(), player, req.Round, req.Choice)
}

func (s *Service) buzz(room *Room, player string, req pressRequest) error {
	return room.Buzz(s.o.Now(), player, req.Round)
}

func (s *Service) reveal(room *Room, player string, req pressRequest) error {
	return room.Reveal(s.o.Now(), player, req.Round)
}

func (s *Service) judge(room *Room, player string, req pressRequest) error {
	if req.Points == nil {
		return api.Fail(http.StatusBadRequest, "invalid_points", "points are 0, 1 or 2")
	}
	return room.Judge(s.o.Now(), player, req.Round, *req.Points)
}
