package party

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
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

// heartbeat keeps an idle stream open through proxies.
const heartbeat = 25 * time.Second

// Options configures New.
type Options struct {
	Accounts *account.Service
	Songs    *deck.Repo
	// Previews resolves clip URLs; nil answers 503 where a round needs one.
	Previews preview.Source
	// Key signs the guest cookies.
	Key    []byte
	Secure bool
	// ClientKey is the rate-limit key of a request.
	ClientKey func(*http.Request) string
	// Now is the clock; nil is time.Now.
	Now func() time.Time
	// Rand makes each room's random source; nil seeds one at random.
	Rand func() *rand.Rand
}

// Service holds the rooms and serves their API.
type Service struct {
	o       Options
	guests  *auth.Sessions
	join    func(http.Handler) http.Handler
	actions func(http.Handler) http.Handler

	mu    sync.Mutex
	rooms map[string]*Room
}

// New returns the service.
func New(o Options) (*Service, error) {
	g, err := auth.NewSessions(auth.SessionConfig{
		Name: "reworm_guest", Key: auth.DeriveKey(hex.EncodeToString(o.Key), "guest"), TTL: GuestTTL,
		Path: "/api/rooms/", Secure: o.Secure,
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
	return &Service{o: o, guests: g, join: joinLim.Middleware(o.ClientKey), actions: actLim.Middleware(o.ClientKey),
		rooms: map[string]*Room{}}, nil
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
func (s *Service) Tick(now time.Time) {
	s.mu.Lock()
	rooms := make([]*Room, 0, len(s.rooms))
	for code, r := range s.rooms {
		if now.Sub(r.idleSince()) >= IdleTTL {
			delete(s.rooms, code)
			r.close()
			continue
		}
		rooms = append(rooms, r)
	}
	s.mu.Unlock()
	for _, r := range rooms {
		r.Tick(now)
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

// create adds a room for the host profile under a fresh code.
func (s *Service) create(hostProfile string, c Config) (*Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.rooms) >= MaxRooms {
		return nil, api.Fail(http.StatusServiceUnavailable, "too_many_rooms", "too many rooms are open")
	}
	code := api.Code(4)
	for s.rooms[code] != nil {
		code = api.Code(4)
	}
	r := newRoom(code, hostProfile, c, s.o.Now(), s.o.Rand(), s.resolver())
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

// loader is the play order of a deck for the host: shuffled, the host's recently played songs last.
func (s *Service) loader(hostProfile string) Loader {
	return func(ctx context.Context, sel deck.Selector) ([]deck.Song, error) {
		songs, err := s.o.Songs.Select(ctx, sel)
		if err != nil {
			return nil, err
		}
		played, err := s.o.Songs.Played(ctx, hostProfile, s.o.Now().Add(-historyWindow))
		if err != nil {
			return nil, err
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

// recordPlayed notes a round's song in the host's history; a failure costs only the ordering of a later game.
func (s *Service) recordPlayed(ctx context.Context, hostProfile, songID string) {
	if songID == "" {
		return
	}
	if err := s.o.Songs.RecordPlayed(ctx, hostProfile, songID, s.o.Now()); err != nil {
		slog.WarnContext(ctx, "played history not written", "song", songID, logging.Err(err))
	}
}

// --- HTTP ------------------------------------------------------------------------------------------------------

// Mount adds the room routes.
func (s *Service) Mount(mux *http.ServeMux) {
	acc := s.o.Accounts
	mux.HandleFunc("POST /api/rooms", acc.Require(s.handleCreate))
	mux.HandleFunc("GET /api/rooms/{code}", s.withRoom(s.handleState))
	mux.HandleFunc("GET /api/rooms/{code}/events", s.withRoom(s.handleEvents))
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

func (s *Service) withRoom(h func(http.ResponseWriter, *http.Request, *Room)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		room, err := s.room(r.PathValue("code"))
		if err != nil {
			api.WriteError(w, r, err)
			return
		}
		h(w, r, room)
	}
}

type createResponse struct {
	Code string `json:"code"`
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request, id account.Identity) {
	var c Config
	if !api.Decode(w, r, &c) {
		return
	}
	if err := c.Normalize(s.o.Now().Year()); err != nil {
		api.WriteError(w, r, err)
		return
	}
	room, err := s.create(id.ProfileID, c)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, createResponse{Code: room.Code()})
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

func (s *Service) handleEvents(w http.ResponseWriter, r *http.Request, room *Room) {
	ch, cancel, first, ok := room.Subscribe(s.playerID(r, room), s.o.ClientKey(r))
	if !ok {
		httpx.SetRetryAfter(w, 5*time.Second)
		api.WriteError(w, r, api.Fail(http.StatusServiceUnavailable, "too_many_streams", "too many open streams"))
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
	sw.Heartbeat(heartbeat)
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
	var profileID string
	if id, err := s.o.Accounts.Identify(r); err == nil {
		// A paired device plays as its profile.
		profileID, req.Name, req.Avatar = id.ProfileID, id.Name, id.Avatar
	}
	name, err := account.CheckName(req.Name)
	if err == nil {
		err = account.CheckAvatar(req.Avatar)
	}
	var prev string
	if c, ok := s.guests.Read(r); ok && c.Name == room.Code() {
		prev = c.Subject
	}
	var playerID string
	if err == nil {
		playerID, err = room.Join(s.o.Now(), prev, profileID, name, req.Avatar)
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

// hostCall is one host action: the room, the request, the caller's profile and the body (nil when empty).
type hostCall struct {
	room    *Room
	ctx     context.Context
	profile string
	body    json.RawMessage
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
		id, err := s.o.Accounts.Identify(r)
		if err == nil {
			err = f(hostCall{room: room, ctx: r.Context(), profile: id.ProfileID, body: body})
		}
		if err != nil {
			api.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, room.State(s.o.Now()))
	})
}

func (s *Service) start(c hostCall) error {
	song, err := c.room.Start(c.ctx, s.o.Now(), c.profile, s.loader(c.room.HostProfile()))
	s.recordPlayed(c.ctx, c.room.HostProfile(), song)
	return err
}

func (s *Service) next(c hostCall) error {
	song, err := c.room.Next(c.ctx, s.o.Now(), c.profile)
	s.recordPlayed(c.ctx, c.room.HostProfile(), song)
	return err
}

func (s *Service) pause(c hostCall) error { return c.room.Pause(s.o.Now(), c.profile) }

func (s *Service) resume(c hostCall) error { return c.room.Resume(s.o.Now(), c.profile) }

func (s *Service) rematch(c hostCall) error { return c.room.Rematch(s.o.Now(), c.profile) }

type kickRequest struct {
	PlayerID string `json:"playerId"`
}

func (s *Service) kick(c hostCall) error {
	var req kickRequest
	if err := c.into(&req); err != nil {
		return err
	}
	return c.room.Kick(s.o.Now(), c.profile, req.PlayerID)
}

func (s *Service) settings(c hostCall) error {
	var cfg Config
	if err := c.into(&cfg); err != nil {
		return err
	}
	if err := cfg.Normalize(s.o.Now().Year()); err != nil {
		return err
	}
	return c.room.Configure(s.o.Now(), c.profile, cfg)
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
