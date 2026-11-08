package deck

// Song is a song as a round needs it. The answer (artist, title, year) never leaves the server before a reveal.
//
// Table: songs
type Song struct {
	ID         string
	Artist     string
	Title      string
	Year       int
	ISRC       string
	AppleID    string
	Storefront string
	// CoverVersion names the cover the import stored (CoverVersion), "" when there is none; the image itself is
	// served by /api/covers/{id}.
	CoverVersion string
}
