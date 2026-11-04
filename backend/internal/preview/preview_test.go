package preview

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/trick77/reworm/internal/deck"
)

func TestCache(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0)
	f := &Fake{Missing: map[string]bool{"gone": true}}
	c := NewCache(f, TTL, func() time.Time { return now })
	for range 3 {
		u, err := c.PreviewURL(ctx, deck.Song{ID: "dq"})
		if err != nil || u != "https://previews.invalid/dq.m4a" {
			t.Fatalf("PreviewURL = %q, %v", u, err)
		}
	}
	if f.Calls() != 1 {
		t.Errorf("source calls = %d, want 1 (cached)", f.Calls())
	}
	now = now.Add(TTL)
	if _, err := c.PreviewURL(ctx, deck.Song{ID: "dq"}); err != nil || f.Calls() != 2 {
		t.Errorf("after the TTL: calls = %d, %v; want a fresh lookup", f.Calls(), err)
	}
	for range 2 {
		if _, err := c.PreviewURL(ctx, deck.Song{ID: "gone"}); !errors.Is(err, ErrNone) {
			t.Errorf("missing = %v", err)
		}
	}
	if f.Calls() != 4 {
		t.Errorf("calls = %d: a failure must not be cached", f.Calls())
	}
}

func TestCache_bounded(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0)
	c := NewCache(&Fake{}, time.Minute, func() time.Time { return now })
	for i := range maxEntries {
		if _, err := c.PreviewURL(ctx, deck.Song{ID: fmt.Sprint(i)}); err != nil {
			t.Fatal(err)
		}
	}
	// All still fresh: the cache starts over rather than grow.
	if _, err := c.PreviewURL(ctx, deck.Song{ID: "new"}); err != nil || len(c.m) != 1 {
		t.Errorf("entries = %d, %v; want 1", len(c.m), err)
	}
	// "new" expires; refill to the bound with fresh ones, then one more: only the expired entry makes room.
	now = now.Add(time.Hour)
	for i := range maxEntries {
		_, _ = c.PreviewURL(ctx, deck.Song{ID: fmt.Sprint("b", i)})
	}
	if _, stale := c.m["new"]; stale || len(c.m) != maxEntries {
		t.Errorf("entries = %d, expired kept = %v", len(c.m), stale)
	}
	if NewCache(&Fake{}, time.Minute, nil).now().IsZero() {
		t.Error("nil clock is not time.Now")
	}
}
