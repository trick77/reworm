// Package lastfm reads Last.fm (import only): a track's listeners and top tags, an artist's top tags. Last.fm asks
// for at most 5 requests per second averaged over 5 minutes and a User-Agent naming the app; the caller's
// transport spaces the requests. The API key goes in the query: the caller's cache must leave it out
// (httpcache.Transport.Redact), and no error here names the URL.
package lastfm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/trick77/ship/kit/run"

	"github.com/trick77/reworm/internal/httpcache"
)

// BaseURL is the web service.
const BaseURL = "https://ws.audioscrobbler.com"

// KeyParam is the query parameter that carries the API key.
const KeyParam = "api_key"

const (
	maxBody = 4 << 20
	tries   = 6
)

// Last.fm's error codes (https://www.last.fm/api/errorcodes) the client tells apart. It answers them with a 200 as
// often as with a 4xx/5xx: the body decides.
const (
	errNotFound    = 6  // "Track not found", "The artist you supplied could not be found"
	errOperation   = 8  // operation failed, try again
	errOffline     = 11 // service offline
	errUnavailable = 16 // temporarily unavailable
	errRateLimited = 29
)

// Client reads Last.fm. BaseURL defaults to BaseURL; HTTP to a 30 s client.
type Client struct {
	BaseURL   string
	Key       string
	HTTP      *http.Client
	UserAgent string
	// Sleep waits between retries; tests replace it.
	Sleep func(context.Context, time.Duration) error
	// Evict, when set, drops a cached answer that must not be replayed (httpcache.Transport.Evict): one that does
	// not decode, or a 200 carrying a transient error.
	Evict func(method, url string) error
}

// Tag is one top tag. Count is Last.fm's weight, 100 for the heaviest; 0 when it sent none (a track's tags).
type Tag struct {
	Name  string
	Count int
}

// Track is a track as Last.fm knows it (autocorrected): its name and artist, listeners, top tags heaviest first.
type Track struct {
	Name, Artist string
	Listeners    int
	Tags         []Tag
}

type errorBody struct {
	Error   int    `json:"error"`
	Message string `json:"message"`
}

type tagsBody struct {
	Tag []struct {
		Name  string          `json:"name"`
		Count json.RawMessage `json:"count"`
	} `json:"tag"`
}

// UnmarshalJSON takes an empty list as Last.fm writes it at times, a string ("toptags": "").
func (b *tagsBody) UnmarshalJSON(raw []byte) error {
	if len(raw) > 0 && raw[0] == '"' {
		*b = tagsBody{}
		return nil
	}
	type plain tagsBody
	return json.Unmarshal(raw, (*plain)(b))
}

func (b tagsBody) tags() []Tag {
	out := make([]Tag, 0, len(b.Tag))
	for _, t := range b.Tag {
		out = append(out, Tag{Name: t.Name, Count: number(t.Count)})
	}
	return out
}

// number reads a count Last.fm sends as a number or a string; 0 when it is neither.
func number(raw json.RawMessage) int {
	s := strings.Trim(string(raw), `"`)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// Track looks a track up by artist and title (Last.fm's autocorrect on); found false when Last.fm has none.
func (c *Client) Track(ctx context.Context, artist, title string) (Track, bool, error) {
	var b struct {
		Track struct {
			Name      string          `json:"name"`
			Listeners json.RawMessage `json:"listeners"`
			Artist    struct {
				Name string `json:"name"`
			} `json:"artist"`
			TopTags tagsBody `json:"toptags"`
		} `json:"track"`
	}
	found, err := c.call(ctx, url.Values{"method": {"track.getInfo"}, "artist": {artist}, "track": {title}}, &b)
	if err != nil || !found {
		return Track{}, false, err
	}
	t := b.Track
	return Track{Name: t.Name, Artist: t.Artist.Name, Listeners: number(t.Listeners), Tags: t.TopTags.tags()}, true, nil
}

// ArtistTags are an artist's top tags, heaviest first; none when Last.fm does not know the artist.
func (c *Client) ArtistTags(ctx context.Context, artist string) ([]Tag, error) {
	var b struct {
		TopTags tagsBody `json:"toptags"`
	}
	found, err := c.call(ctx, url.Values{"method": {"artist.getTopTags"}, "artist": {artist}}, &b)
	if err != nil || !found {
		return nil, err
	}
	return b.TopTags.tags(), nil
}

// call runs one API method into out; found false on Last.fm's "not found". It retries what Last.fm calls transient
// (8, 11, 16, 29), a 5xx, a 429, a network error and a body that does not decode, with a doubling backoff; such an
// answer is evicted from the cache first, so no later run replays it.
func (c *Client) call(ctx context.Context, q url.Values, out any) (bool, error) {
	q.Set("autocorrect", "1")
	q.Set("format", "json")
	q.Set(KeyParam, c.Key)
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = BaseURL
	}
	u := base + "/2.0/?" + q.Encode()
	method := q.Get("method")
	wait := 2 * time.Second
	for attempt := 1; ; attempt++ {
		status, body, err := c.once(ctx, u)
		var retry error
		switch {
		case errors.Is(err, httpcache.ErrOffline):
			return false, err
		case err != nil && ctx.Err() == nil:
			retry = err
		case err != nil:
			return false, ctx.Err()
		default:
			var e errorBody
			decodeErr := json.Unmarshal(body, &e)
			switch {
			case decodeErr == nil && e.Error == errNotFound:
				return false, nil
			case decodeErr == nil && e.Error != 0:
				err := fmt.Errorf("lastfm: %s: error %d: %s", method, e.Error, e.Message)
				if !transient(e.Error) {
					return false, err
				}
				retry = err
			case status == http.StatusOK && decodeErr == nil:
				if err := json.Unmarshal(body, out); err != nil {
					retry = fmt.Errorf("lastfm: decode %s: %w", method, err)
				} else {
					return true, nil
				}
			case status == http.StatusOK:
				retry = fmt.Errorf("lastfm: decode %s: %w", method, decodeErr)
			case status == http.StatusTooManyRequests || status >= 500:
				retry = fmt.Errorf("lastfm: %s: status %d", method, status)
			default:
				return false, fmt.Errorf("lastfm: %s: status %d", method, status)
			}
			if status == http.StatusOK && c.Evict != nil {
				if err := c.Evict(http.MethodGet, u); err != nil {
					return false, fmt.Errorf("lastfm: evict: %w", err)
				}
			}
		}
		if attempt >= tries {
			return false, fmt.Errorf("%w after %d tries", retry, attempt)
		}
		if err := c.sleep(ctx, wait); err != nil {
			return false, err
		}
		wait *= 2
	}
}

func transient(code int) bool {
	return code == errOperation || code == errOffline || code == errUnavailable || code == errRateLimited
}

// once is one GET: status and body, or the transport's error, never naming the URL (it carries the key).
func (c *Client) once(ctx context.Context, u string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, nil, errors.New("lastfm: bad request")
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return 0, nil, fmt.Errorf("lastfm: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return 0, nil, fmt.Errorf("lastfm: read: %w", err)
	}
	return resp.StatusCode, body, nil
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
