package app

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/trick77/ship/kit/logging"

	"github.com/trick77/reworm/internal/apple"
	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/preview"
)

// Catalog is what the preview source asks Apple; *apple.Client implements it.
type Catalog interface {
	SongByID(ctx context.Context, storefront, id string) (apple.Song, bool, error)
	SongsByISRC(ctx context.Context, storefront string, isrcs []string) ([]apple.Song, error)
}

// applePreviews resolves a song's preview URL from Apple Music: the stored track (apple_id in its storefront), else
// any track with the song's ISRC (ch, de, us), else preview.ErrNone. Preview URLs are not stored by the import:
// they change; preview.Cache keeps each one a day.
type applePreviews struct{ catalog Catalog }

var isrcStorefronts = []string{"ch", "de", "us"}

func (a applePreviews) PreviewURL(ctx context.Context, s deck.Song) (string, error) {
	if s.AppleID != "" {
		sf := s.Storefront
		if sf == "" {
			sf = isrcStorefronts[0]
		}
		t, found, err := a.catalog.SongByID(ctx, sf, s.AppleID)
		if err != nil {
			return "", err
		}
		if found && t.PreviewURL != "" {
			return t.PreviewURL, nil
		}
	}
	// The track left the catalog (or never had one): any other with the same recording.
	if s.ISRC != "" {
		for _, sf := range isrcStorefronts {
			songs, err := a.catalog.SongsByISRC(ctx, sf, []string{s.ISRC})
			if err != nil {
				return "", err
			}
			for _, t := range songs {
				if t.PreviewURL != "" {
					return t.PreviewURL, nil
				}
			}
		}
	}
	return "", preview.ErrNone
}

// newApplePreviews is the Apple Music preview source: the web player's token kept in the data dir across restarts,
// every URL cached a day.
func newApplePreviews(dataDir string) preview.Source {
	tokenPath := filepath.Join(dataDir, "apple-token.json")
	tokens := &apple.Source{HTTP: &http.Client{Timeout: time.Minute},
		OnNew: func(t apple.Token) {
			if err := apple.SaveToken(tokenPath, t); err != nil {
				slog.Warn("apple token not kept", logging.Err(err))
			}
		}}
	if t, ok := apple.LoadToken(tokenPath, time.Now()); ok {
		tokens.Seed(t)
	}
	client := &apple.Client{HTTP: &http.Client{Timeout: 20 * time.Second}, Dev: tokens}
	return preview.NewCache(applePreviews{catalog: client}, preview.TTL, nil)
}
