package server

import (
	"bytes"
	"compress/gzip"
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type response struct {
	header     http.Header
	body       bytes.Buffer
	status     int
	compressed []byte
}

func (r *response) Header() http.Header         { return r.header }
func (r *response) WriteHeader(status int)      { r.status = status }
func (r *response) Write(b []byte) (int, error) { return r.body.Write(b) }

type cacheEntry struct {
	key      string
	response *response
	expires  time.Time
	version  uint64
}
type pageCache struct {
	mu      sync.Mutex
	flights map[flightKey]*pageFlight
	items   map[string]*list.Element
	lru     list.List
	bytes   int
}

type flightKey struct {
	key     string
	version uint64
}

type pageFlight struct {
	done chan struct{}
	page *response
}

// Lookup and elect the renderer under the same lock. Publication changes use a
// different flight, even when an earlier render is still running.
func (c *pageCache) lookup(k flightKey) (hit *response, flight *pageFlight, leader bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el := c.items[k.key]; el != nil {
		entry := el.Value.(cacheEntry)
		if entry.version == k.version && time.Now().Before(entry.expires) {
			c.lru.MoveToFront(el)
			return entry.response, nil, false
		}
	}
	if f := c.flights[k]; f != nil {
		return nil, f, false
	}
	if c.flights == nil {
		c.flights = make(map[flightKey]*pageFlight)
	}
	f := &pageFlight{done: make(chan struct{})}
	c.flights[k] = f
	return nil, f, true
}
func (c *pageCache) finish(k flightKey, f *pageFlight, b *response) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f.page = b
	delete(c.flights, k)
	close(f.done)
}
func (r *response) size() int { return r.body.Len() + len(r.compressed) }

func (c *pageCache) put(k string, v uint64, r *response) {
	if r.status != 200 || r.body.Len() > 256<<10 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		c.items = map[string]*list.Element{}
	}
	if el := c.items[k]; el != nil {
		if el.Value.(cacheEntry).version > v {
			return
		}
		c.remove(el)
	}
	for c.lru.Len() >= 256 || c.bytes+r.size() > 8<<20 {
		c.remove(c.lru.Back())
	}
	c.items[k] = c.lru.PushFront(cacheEntry{k, r, time.Now().Add(30 * time.Second), v})
	c.bytes += r.size()
}
func (c *pageCache) remove(el *list.Element) {
	old := el.Value.(cacheEntry)
	c.bytes -= old.response.size()
	delete(c.items, old.key)
	c.lru.Remove(el)
}

var compressors = sync.Pool{New: func() any { z, _ := gzip.NewWriterLevel(nil, gzip.BestSpeed); return z }}

func (b *response) prepare() {
	h := sha256.Sum256(b.body.Bytes())
	b.header.Set("ETag", "\""+hex.EncodeToString(h[:16])+"\"")
	b.header.Set("Cache-Control", "private, no-cache")
	b.header.Add("Vary", "Accept-Encoding")
	if b.body.Len() < 1024 || b.body.Len() > 256<<10 {
		return
	}
	var compressed bytes.Buffer
	z := compressors.Get().(*gzip.Writer)
	z.Reset(&compressed)
	z.Write(b.body.Bytes())
	z.Close()
	z.Reset(nil)
	compressors.Put(z)
	if compressed.Len() < b.body.Len() {
		b.compressed = compressed.Bytes()
	}
}

func serveCopy(w http.ResponseWriter, r *http.Request, b *response) {
	for k, v := range b.header {
		w.Header()[k] = v
	}
	body := b.body.Bytes()
	tag := b.header.Get("ETag")
	if b.compressed != nil && acceptsGzip(r.Header.Get("Accept-Encoding")) {
		body = b.compressed
		w.Header().Set("Content-Encoding", "gzip")
		tag = strings.TrimSuffix(tag, "\"") + "-gz\""
		w.Header().Set("ETag", tag)
	}
	if tag != "" && r.Header.Get("If-None-Match") == tag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(b.status)
	if r.Method != http.MethodHead {
		w.Write(body)
	}
}

const timelineMediaType = "application/vnd.th0ughts.timeline+json"

func wantsTimelineFragment(r *http.Request) bool {
	return r.Header.Get("Accept") == timelineMediaType
}

func (s *Server) cachedPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/login" || r.URL.Path == "/sitemap.xml" || r.URL.Path == "/robots.txt" || r.Header.Get("Authorization") != "" || cookie(r, "session") != "" || cookie(r, "bootstrap") != "" || r.URL.Query().Has("preview") || len(r.URL.RawQuery) > 2048 {
		s.page(w, r)
		return
	}
	lang := cookie(r, "lang")
	if lang != "en" && lang != "tr" {
		lang = ""
	}
	key := r.Host + "\x00" + r.URL.RequestURI() + "\x00" + lang
	if strings.Contains(r.Header.Get("Accept"), "text/markdown") {
		key += "\x00md"
	} else if wantsTimelineFragment(r) {
		key += "\x00timeline"
	}
	if r.TLS != nil {
		key += "\x00tls"
	}
	version := func() uint64 { return s.S.Version.Load() + s.T.Version.Load() }
	for {
		k := flightKey{key, version()}
		hit, flight, leader := s.cache.lookup(k)
		if leader {
			b := s.renderPublic(w, r, k, flight)
			serveCopy(w, r, b)
			return
		}
		if hit == nil {
			select {
			case <-r.Context().Done():
				return
			case <-flight.done:
			}
			if version() != k.version {
				continue
			}
			hit = flight.page
			if hit == nil {
				s.page(w, r)
				return
			}
		}
		if l := r.URL.Query().Get("lang"); l == "en" || l == "tr" {
			s.setCookie(w, r, "lang", l, 31536000)
		}
		w.Header().Set("X-Cache", "HIT")
		serveCopy(w, r, hit)
		return
	}
}

func (s *Server) renderPublic(w http.ResponseWriter, r *http.Request, k flightKey, flight *pageFlight) *response {
	var shared *response
	defer func() { s.cache.finish(k, flight, shared) }()
	b := &response{header: w.Header().Clone(), status: 200}
	s.page(b, r)
	if b.status != 200 {
		return b
	}
	for _, v := range b.header.Values("Set-Cookie") {
		w.Header().Add("Set-Cookie", v)
	}
	b.header.Del("Set-Cookie")
	b.prepare()
	s.cache.put(k.key, k.version, b)
	w.Header().Set("X-Cache", "MISS")
	shared = b
	return b
}
