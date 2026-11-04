// Package apple reads the Apple Music catalog the way the music.apple.com web player does: the player's developer
// token and the player's Origin. Access is unofficial and can break any time. It only reads: there is no method that
// sends anything but GET. Client and token code are copied from loom's tunewire (plan D5), cut to the catalog calls
// reworm uses.
package apple

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/trick77/ship/kit/fsx"
	"github.com/trick77/ship/kit/run"
)

// APIURL is the API host the web player talks to.
const APIURL = "https://amp-api.music.apple.com"

const (
	origin           = "https://music.apple.com"
	maxResponseBytes = 8 << 20
	// maxPages bounds one ISRC lookup: Apple pages filter[isrc] at 25, a hit song has a few pages of compilations.
	maxPages = 8
	// maxRetryAfter caps how long a 429 makes a call wait before a retry.
	maxRetryAfter = time.Minute
	// tries is how often a 429 or a cut-off body is tried before the call fails. Apple's 429 window outlasts a few
	// seconds: a full import at 8 workers met 30 refusals with 12 s of backoff in all (2026-11-04).
	tries = 6
)

var (
	// ErrDevTokenRefused means Apple refused the web player's developer token even freshly fetched: Apple changed
	// something.
	ErrDevTokenRefused = errors.New("apple music refused the web player's developer token")
)

// Tokens hands out the developer token; *Source implements it.
type Tokens interface {
	Token(ctx context.Context) (string, error)
	Invalidate(refused string)
}

// Client calls the catalog API. BaseURL and HTTP default when empty.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Dev     Tokens
	// Sleep waits out a 429; tests replace it.
	Sleep func(context.Context, time.Duration) error
	// Evict, when set, drops a cached answer the client could not decode (httpcache.Transport.Evict).
	Evict func(method, url string) error
}

// Song is one catalog song.
type Song struct {
	ID     string
	Name   string
	Artist string
	Album  string
	// AlbumArtist is the album's artist: the song's own for its albums, "Various Artists" (localized) for the
	// playlist-like compilations that crowd an ISRC lookup.
	AlbumArtist string
	ReleaseDate string // YYYY-MM-DD or YYYY
	ISRC        string
	// Artwork is Apple's image template with its {w}x{h} placeholders, https only.
	Artwork string
	// Preview says Apple ships a preview clip; its URL is fetched at play time, never stored.
	Preview bool
}

// Year is the release year, 0 when Apple sent none.
func (s Song) Year() int {
	if len(s.ReleaseDate) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(s.ReleaseDate[:4])
	return y
}

type songResource struct {
	ID         string `json:"id"`
	Attributes struct {
		Name            string `json:"name"`
		ArtistName      string `json:"artistName"`
		AlbumName       string `json:"albumName"`
		AlbumArtistName string `json:"albumArtistName"`
		ReleaseDate     string `json:"releaseDate"`
		ISRC            string `json:"isrc"`
		Artwork         struct {
			URL string `json:"url"`
		} `json:"artwork"`
		Previews []struct {
			URL string `json:"url"`
		} `json:"previews"`
	} `json:"attributes"`
}

func (r songResource) song() Song {
	a := r.Attributes
	art := a.Artwork.URL
	if !strings.HasPrefix(art, "https://") {
		art = ""
	}
	return Song{ID: r.ID, Name: a.Name, Artist: a.ArtistName, Album: a.AlbumName, AlbumArtist: a.AlbumArtistName,
		ReleaseDate: a.ReleaseDate, ISRC: a.ISRC, Artwork: art, Preview: len(a.Previews) > 0 && a.Previews[0].URL != ""}
}

// SongsByISRC lists a storefront's songs with any of the ISRCs. Apple answers at most 25 per ISRC, without a next
// link or offset (checked 2026-11-03): a hit's original album can be missing, AlbumTracks finds it. Next links are
// followed should Apple ever send them.
func (c *Client) SongsByISRC(ctx context.Context, storefront string, isrcs []string) ([]Song, error) {
	if len(isrcs) == 0 {
		return nil, nil
	}
	q := url.Values{}
	q.Set("filter[isrc]", strings.Join(isrcs, ","))
	return c.songPages(ctx, "/v1/catalog/"+url.PathEscape(storefront)+"/songs?"+q.Encode())
}

// SongByID reads one catalog song; found is false when the storefront has no such song.
func (c *Client) SongByID(ctx context.Context, storefront, id string) (s Song, found bool, err error) {
	songs, err := c.songPages(ctx, "/v1/catalog/"+url.PathEscape(storefront)+"/songs/"+url.PathEscape(id))
	if err != nil || len(songs) == 0 {
		return Song{}, false, err
	}
	return songs[0], true, nil
}

// AlbumTracks lists an album's songs (music videos left out).
func (c *Client) AlbumTracks(ctx context.Context, storefront, albumID string) ([]Song, error) {
	return c.songPages(ctx, "/v1/catalog/"+url.PathEscape(storefront)+"/albums/"+url.PathEscape(albumID)+"/tracks?limit=100")
}

func (c *Client) songPages(ctx context.Context, path string) ([]Song, error) {
	var out []Song
	for n := 0; path != "" && n < maxPages; n++ {
		var page struct {
			Data []struct {
				Type string `json:"type"`
				songResource
			} `json:"data"`
			Next string `json:"next"`
		}
		found, err := c.get(ctx, path, &page)
		if err != nil {
			return nil, err
		}
		if !found {
			break
		}
		for _, d := range page.Data {
			if d.Type == "" || d.Type == "songs" {
				out = append(out, d.song())
			}
		}
		path = page.Next
		if path != "" && !strings.HasPrefix(path, "/v1/") {
			return nil, fmt.Errorf("apple music: unexpected next link %q", path)
		}
	}
	return out, nil
}

// Album is one catalog album.
type Album struct {
	ID          string
	Name        string
	Artist      string
	ReleaseDate string
}

// Year is the release year, 0 when Apple sent none.
func (a Album) Year() int { return Song{ReleaseDate: a.ReleaseDate}.Year() }

// SearchAlbums looks term up among a storefront's albums.
func (c *Client) SearchAlbums(ctx context.Context, storefront, term string, limit int) ([]Album, error) {
	q := url.Values{}
	q.Set("term", term)
	q.Set("types", "albums")
	q.Set("limit", strconv.Itoa(limit))
	var body struct {
		Results struct {
			Albums struct {
				Data []struct {
					ID         string `json:"id"`
					Attributes struct {
						Name        string `json:"name"`
						ArtistName  string `json:"artistName"`
						ReleaseDate string `json:"releaseDate"`
					} `json:"attributes"`
				} `json:"data"`
			} `json:"albums"`
		} `json:"results"`
	}
	if _, err := c.get(ctx, "/v1/catalog/"+url.PathEscape(storefront)+"/search?"+q.Encode(), &body); err != nil {
		return nil, err
	}
	out := make([]Album, 0, len(body.Results.Albums.Data))
	for _, d := range body.Results.Albums.Data {
		a := d.Attributes
		out = append(out, Album{ID: d.ID, Name: a.Name, Artist: a.ArtistName, ReleaseDate: a.ReleaseDate})
	}
	return out, nil
}

// SearchSongs looks term up among a storefront's songs.
func (c *Client) SearchSongs(ctx context.Context, storefront, term string, limit int) ([]Song, error) {
	q := url.Values{}
	q.Set("term", term)
	q.Set("types", "songs")
	q.Set("limit", strconv.Itoa(limit))
	var body struct {
		Results struct {
			Songs struct {
				Data []songResource `json:"data"`
			} `json:"songs"`
		} `json:"results"`
	}
	if _, err := c.get(ctx, "/v1/catalog/"+url.PathEscape(storefront)+"/search?"+q.Encode(), &body); err != nil {
		return nil, err
	}
	out := make([]Song, 0, len(body.Results.Songs.Data))
	for _, d := range body.Results.Songs.Data {
		out = append(out, d.song())
	}
	return out, nil
}

// get fetches path into out; found is false on a 404. A 401 or 403 refetches the developer token once and retries;
// refused again, it is ErrDevTokenRefused. A 429, a network error, a body cut off mid-transfer (Apple does that)
// or a 200 that does not decode waits and retries, tries times in all; the undecodable 200 is evicted first, so
// the cache cannot replay it.
func (c *Client) get(ctx context.Context, path string, out any) (found bool, err error) {
	refetched := false
	for attempt := 1; ; attempt++ {
		wait := time.Second << (attempt - 1) // 1 s, 2 s, 4 s
		dev, err := c.Dev.Token(ctx)
		if err != nil {
			return false, err
		}
		status, body, after, err := c.do(ctx, dev, path)
		switch {
		case err != nil && ctx.Err() != nil:
			return false, ctx.Err()
		case err != nil:
			if attempt >= tries {
				return false, fmt.Errorf("%w after %d tries", err, attempt)
			}
		case status == http.StatusOK:
			err := json.Unmarshal(body, out)
			if err == nil {
				return true, nil
			}
			if c.Evict != nil {
				if err := c.Evict(http.MethodGet, c.base()+path); err != nil {
					return false, fmt.Errorf("apple music: evict %s: %w", redact(path), err)
				}
			}
			if attempt >= tries {
				return false, fmt.Errorf("apple music: decode %s: %w", redact(path), err)
			}
		case status == http.StatusNotFound:
			return false, nil
		case status == http.StatusUnauthorized || status == http.StatusForbidden:
			if refetched {
				return false, ErrDevTokenRefused
			}
			c.Dev.Invalidate(dev)
			refetched = true
			continue
		case status == http.StatusTooManyRequests && attempt < tries:
			wait = min(max(after, 5*time.Second<<(attempt-1)), maxRetryAfter) // 5 s, 10 s, … 1 min
		default:
			return false, fmt.Errorf("apple music: %s returned %d: %s", redact(path), status, apiError(body))
		}
		if err := c.sleep(ctx, wait); err != nil {
			return false, err
		}
	}
}

func (c *Client) base() string {
	if base := strings.TrimRight(c.BaseURL, "/"); base != "" {
		return base
	}
	return APIURL
}

func (c *Client) do(ctx context.Context, dev, path string) (int, []byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+path, nil)
	if err != nil {
		return 0, nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+dev)
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", origin+"/")
	req.Header.Set("Accept", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, 0, fmt.Errorf("apple music: %s: %w", redact(path), err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return 0, nil, 0, fmt.Errorf("apple music: %s: read: %w", redact(path), err)
	}
	return resp.StatusCode, body, retryAfter(resp.Header.Get("Retry-After")), nil
}

func (c *Client) sleep(ctx context.Context, d time.Duration) error {
	if c.Sleep != nil {
		return c.Sleep(ctx, d)
	}
	if !run.Sleep(ctx, d) {
		return ctx.Err()
	}
	return nil
}

func retryAfter(h string) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(h)); err == nil && s > 0 {
		return min(time.Duration(s)*time.Second, maxRetryAfter)
	}
	return 2 * time.Second
}

// redact drops the query from error text.
func redact(path string) string {
	p, _, _ := strings.Cut(path, "?")
	return p
}

// apiError pulls Apple's error title out of a response body.
func apiError(body []byte) string {
	var e struct {
		Errors []struct {
			Title  string `json:"title"`
			Detail string `json:"detail"`
		} `json:"errors"`
	}
	if json.Unmarshal(body, &e) == nil && len(e.Errors) > 0 {
		if e.Errors[0].Detail != "" {
			return e.Errors[0].Title + ": " + e.Errors[0].Detail
		}
		return e.Errors[0].Title
	}
	return "no error detail"
}

// LoadToken reads a token kept by SaveToken; ok is false when there is none or it expired.
func LoadToken(path string, now time.Time) (Token, bool) {
	b, err := os.ReadFile(path) //nolint:gosec // the import's own cache file
	if err != nil {
		return Token{}, false
	}
	var t Token
	if json.Unmarshal(b, &t) != nil || t.Value == "" || !now.Before(t.Expires) {
		return Token{}, false
	}
	return t, true
}

// SaveToken keeps t at path (the import's cache dir), so the next run skips the bundle download.
func SaveToken(path string, t Token) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return fsx.WriteFileAtomic(path, b, 0o600)
}
