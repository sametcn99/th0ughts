package server

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

type response struct {
	header http.Header
	body   bytes.Buffer
	status int
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
	mu    sync.Mutex
	items map[string]*list.Element
	lru   list.List
	bytes int
}

func (c *pageCache) get(k string, v uint64) *response {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el := c.items[k]; el != nil {
		entry := el.Value.(cacheEntry)
		if entry.version == v && time.Now().Before(entry.expires) {
			c.lru.MoveToFront(el)
			return entry.response
		}
	}
	return nil
}
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
		c.remove(el)
	}
	for c.lru.Len() >= 256 || c.bytes+r.body.Len() > 8<<20 {
		c.remove(c.lru.Back())
	}
	c.items[k] = c.lru.PushFront(cacheEntry{k, r, time.Now().Add(30 * time.Second), v})
	c.bytes += r.body.Len()
}
func (c *pageCache) remove(el *list.Element) {
	old := el.Value.(cacheEntry)
	c.bytes -= old.response.body.Len()
	delete(c.items, old.key)
	c.lru.Remove(el)
}
func serveCopy(w http.ResponseWriter, r *http.Request, b *response) {
	for k, v := range b.header {
		w.Header()[k] = v
	}
	if tag := b.header.Get("ETag"); tag != "" && r.Header.Get("If-None-Match") == tag {
		w.WriteHeader(304)
		return
	}
	w.WriteHeader(b.status)
	w.Write(b.body.Bytes())
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
	}
	v := s.S.Version.Load() + s.T.Version.Load()
	if hit := s.cache.get(key, v); hit != nil {
		if l := r.URL.Query().Get("lang"); l == "en" || l == "tr" {
			s.setCookie(w, r, "lang", l, 31536000)
		}
		w.Header().Set("X-Cache", "HIT")
		serveCopy(w, r, hit)
		return
	}
	b := &response{header: w.Header().Clone(), status: 200}
	s.page(b, r)
	if b.status != 200 {
		serveCopy(w, r, b)
		return
	}
	for _, v := range b.header.Values("Set-Cookie") {
		w.Header().Add("Set-Cookie", v)
	}
	b.header.Del("Set-Cookie")
	h := sha256.Sum256(b.body.Bytes())
	b.header.Set("ETag", "\""+hex.EncodeToString(h[:16])+"\"")
	b.header.Set("Cache-Control", "private, no-cache")
	s.cache.put(key, v, b)
	w.Header().Set("X-Cache", "MISS")
	serveCopy(w, r, b)
}
