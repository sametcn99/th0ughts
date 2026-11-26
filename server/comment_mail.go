package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/storage"
	"golang.org/x/net/idna"
	"math/big"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

var invalidEmail = errors.New("invalid_email")
var invalidCode = errors.New("invalid_comment_code")
var mailUnavailable = errors.New("mail_unavailable")
var commentRate = errors.New("comment_rate")

type mailRelay struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	From     string `json:"from"`
	User     string `json:"user"`
	Password string `json:"password,omitempty"`
	TLSMode  string `json:"tlsMode"`
}

func mailbox(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	local, domain, ok := strings.Cut(raw, "@")
	host, e := idna.Lookup.ToASCII(domain)
	if !ok || e != nil || len(local) == 0 || len(local) > 64 || !strings.Contains(host, ".") || strings.HasSuffix(host, ".") || net.ParseIP(host) != nil || strings.ContainsAny(local, "<>(),:;[]\\\" \t\r\n") {
		return "", invalidEmail
	}
	address := local + "@" + strings.ToLower(host)
	parsed, e := mail.ParseAddress(address)
	if e != nil || parsed.Address != address || len(address) > 254 {
		return "", invalidEmail
	}
	return address, nil
}
func (s *Server) relay(ctx context.Context) (mailRelay, error) {
	var m mailRelay
	enc, e := feed.Read[[]byte](ctx, s.S.DB, "config", "comment-mail")
	if errors.Is(e, storage.ErrMissing) {
		return m, nil
	}
	if e != nil {
		return m, e
	}
	raw, e := s.v.Open("comment-mail", enc)
	if e != nil {
		return m, e
	}
	e = json.Unmarshal(raw, &m)
	return m, e
}
func (s *Server) saveRelay(ctx context.Context, a feed.Actor, m mailRelay) error {
	if !a.Recent() {
		return feed.Denied
	}
	m.Host = strings.TrimSpace(m.Host)
	var e error
	m.From, e = mailbox(m.From)
	if e != nil || m.Host == "" || len(m.Host) > 253 || strings.ContainsAny(m.Host, "/\\@ \t\r\n") || m.Port < 1 || m.Port > 65535 || m.TLSMode != "tls" && m.TLSMode != "starttls" || strings.IndexFunc(m.User, unicode.IsControl) >= 0 {
		return feed.Invalid
	}
	if m.Password == "" && m.User != "" {
		old, e := s.relay(ctx)
		if e != nil {
			return e
		}
		if old.Host == m.Host && old.Port == m.Port && old.User == m.User {
			m.Password = old.Password
		}
	}
	b, e := json.Marshal(m)
	if e != nil {
		return e
	}
	return feed.Write(ctx, s.S.DB, "config", "comment-mail", s.v.Seal("comment-mail", b), 0, false)
}
func sendCommentMail(ctx context.Context, m mailRelay, to, subject, body string) error {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	target := net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
	d := net.Dialer{}
	conn, e := d.DialContext(ctx, "tcp", target)
	if e != nil {
		return e
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	cfg := &tls.Config{ServerName: m.Host, MinVersion: tls.VersionTLS12}
	if m.TLSMode == "tls" {
		conn = tls.Client(conn, cfg)
	}
	c, e := smtp.NewClient(conn, m.Host)
	if e != nil {
		return e
	}
	defer c.Close()
	if m.TLSMode == "starttls" {
		if e = c.StartTLS(cfg); e != nil {
			return e
		}
	}
	if m.User != "" {
		if e = c.Auth(smtp.PlainAuth("", m.User, m.Password, m.Host)); e != nil {
			return e
		}
	}
	if e = c.Mail(m.From); e != nil {
		return e
	}
	if e = c.Rcpt(to); e != nil {
		return e
	}
	w, e := c.Data()
	if e != nil {
		return e
	}
	_, e = fmt.Fprintf(w, "From: %s\r\nTo: %s\r\nDate: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n", m.From, to, time.Now().Format(time.RFC1123Z), mime.QEncoding.Encode("UTF-8", subject), base64.StdEncoding.EncodeToString([]byte(body)))
	if e != nil {
		return e
	}
	if e = w.Close(); e != nil {
		return e
	}
	return c.Quit()
}

type commentChallenge struct {
	post, digest string
	until        time.Time
	tries        int
	busy         bool
}
type mailWindow struct {
	until time.Time
	n     int
}
type guestMail struct {
	mu         sync.Mutex
	challenges map[string]*commentChallenge
	limits     map[string]mailWindow
	send       func(context.Context, mailRelay, string, string, string) error
}

func (g *guestMail) issue(post, email, ip, code string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	if g.challenges == nil {
		g.challenges = map[string]*commentChallenge{}
		g.limits = map[string]mailWindow{}
	}
	for k, v := range g.challenges {
		if now.After(v.until) {
			delete(g.challenges, k)
		}
	}
	for k, v := range g.limits {
		if now.After(v.until) {
			delete(g.limits, k)
		}
	}
	type quota struct {
		k      string
		max    int
		period time.Duration
	}
	quotas := []quota{{"all", 120, time.Hour}, {"ip:" + ip, 12, time.Hour}, {"email:" + feed.Digest(strings.ToLower(email)), 1, time.Minute}}
	for _, q := range quotas {
		if g.limits[q.k].n >= q.max {
			return "", commentRate
		}
	}
	for _, q := range quotas {
		v := g.limits[q.k]
		if v.n == 0 {
			v.until = now.Add(q.period)
		}
		v.n++
		g.limits[q.k] = v
	}
	id := feed.ID(24)
	g.challenges[id] = &commentChallenge{post: post, digest: feed.Digest(id + code), until: now.Add(10 * time.Minute)}
	return id, nil
}
func (g *guestMail) reserve(id, post, code string) (func(bool), error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	c := g.challenges[id]
	if c == nil || c.post != post || time.Now().After(c.until) || c.busy || c.tries >= 5 {
		return nil, invalidCode
	}
	if subtle.ConstantTimeCompare([]byte(feed.Digest(id+code)), []byte(c.digest)) != 1 {
		c.tries++
		return nil, invalidCode
	}
	c.tries++
	c.busy = true
	return func(done bool) {
		g.mu.Lock()
		defer g.mu.Unlock()
		if done {
			delete(g.challenges, id)
		} else {
			c.busy = false
		}
	}, nil
}
func (s *Server) requestCode(w http.ResponseWriter, r *http.Request) {
	var x struct{ Post, Email, Lang string }
	if e := decode(w, r, &x); e != nil {
		fail(w, e)
		return
	}
	if _, e := s.S.CommentPolicy(r.Context(), x.Post, true); e != nil {
		fail(w, e)
		return
	}
	address, e := mailbox(x.Email)
	if e != nil {
		fail(w, e)
		return
	}
	m, e := s.relay(r.Context())
	if e != nil || m.Host == "" {
		fail(w, mailUnavailable)
		return
	}
	n, e := rand.Int(rand.Reader, big.NewInt(1000000))
	if e != nil {
		fail(w, e)
		return
	}
	code := fmt.Sprintf("%06d", n.Int64())
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	id, e := s.guest.issue(x.Post, address, ip, code)
	if e != nil {
		fail(w, e)
		return
	}
	words := s.catalog[x.Lang]
	if words == nil {
		words = s.catalog["en"]
	}
	send := s.guest.send
	if send == nil {
		send = sendCommentMail
	}
	if e = send(r.Context(), m, address, words["comment_mail_subject"], strings.ReplaceAll(words["comment_mail_body"], "{code}", code)); e != nil {
		s.guest.mu.Lock()
		delete(s.guest.challenges, id)
		s.guest.mu.Unlock()
		fail(w, mailUnavailable)
		return
	}
	respond(w, map[string]string{"challenge": id})
}
