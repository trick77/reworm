package party

import (
	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/engine"
)

// Modes.
const (
	ModeYears  = "years"
	ModeBuzzer = "buzzer"
)

// Phases.
const (
	PhaseLobby = "lobby"
	// PhaseCountdown is the 3, 2, 1 before a round: the clip and the answer window start at round.startedAt.
	PhaseCountdown = "countdown"
	PhasePlaying   = "playing"
	PhaseBuzzed    = "buzzed"
	PhaseJudging   = "judging"
	PhaseResult    = "result"
	PhasePaused    = "paused"
	PhaseOver      = "over"
)

// Player states.
const (
	StateThinking = "thinking"
	StateOut      = "out"
	StateBuzzed   = "buzzed"
	StateAnswered = "answered"
	// StateLate is a player who joined mid-round: plays from the next round.
	StateLate = "late"
)

// Settings is what the host picks in the lobby.
type Settings struct {
	Deck       deck.Selector       `json:"deck"`
	Clip       engine.ClipSettings `json:"clip"`
	Difficulty string              `json:"difficulty"`
}

// State is RoomState of docs/api.md: the whole room, sent on every change. It never holds the song before the
// round's result.
type State struct {
	Code string `json:"code"`
	// Seq grows by one with every change of the room: a client keeps the state with the highest.
	Seq int64 `json:"seq"`
	// Now is the server clock (unix ms) when the state was written: clients derive their offset for countdowns.
	Now   int64  `json:"now"`
	Mode  string `json:"mode"`
	Phase string `json:"phase"`
	// PausedFrom is the phase a pause froze (playing, buzzed or judging), set only while paused: screens keep
	// drawing that phase. Guessing it from what a screen last drew failed when two updates arrived together.
	PausedFrom string        `json:"pausedFrom,omitempty"`
	HostID     string        `json:"hostId"`
	ToWin      int           `json:"toWin"`
	Settings   Settings      `json:"settings"`
	Players    []PlayerState `json:"players"`
	Round      *RoundState   `json:"round"`
	NextClip   *engine.Clip  `json:"nextClip"`
}

// PlayerState is one player in State.
type PlayerState struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Avatar    string `json:"avatar"`
	Score     int    `json:"score"`
	Connected bool   `json:"connected"`
	State     string `json:"state"`
}

// RoundState is the running or last round.
type RoundState struct {
	N int `json:"n"`
	// StartedAt is when the clip and the answer window start: still ahead while the countdown runs.
	StartedAt    int64          `json:"startedAt"`
	EndsAt       int64          `json:"endsAt"`
	ProposedYear *int           `json:"proposedYear,omitempty"`
	Clip         engine.Clip    `json:"clip"`
	Buzz         *BuzzState     `json:"buzz"`
	RevealVotes  []string       `json:"revealVotes"`
	JudgeVotes   map[string]int `json:"judgeVotes"`
	// JudgeEndsAt is when the votes so far decide (buzzer, from the reveal on); null before.
	JudgeEndsAt *int64       `json:"judgeEndsAt"`
	Result      *ResultState `json:"result"`
}

// BuzzState is the buzz that paused the round.
type BuzzState struct {
	PlayerID     string `json:"playerId"`
	At           int64  `json:"at"`
	AnswerEndsAt int64  `json:"answerEndsAt"`
}

// ResultState is the reveal: the song, who took the round and how close the others came.
type ResultState struct {
	Song     deck.Revealed `json:"song"`
	WinnerID *string       `json:"winnerId"`
	// WinnerMs is the winning press (years: the answer, buzzer: the buzz) in ms of round time, pauses left out.
	WinnerMs    *int64        `json:"winnerMs"`
	Points      int           `json:"points"`
	PhotoFinish []PhotoFinish `json:"photoFinish"`
}

// PhotoFinish is a press that came in within the window after the decisive one (D15).
type PhotoFinish struct {
	PlayerID string `json:"playerId"`
	DeltaMs  int64  `json:"deltaMs"`
}
