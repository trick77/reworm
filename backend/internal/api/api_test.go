package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCodeAndID(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		c := Code(6)
		if len(c) != 6 || strings.Trim(c, codeAlphabet) != "" {
			t.Fatalf("code %q", c)
		}
		seen[c] = true
	}
	if len(seen) < 195 {
		t.Errorf("%d distinct codes of 200", len(seen))
	}
	id := ID()
	if len(id) != 16 || strings.ToLower(id) != id {
		t.Errorf("id %q", id)
	}
	if Millis(time.UnixMilli(42)) != 42 {
		t.Error("Millis")
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, httptest.NewRequest(http.MethodGet, "/", nil), Fail(http.StatusConflict, "room_full", "the room is full"))
	var body struct{ Error, Message string }
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if rec.Code != http.StatusConflict || body.Error != "room_full" || body.Message != "the room is full" {
		t.Errorf("problem = %d %+v", rec.Code, body)
	}
	rec = httptest.NewRecorder()
	WriteError(rec, httptest.NewRequest(http.MethodGet, "/", nil), ErrUnauthorized)
	if rec.Code != http.StatusUnauthorized || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("401 = %d %v", rec.Code, rec.Header())
	}
	rec = httptest.NewRecorder()
	WriteError(rec, httptest.NewRequest(http.MethodGet, "/", nil), errors.New("disk on fire"))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "fire") {
		t.Errorf("500 = %d %s", rec.Code, rec.Body)
	}
	if ErrNotFound.Error() != "not_found: not found" {
		t.Errorf("Error() = %q", ErrNotFound.Error())
	}
}

func TestDecode(t *testing.T) {
	var v struct{ A int }
	rec := httptest.NewRecorder()
	if !Decode(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"A":1}`)), &v) || v.A != 1 {
		t.Error("Decode")
	}
	if !DecodeOptional(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil), &v) {
		t.Error("DecodeOptional of an empty body")
	}
	rec = httptest.NewRecorder()
	if Decode(rec, httptest.NewRequest(http.MethodPost, "/", nil), &v) || rec.Code != http.StatusBadRequest {
		t.Errorf("empty body = %d", rec.Code)
	}
}
