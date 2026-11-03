// Package dataset embeds the song dataset, data/songs.jsonl. go:embed cannot reach outside the module, so the
// build copies the file into files/ first (Containerfile, make build/run), the way web/dist gets the UI build;
// files/ holds only .gitkeep in a fresh clone.
package dataset

import (
	"embed"
	"errors"
	"io/fs"

	"github.com/trick77/reworm/internal/song"
)

//go:embed all:files
var files embed.FS

// ErrMissing says the binary was built without the dataset.
var ErrMissing = errors.New("dataset: songs.jsonl not embedded (copy data/songs.jsonl into internal/dataset/files before the build)")

// Songs decodes the embedded dataset.
func Songs() ([]song.Song, error) { return load(files) }

func load(fsys fs.FS) ([]song.Song, error) {
	b, err := fs.ReadFile(fsys, "files/songs.jsonl")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrMissing
	}
	if err != nil {
		return nil, err
	}
	return song.Decode(b)
}
