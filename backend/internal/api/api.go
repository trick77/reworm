// Package api holds what every JSON handler shares: the error body of docs/api.md, request decoding, random ids
// and the server clock.
package api

import (
	"crypto/rand"
	"errors"
	"net/http"
	"time"

	"github.com/trick77/ship/kit/httpx"
)

// MaxBody bounds every JSON request body: the largest is a deck selector with a few dozen ids.
const MaxBody = 16 << 10

// Problem is an error a handler answers with its own status and code instead of a 500.
type Problem struct {
	Status  int
	Code    string
	Message string
}

func (p *Problem) Error() string { return p.Code + ": " + p.Message }

// Fail is a Problem.
func Fail(status int, code, message string) *Problem {
	return &Problem{Status: status, Code: code, Message: message}
}

// Common problems.
var (
	ErrUnauthorized = Fail(http.StatusUnauthorized, "unauthorized", "this device is not paired")
	ErrForbidden    = Fail(http.StatusForbidden, "forbidden", "not allowed")
	ErrNotFound     = Fail(http.StatusNotFound, "not_found", "not found")
	// ErrNoPreviews answers an endpoint that needs a clip while no preview source is configured.
	ErrNoPreviews = Fail(http.StatusServiceUnavailable, "previews_unavailable", "no preview source is configured")
)

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// WriteError answers err: a Problem with its status and code, anything else as a logged 500.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var p *Problem
	if errors.As(err, &p) {
		if p.Status == http.StatusUnauthorized {
			w.Header().Set("Cache-Control", "no-store")
		}
		httpx.WriteJSON(w, p.Status, errorBody{Error: p.Code, Message: p.Message})
		return
	}
	httpx.ServerError(w, r, err, "internal error")
}

// Decode reads the JSON body into dst; on false the 400 or 413 is already written.
func Decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	return httpx.DecodeJSON(w, r, dst, MaxBody)
}

// DecodeOptional is Decode for a route where a bare POST means the defaults.
func DecodeOptional(w http.ResponseWriter, r *http.Request, dst any) bool {
	return httpx.DecodeJSONWith(w, r, dst, MaxBody, httpx.DecodeOptions{AllowEmpty: true})
}

// codeAlphabet has 32 symbols, so a random byte masked to 5 bits picks one without bias. No 0/O and 1/I: codes
// are read off a screen and typed on a phone.
const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// Code is n random symbols of the code alphabet (pairing and room codes).
func Code(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // a broken entropy source must not hand out guessable codes
	}
	for i := range b {
		b[i] = codeAlphabet[b[i]&31]
	}
	return string(b)
}

// ID is a random identifier for a row or a player: 16 lower-case symbols, 80 bits.
func ID() string {
	b := []byte(Code(16))
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// Millis is t as the unix milliseconds every API time is.
func Millis(t time.Time) int64 { return t.UnixMilli() }
