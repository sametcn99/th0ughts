// Package feed implements publishing and visibility independently of transport.
package feed

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sametcn99/th0ughts/storage"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var Denied = errors.New("forbidden")
var Invalid = errors.New("invalid_input")
var Conflict = errors.New("conflict")

type Actor struct {
	Owner   bool
	Scopes  []string
	Fresh   time.Time
	Session string
}

func (a Actor) Can(scope string) bool {
	if a.Owner {
		return true
	}
	for _, s := range a.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}
func (a Actor) Recent() bool { return a.Owner && time.Since(a.Fresh) < 10*time.Minute }

type Translation struct {
	Cover       string            `json:"cover,omitempty"`
	Media       []string          `json:"media,omitempty"`
	Body        string            `json:"body"`
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Alt         map[string]string `json:"alt,omitempty"`
	Published   bool              `json:"published"`
}
type Post struct {
	CommentsClosed bool                   `json:"commentsClosed,omitempty"`
	ID             string                 `json:"id,omitempty"`
	Original       string                 `json:"original"`
	Text           map[string]Translation `json:"text"`
	Tags           []string               `json:"tags,omitempty"`
	Media          []string               `json:"media,omitempty"`
	Pinned         bool                   `json:"pinned,omitempty"`
	Created        int64                  `json:"created,omitempty"`
	Updated        int64                  `json:"updated,omitempty"`
	Revision       int                    `json:"revision,omitempty"`
}
type Settings struct {
	Comments    CommentPolicy `json:"comments"`
	refs        map[string][]string
	Name        string                       `json:"name"`
	Description string                       `json:"description,omitempty"`
	Author      string                       `json:"author"`
	URL         string                       `json:"url"`
	Cover       string                       `json:"cover"`
	Lang        string                       `json:"lang"`
	Theme       string                       `json:"theme"`
	Options     map[string]map[string]string `json:"options"`
}
type Service struct {
	Themes       *Themes
	DB           storage.Store
	Version      atomic.Uint64
	mu           sync.RWMutex
	hookWake     chan struct{}
	hookDelivery sync.Mutex
	publicMedia  publicMediaCache
	index        *searchIndex
}

func New(db storage.Store) *Service {
	return &Service{DB: db, Themes: NewThemes(db), hookWake: make(chan struct{}, 1)}
}
func ID(n int) string { b := make([]byte, n); rand.Read(b); return hex.EncodeToString(b) }
func Read[T any](ctx context.Context, db storage.Store, kind, id string) (T, error) {
	var v T
	r, e := db.Get(ctx, kind, id)
	if e == nil {
		e = json.Unmarshal(r.Data, &v)
	}
	return v, e
}
func Write(ctx context.Context, db storage.Store, k, id string, v any, rank int64, pub bool) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return db.Put(ctx, storage.Record{Kind: k, ID: id, Data: b, Rank: rank, Published: pub})
}
func Locale(s string) bool { return s == "en" || s == "tr" }
func (s *Service) Settings(ctx context.Context) (Settings, error) {
	rec, e := s.Themes.record(ctx)
	if e == nil {
		e = s.Themes.normalize(ctx, &rec.Settings)
	}
	return rec.Settings, e
}
func (s *Service) SaveSettings(ctx context.Context, a Actor, v Settings) error {
	e := s.Themes.Publish(ctx, a, v, "")
	if e == nil {
		s.Version.Add(1)
	}
	return e
}
func validateSettings(v *Settings) error {
	if !v.Comments.valid() {
		return Invalid
	}
	if !Locale(v.Lang) || v.Name == "" || !storage.ValidKey(v.Theme) || len(v.Name) > 150 || len(v.Description) > 1000 {
		return Invalid
	}
	if v.Cover != "" {
		u, e := url.Parse(v.Cover)
		if e != nil || u.User != nil || !(u.Scheme == "https" && u.Host != "" || u.Host == "" && u.Scheme == "" && strings.HasPrefix(v.Cover, "/") && !strings.HasPrefix(v.Cover, "//")) {
			return Invalid
		}
	}
	if v.URL != "" {
		u, e := url.Parse(v.URL)
		if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
			return Invalid
		}
		v.URL = strings.TrimRight(v.URL, "/")
	}
	return nil
}

// Attachments returns shared media followed by this translation's attachments.
func (p Post) Attachments(t Translation) []string {
	ids := append([]string(nil), p.Media...)
	return append(ids, t.Media...)
}

// AllMedia only includes translations present in the projection.
func (p Post) AllMedia() []string {
	ids := append([]string(nil), p.Media...)
	for _, t := range p.Text {
		ids = append(ids, t.Media...)
	}
	return ids
}
func (p Post) Visible() bool { return p.Text[p.Original].Published }
func (p Post) Public() (Post, error) {
	if !p.Visible() {
		return Post{}, storage.ErrMissing
	}
	texts := map[string]Translation{}
	for k, t := range p.Text {
		if t.Published {
			texts[k] = t
		}
	}
	p.Text = texts
	return p, nil
}
func (p Post) Select(lang string) (string, Translation) {
	if t, ok := p.Text[lang]; ok {
		return lang, t
	}
	return p.Original, p.Text[p.Original]
}
func (s *Service) Get(ctx context.Context, a Actor, id string, private bool) (Post, error) {
	if private && !a.Can("posts:read") {
		return Post{}, Denied
	}
	p, e := Read[Post](ctx, s.DB, "post", id)
	if e != nil {
		return p, e
	}
	if !private {
		return p.Public()
	}
	return p, nil
}
func (s *Service) Save(ctx context.Context, a Actor, p Post) (Post, error) {
	if !a.Can("posts:write") {
		return Post{}, Denied
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !Locale(p.Original) || len(p.Text) == 0 || len(p.Tags) > 20 || len(p.AllMedia()) > 20 {
		return Post{}, Invalid
	}
	if _, ok := p.Text[p.Original]; !ok {
		return Post{}, Invalid
	}
	for l, t := range p.Text {
		if t.Cover != "" && !slices.Contains(p.Attachments(t), t.Cover) {
			return Post{}, Invalid
		}
		if !Locale(l) || len(t.Body) > 128000 || len(t.Title) > 250 || len(t.Description) > 1000 || strings.TrimSpace(t.Body) == "" && len(p.Attachments(t)) == 0 {
			return Post{}, Invalid
		}
	}
	for _, tag := range p.Tags {
		if tag == "" || len(tag) > 80 {
			return Post{}, Invalid
		}
	}
	for _, id := range p.AllMedia() {
		m, e := Read[Media](ctx, s.DB, "media", id)
		if m.Comment != "" {
			return Post{}, Invalid
		}
		if e == nil && m.Post != "" && m.Post != p.ID {
			_, claimErr := s.DB.Get(ctx, "post", m.Post)
			if !errors.Is(claimErr, storage.ErrMissing) {
				return Post{}, Invalid
			}
		}
		if e != nil {
			return Post{}, Invalid
		}
	}
	now := time.Now().UTC().UnixMilli()
	if p.ID == "" {
		p.ID = ID(12)
		p.Created = now
		p.Revision = 1
	} else {
		old, e := Read[Post](ctx, s.DB, "post", p.ID)
		if e != nil {
			return Post{}, e
		}
		if p.Revision != old.Revision {
			return Post{}, Conflict
		}
		p.Created = old.Created
		p.Revision++
	}
	p.Updated = now
	// Claim attachments before publishing. Visibility always checks the live post.
	for _, id := range p.AllMedia() {
		m, e := Read[Media](ctx, s.DB, "media", id)
		if e != nil {
			return Post{}, e
		}
		if m.Post != p.ID {
			m.Post = p.ID
			if e = Write(ctx, s.DB, "media", id, m, 0, false); e != nil {
				return Post{}, e
			}
		}
	}
	rank := p.Created
	if p.Pinned {
		rank += 1 << 54
	}
	e := Write(ctx, s.DB, "post", p.ID, p, rank, p.Visible())
	if e == nil {
		s.updateIndex(p)
		s.Version.Add(1)
	}
	return p, e
}
func (s *Service) Delete(ctx context.Context, a Actor, id string) error {
	return s.deletePost(ctx, a, id, nil)
}
func (s *Service) deletePost(ctx context.Context, a Actor, id string, discard func(context.Context, []string) error) error {
	if !a.Can("posts:write") {
		return Denied
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !storage.ValidKey(id) {
		return Invalid
	}
	if e := s.resolveHookIntent(ctx); e != nil {
		return e
	}
	if e := Write(ctx, s.DB, "thread_gc", id, true, 0, false); e != nil {
		return e
	}
	e := s.DB.Delete(ctx, "post", id)
	if e == nil {
		if s.index != nil {
			s.index.remove(id)
		}
		s.Version.Add(1)
		if e = s.purgePostComments(ctx, id, discard); e != nil {
			return e
		}
		return s.DB.Delete(ctx, "thread_gc", id)
	}
	return e
}

type Filter struct {
	Lang    string `json:"lang,omitempty"`
	Search  string `json:"search,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Cursor  string `json:"cursor,omitempty"`
	Private bool   `json:"private,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}
type Page struct {
	Posts []Post `json:"posts"`
	Next  string `json:"next,omitempty"`
}

func cursor(r storage.Record) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d/%s", r.Rank, r.ID)))
}
func parseCursor(value string) (*storage.Cursor, error) {
	b, e := base64.RawURLEncoding.DecodeString(value)
	if e != nil {
		return nil, Invalid
	}
	r, id, ok := strings.Cut(string(b), "/")
	rank, e := strconv.ParseInt(r, 10, 64)
	if !ok || e != nil || !storage.ValidKey(id) {
		return nil, Invalid
	}
	return &storage.Cursor{Rank: rank, ID: id}, nil
}
func (s *Service) List(ctx context.Context, a Actor, f Filter) (Page, error) {
	out := Page{Posts: []Post{}}
	if f.Private && !a.Can("posts:read") {
		return out, Denied
	}
	q := storage.Query{Kind: "post", Public: !f.Private}
	if f.Cursor != "" {
		var e error
		q.Before, e = parseCursor(f.Cursor)
		if e != nil {
			return out, e
		}
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if !f.Private && (f.Search != "" || f.Tag != "") {
		return s.search(ctx, f, q, limit)
	}
	q.Limit = limit + 1
	lower := cases.Lower(language.Make(f.Lang))
	needle := lower.String(f.Search)
	last := ""
	for {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		rows, e := s.DB.List(ctx, q)
		if e != nil {
			return out, e
		}
		for _, r := range rows {
			var p Post
			if e = json.Unmarshal(r.Data, &p); e != nil {
				return out, e
			}
			q.Before = &storage.Cursor{Rank: r.Rank, ID: r.ID}
			if !f.Private {
				p, e = p.Public()
				if e != nil {
					continue
				}
			}
			_, t := p.Select(f.Lang)
			if needle != "" && !strings.Contains(lower.String(t.Body+" "+t.Title), needle) {
				continue
			}
			if f.Tag != "" {
				ok := false
				for _, tag := range p.Tags {
					if tag == f.Tag {
						ok = true
						break
					}
				}
				if !ok {
					continue
				}
			}
			if len(out.Posts) == limit {
				out.Next = last
				return out, nil
			}
			out.Posts = append(out.Posts, p)
			last = cursor(r)
		}
		if len(rows) < q.Limit {
			return out, nil
		}
	}
}
