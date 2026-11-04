// Package dev is the local test rig (e2e, a laptop without Apple Music): clips from a directory of audio files
// instead of Apple previews, and a fixture song list instead of the dataset. Off unless BACKEND_DEV_PREVIEWS or
// BACKEND_DEV_SONGS is set, and refused together with PUBLIC_FQDN (app.LoadConfig): a dev clip URL names the song,
// so it must never reach a real party.
package dev

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/preview"
	"github.com/trick77/reworm/internal/store"
)

// PreviewPath is where the dev clips are served.
const PreviewPath = "/api/dev/previews/"

// Previews is a preview.Source over a directory: song <id> plays <dir>/<id>.wav, a song without a file has no
// preview (the round skips it). It serves the files itself.
type Previews struct {
	files fs.FS
}

// NewPreviews serves the clips in dir.
func NewPreviews(dir string) (*Previews, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("dev previews: %w", err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("dev previews: %s is not a directory", dir)
	}
	return &Previews{files: os.DirFS(dir)}, nil
}

// PreviewURL is the song's clip under PreviewPath. The id is in the URL on purpose: an e2e test reads the answer
// from it.
func (p *Previews) PreviewURL(_ context.Context, s deck.Song) (string, error) {
	name := s.ID + ".wav"
	if !p.has(name) {
		return "", preview.ErrNone
	}
	return PreviewPath + url.PathEscape(name), nil
}

// has reports whether name is a plain file directly in the directory.
func (p *Previews) has(name string) bool {
	if !fs.ValidPath(name) || path.Base(name) != name {
		return false
	}
	st, err := fs.Stat(p.files, name)
	return err == nil && st.Mode().IsRegular()
}

// Mount adds the clip route.
func (p *Previews) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET "+PreviewPath+"{file}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("file")
		if !p.has(name) {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, p.files, name)
	})
}

// Song is one line of a fixture file (JSON Lines).
type Song struct {
	ID         string   `json:"id"`
	Artist     string   `json:"artist"`
	Title      string   `json:"title"`
	Year       int      `json:"year"`
	Sets       []string `json:"sets"`
	Categories []string `json:"categories"`
	Genres     []string `json:"genres"`
}

// LoadSongs makes the fixture file the active song list: every other song is deactivated (kept, as a dataset
// change keeps them), the fixture's are upserted. It returns how many it loaded.
func LoadSongs(ctx context.Context, db *sql.DB, file string) (int, error) {
	songs, err := readSongs(file)
	if err != nil {
		return 0, err
	}
	err = store.InTx(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE songs SET active = 0`); err != nil {
			return err
		}
		for _, s := range songs {
			sets, cats, genres := jsonList(s.Sets), jsonList(s.Categories), jsonList(s.Genres)
			if _, err := tx.ExecContext(ctx, `
INSERT INTO songs (id, artist, title, year, year_source, sets, categories, genres, active)
VALUES (?, ?, ?, ?, 'fixture', ?, ?, ?, 1)
ON CONFLICT (id) DO UPDATE SET artist = excluded.artist, title = excluded.title, year = excluded.year,
	year_source = excluded.year_source, sets = excluded.sets, categories = excluded.categories,
	genres = excluded.genres, active = 1`,
				s.ID, s.Artist, s.Title, s.Year, sets, cats, genres); err != nil {
				return fmt.Errorf("song %s: %w", s.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("dev songs: %w", err)
	}
	return len(songs), nil
}

func readSongs(file string) ([]Song, error) {
	f, err := os.Open(file) //nolint:gosec // the operator's own fixture path
	if err != nil {
		return nil, fmt.Errorf("dev songs: %w", err)
	}
	defer func() { _ = f.Close() }()
	var out []Song
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var s Song
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			return nil, fmt.Errorf("dev songs: %s:%d: %w", file, n, err)
		}
		switch {
		case s.ID == "" || s.Artist == "" || s.Title == "" || s.Year <= 0:
			return nil, fmt.Errorf("dev songs: %s:%d: id, artist, title and year are required", file, n)
		case seen[s.ID]:
			return nil, fmt.Errorf("dev songs: %s:%d: id %s twice", file, n, s.ID)
		}
		seen[s.ID] = true
		out = append(out, s)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("dev songs: %w", err)
	}
	if len(out) == 0 {
		return nil, errors.New("dev songs: " + file + " has no songs")
	}
	return out, nil
}

// jsonList is a JSON array of strings, [] for none (the columns are never null).
func jsonList(v []string) string {
	if v == nil {
		v = []string{}
	}
	b, _ := json.Marshal(v) // a string slice always marshals
	return string(b)
}
