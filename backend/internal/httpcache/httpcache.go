// Package httpcache keeps the import's HTTP responses on disk, keyed by method and URL, so a re-run is cheap and
// resumes where a blocked or interrupted one stopped. Only answers that will not change on a retry are kept: 2xx,
// 3xx and 404. A 401/403 (a replayed refusal would discard every fresh Apple token), 429 and 5xx always go out again.
// Whether a 200's body is usable only its client can tell: it evicts one it cannot decode (Evict).
package httpcache

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/trick77/ship/kit/fsx"
	"github.com/trick77/ship/kit/ratelimit"
)

// HitHeader is set on a response served from disk.
const HitHeader = "X-Httpcache-Hit"

// maxBody bounds one stored response (a MusicBrainz search of 100 recordings is ~1 MB).
const maxBody = 64 << 20

// kept are the response headers a cached answer carries: what the import reads.
var kept = []string{"Content-Type", "Location"}

// Transport answers GET and HEAD from Dir when it can, else asks Next (http.DefaultTransport when nil) and stores
// the answer. Other methods pass through.
type Transport struct {
	Dir  string
	Next http.RoundTripper
}

type meta struct {
	Method string              `json:"method"`
	URL    string              `json:"url"`
	Status int                 `json:"status"`
	Header map[string][]string `json:"header,omitempty"`
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return t.next().RoundTrip(req)
	}
	path := t.path(req)
	if resp, ok := load(path, req); ok {
		return resp, nil
	}
	resp, err := t.next().RoundTrip(req)
	if err != nil {
		return nil, err
	}
	// A body cut off mid-transfer is a read error here (unexpected EOF): returned, never stored.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("httpcache: read %s: %w", req.URL.Redacted(), err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	if cacheable(resp.StatusCode) {
		if err := store(path, req, resp, body); err != nil {
			return nil, err
		}
	}
	return resp, nil
}

// Evict drops the stored answer to method rawURL. A 200 the client cannot decode (complete on the wire, broken in
// content) must not be replayed by every later run: the client evicts it, then retries.
func (t *Transport) Evict(method, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	err = os.Remove(t.key(method, u.String()))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (t *Transport) next() http.RoundTripper {
	if t.Next != nil {
		return t.Next
	}
	return http.DefaultTransport
}

func cacheable(status int) bool {
	return (status >= 200 && status < 400) || status == http.StatusNotFound
}

func (t *Transport) path(req *http.Request) string { return t.key(req.Method, req.URL.String()) }

func (t *Transport) key(method, u string) string {
	sum := sha256.Sum256([]byte(method + " " + u))
	key := hex.EncodeToString(sum[:])
	return filepath.Join(t.Dir, key[:2], key+".gz")
}

func load(path string, req *http.Request) (*http.Response, bool) {
	f, err := os.Open(path) //nolint:gosec // our own cache dir, file name from a hash
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, false
	}
	r := bufio.NewReader(zr)
	line, err := r.ReadBytes('\n')
	if err != nil {
		return nil, false
	}
	var m meta
	if json.Unmarshal(line, &m) != nil || m.URL != req.URL.String() || m.Method != req.Method {
		return nil, false
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, false
	}
	h := http.Header{}
	for k, v := range m.Header {
		h[k] = v
	}
	h.Set(HitHeader, "1")
	return &http.Response{
		Status: fmt.Sprintf("%d %s", m.Status, http.StatusText(m.Status)), StatusCode: m.Status,
		Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1, Header: h, Request: req,
		Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)),
	}, true
}

func store(path string, req *http.Request, resp *http.Response, body []byte) error {
	m := meta{Method: req.Method, URL: req.URL.String(), Status: resp.StatusCode, Header: map[string][]string{}}
	for _, k := range kept {
		if v := resp.Header.Values(k); len(v) > 0 {
			m.Header[k] = v
		}
	}
	line, err := json.Marshal(m)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write(append(line, '\n'))
	_, _ = zw.Write(body)
	if err := zw.Close(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("httpcache: %w", err)
	}
	if err := fsx.WriteFileAtomic(path, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("httpcache: %w", err)
	}
	return nil
}

// Paced lets requests to Next out one per Throttle gap (kit's ratelimit.Throttle, one key for the whole host),
// counted from when each went out: MusicBrainz limits the request rate, and its search answers take ~1 s
// themselves. Put it below a Transport, so only cache misses wait.
type Paced struct {
	Throttle *ratelimit.Throttle
	Next     http.RoundTripper
}

// pacedKey is the one throttle key: a Paced stands for one host.
const pacedKey = "host"

// RoundTrip implements http.RoundTripper.
func (p *Paced) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := p.Throttle.Wait(req.Context(), pacedKey); err != nil {
		return nil, fmt.Errorf("httpcache: pace %s: %w", req.URL.Host, err)
	}
	next := p.Next
	if next == nil {
		next = http.DefaultTransport
	}
	return next.RoundTrip(req)
}

// ErrOffline is what Offline answers: the request would have gone to the network.
var ErrOffline = errors.New("httpcache: offline, not cached")

// Offline is a Next that refuses every request: a Transport over it answers from the cache only.
type Offline struct{}

// RoundTrip implements http.RoundTripper.
func (Offline) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("%w: %s %s", ErrOffline, req.Method, req.URL.Host)
}
