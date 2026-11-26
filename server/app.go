// Package server runs the embedded web application and its transport adapters.
package server

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"io"

	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/storage"
	theme "github.com/sametcn99/th0ughts/themes"
)

//go:embed assets/* locales/*.json preview/*.html
var assets embed.FS

type ticket struct {
	Expires time.Time
	Value   any
	Binding string
}
type Server struct {
	guest       guestMail
	imageMu     sync.Mutex
	image       socialImage
	instance    *flock.Flock
	cache       *pageCache
	mu          sync.RWMutex
	lock        sync.Mutex
	tickets     map[string]ticket
	attempts    map[string]time.Time
	dir, master string
	v           feed.Vault
	cfg         storage.Config
	S           *feed.Service
	A           *feed.Auth
	L           *feed.Library
	T           *feed.Themes
	catalog     map[string]map[string]string
	mux         *http.ServeMux
}

func Open(ctx context.Context, dir, key string, initial storage.Config) (*Server, error) {
	v, e := feed.NewVault(key)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	s := &Server{dir: dir, master: feed.Digest(key), v: v, tickets: map[string]ticket{}, attempts: map[string]time.Time{}, catalog: map[string]map[string]string{}}
	p := filepath.Join(dir, "connection.enc")
	b, e := os.ReadFile(p)
	if e == nil {
		raw, e := v.Open("database", b)
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &initial); e != nil {
			return nil, e
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	if initial.Driver == "" {
		initial = storage.Config{Driver: "sqlite", Address: filepath.Join(dir, "feed.db")}
	}
	s.instance = flock.New(filepath.Join(dir, ".instance.lock"))
	ok, e := s.instance.TryLock()
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, errors.New("data directory is already in use")
	}
	if e = s.connect(ctx, initial); e != nil {
		s.instance.Unlock()
		return nil, e
	}
	for _, l := range []string{"en", "tr"} {
		b, e := assets.ReadFile("locales/" + l + ".json")
		if e != nil {
			s.Close()
			return nil, e
		}
		var m map[string]string
		if e = json.Unmarshal(b, &m); e != nil {
			s.Close()
			return nil, e
		}
		s.catalog[l] = m
	}
	s.routes()
	return s, nil
}
func (s *Server) connect(ctx context.Context, c storage.Config) error {
	db, e := storage.Open(ctx, c)
	if e != nil {
		return e
	}
	if s.S != nil {
		if _, e := db.Get(ctx, "auth", "owner"); !errors.Is(e, storage.ErrMissing) {
			db.Close()
			return feed.Conflict
		}
	}
	b, _ := json.Marshal(c)
	if e = storage.Atomic(filepath.Join(s.dir, "connection.enc"), s.v.Seal("database", b)); e != nil {
		db.Close()
		return e
	}
	if s.S != nil {
		s.S.DB.Close()
	}
	s.cache = &pageCache{}
	s.cfg = c
	s.S = feed.New(db)
	s.T = s.S.Themes
	s.A = feed.NewAuth(db, s.v)
	s.L = &feed.Library{Service: s.S, Dir: filepath.Join(s.dir, "media"), Vault: s.v}
	if e = s.L.RepairUploads(ctx); e != nil {
		return e
	}
	if e = s.L.RollbackImport(ctx); e != nil {
		return e
	}
	if e = s.L.RepairCommentMedia(ctx); e != nil {
		return e
	}
	return s.T.Reclaim(ctx)
}
func (s *Server) Close() error { return errors.Join(s.S.DB.Close(), s.instance.Unlock()) }
func (s *Server) issueTicket(value any, binding string) string {
	s.lock.Lock()
	defer s.lock.Unlock()
	now := time.Now()
	for k, v := range s.tickets {
		if now.After(v.Expires) {
			delete(s.tickets, k)
		}
	}
	key := feed.ID(24)
	s.tickets[key] = ticket{now.Add(5 * time.Minute), value, binding}
	return key
}
func (s *Server) ticket(key, binding string, consume bool) (any, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	v, ok := s.tickets[key]
	if consume {
		delete(s.tickets, key)
	}
	return v.Value, ok && time.Now().Before(v.Expires) && v.Binding == binding
}
func (s *Server) routes() {
	m := http.NewServeMux()
	s.mux = m
	for _, p := range []string{"GET /{$}", "GET /login", "GET /feed.md", "GET /post/{id}", "GET /feed.xml", "GET /sitemap.xml", "GET /robots.txt"} {
		m.HandleFunc(p, s.cachedPage)
	}
	m.HandleFunc("POST /api/v1/comment-code", s.requestCode)
	m.HandleFunc("POST /api/v1/comments/{post}", s.submitComment)
	m.HandleFunc("GET /social.png", s.social)
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	m.HandleFunc("GET /assets/{name}", static)
	for _, route := range []string{"GET /api/v1/posts", "POST /api/v1/posts", "GET /api/v1/posts/{id}", "PUT /api/v1/posts/{id}", "DELETE /api/v1/posts/{id}"} {
		m.HandleFunc(route, s.postResource)
	}
	m.HandleFunc("GET /api/v1/tools", s.toolList)
	m.HandleFunc("GET /api/v1/openapi.json", s.openapi)
	m.HandleFunc("POST /api/v1/{op}", s.operation)
	m.HandleFunc("GET /api/v1/{op}", s.operation)
	m.HandleFunc("POST /auth/{op}", s.auth)
	m.HandleFunc("GET /media/{id}", s.media)
	m.HandleFunc("POST /api/v1/upload", s.upload)
	m.HandleFunc("GET /api/v1/backup", s.backup)
	m.HandleFunc("POST /api/v1/restore", s.restore)
	m.HandleFunc("/mcp", s.mcpHTTP)
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	budget := 30 * time.Second
	if r.URL.Path == "/api/v1/restore" || r.URL.Path == "/api/v1/backup" || r.URL.Path == "/api/v1/upload" || strings.HasPrefix(r.URL.Path, "/media/") {
		budget = 3 * time.Minute
	}
	ctx, cancel := context.WithTimeout(r.Context(), budget)
	defer cancel()
	r = r.WithContext(ctx)
	if name := auditAction(r); name != "" {
		audit := &auditedResponse{w, http.StatusOK}
		defer logSecurity(name, audit, r, time.Now())
		w = audit
	}
	if r.URL.Path == "/auth/setup" || r.URL.Path == "/api/v1/restore" || r.URL.Path == "/api/v1/backup" {
		s.mu.Lock()
		defer s.mu.Unlock()
	} else {
		s.mu.RLock()
		defer s.mu.RUnlock()
	}
	if secureTransport(r) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; media-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'self'")
	if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
		if origin := r.Header.Get("Origin"); origin != "" && origin != s.origin(r) {
			fail(w, feed.Denied)
			return
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			fail(w, feed.Denied)
			return
		}
		if cookie(r, "session") != "" && r.Header.Get("X-Th0ughts-Request") != "1" {
			fail(w, feed.Denied)
			return
		}
	}
	s.mux.ServeHTTP(w, r)
}
func (s *Server) origin(r *http.Request) string {
	c, e := s.S.Settings(r.Context())
	if e == nil && c.URL != "" {
		return c.URL
	}
	scheme := "http"
	host, _, _ := net.SplitHostPort(r.Host)
	if r.TLS != nil || host != "localhost" && host != "127.0.0.1" && host != "::1" && r.Host != "localhost" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
func (s *Server) actor(r *http.Request) feed.Actor {
	if t := r.Header.Get("Authorization"); t != "" {
		return s.A.Resolve(r.Context(), "token", strings.TrimPrefix(t, "Bearer "))
	}
	if raw := cookie(r, "session"); raw != "" {
		return s.A.Resolve(r.Context(), "session", raw)
	}
	return feed.Actor{}
}
func secureTransport(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = strings.Trim(r.Host, "[]")
	}
	ip := net.ParseIP(host)
	return r.TLS != nil || !(strings.EqualFold(host, "localhost") || ip != nil && ip.IsLoopback())
}
func cookieName(r *http.Request, name string) string {
	if secureTransport(r) {
		return "__Host-" + name
	}
	return name
}
func (s *Server) setCookie(w http.ResponseWriter, r *http.Request, k, v string, age int) {
	http.SetCookie(w, &http.Cookie{Name: cookieName(r, k), Value: v, Path: "/", HttpOnly: true, Secure: secureTransport(r), SameSite: http.SameSiteStrictMode, MaxAge: age})
}
func (s *Server) locale(r *http.Request, site feed.Settings) string {
	l := r.URL.Query().Get("lang")
	if feed.Locale(l) {
		return l
	}
	if l := cookie(r, "lang"); feed.Locale(l) {
		return l
	}
	return site.Lang
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}
func errorStatus(e error) (int, string) {
	code := 500
	msg := "internal_error"
	switch {
	case errors.Is(e, dnsUnavailable):
		return 503, e.Error()
	case errors.Is(e, feed.EmailCodeRequired):
		return 409, e.Error()
	case errors.Is(e, invalidEmail), errors.Is(e, invalidCode):
		return 400, e.Error()
	case errors.Is(e, mailUnavailable):
		return 503, "mail_unavailable"
	case errors.Is(e, commentRate):
		return 429, "comment_rate"
	case errors.Is(e, feed.CommentsClosed):
		return 403, "comments_closed"
	case errors.Is(e, feed.Denied):
		code = 403
		msg = "forbidden"
	case isThemeError(e):
		return 400, "invalid_theme"
	case errors.Is(e, feed.Invalid):
		code = 400
		msg = "invalid_input"
	case errors.Is(e, feed.Conflict):
		code = 409
		msg = "conflict"
	case errors.Is(e, storage.ErrMissing):
		code = 404
		msg = "not_found"
	}
	return code, msg
}
func isThemeError(e error) bool { var d *theme.Diagnostic; return errors.As(e, &d) }
func fail(w http.ResponseWriter, e error) {
	code, msg := errorStatus(e)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	body := map[string]any{"error": msg}
	var d *theme.Diagnostic
	if errors.As(e, &d) {
		body["diagnostic"] = d
	}
	json.NewEncoder(w).Encode(body)
}
func decode(w http.ResponseWriter, r *http.Request, to any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(to); e != nil {
		return feed.Invalid
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF {
		return feed.Invalid
	}
	return nil
}
func (s *Server) masterOK(v string) bool {
	return subtle.ConstantTimeCompare([]byte(feed.Digest(v)), []byte(s.master)) == 1
}
func (s *Server) allowedAttempt(r *http.Request) bool {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	s.lock.Lock()
	defer s.lock.Unlock()
	now := time.Now()
	for k, v := range s.attempts {
		if v.Before(now) {
			delete(s.attempts, k)
		}
	}
	if s.attempts[host].After(now.Add(8 * time.Second)) {
		return false
	}
	t := s.attempts[host]
	if t.Before(now) {
		t = now
	}
	s.attempts[host] = t.Add(2 * time.Second)
	return true
}
func (s *Server) media(w http.ResponseWriter, r *http.Request) {
	m, f, e := s.L.Open(r.Context(), s.actor(r), r.PathValue("id"))
	if e != nil {
		fail(w, e)
		return
	}
	defer f.Close()
	if size := r.URL.Query().Get("w"); (size == "480" || size == "960") && !m.Video() {
		n, _ := strconv.Atoi(size)
		if n < m.Width {
			if e = s.thumbnail(w, r, m, f, n); e != nil {
				fail(w, e)
			}
			return
		}
	}
	w.Header().Set("Content-Type", m.Type)
	http.ServeContent(w, r, m.ID, time.Time{}, f)
}
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	m, e := s.L.Upload(r.Context(), s.actor(r), http.MaxBytesReader(w, r.Body, feed.MaxMedia+1))
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, m)
}
func link(base, id, lang string) string {
	q := url.Values{"lang": {lang}}
	if id != "" {
		q.Set("post", id)
	}
	return base + "/?" + q.Encode()
}

func (s *Server) DefaultTheme() string { return theme.Builtins()["paper"] }
