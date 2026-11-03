package dataset

import (
	"errors"
	"testing"
	"testing/fstest"
)

func TestLoad(t *testing.T) {
	got, err := load(fstest.MapFS{"files/songs.jsonl": {Data: []byte(
		`{"id":"a","artist":"ABBA","title":"Waterloo","year":1974,"year_source":"card","sets":["essentials"]}` + "\n")}})
	if err != nil || len(got) != 1 || got[0].Sets[0] != "essentials" {
		t.Fatalf("%+v, %v", got, err)
	}
	if _, err := load(fstest.MapFS{}); !errors.Is(err, ErrMissing) {
		t.Errorf("missing: %v", err)
	}
	if _, err := load(fstest.MapFS{"files/songs.jsonl": {Data: []byte("{")}}); err == nil {
		t.Error("bad file: no error")
	}
	if _, err := load(fstest.MapFS{"files/songs.jsonl": {Mode: 0o20000000000}}); err == nil {
		t.Error("unreadable: no error")
	}
	// The real embed: whatever this build carries, it is never a broken file.
	if _, err := Songs(); err != nil && !errors.Is(err, ErrMissing) {
		t.Errorf("embedded dataset: %v", err)
	}
}
