package feed

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/sametcn99/th0ughts/storage"
	theme "github.com/sametcn99/th0ughts/themes"
)

type Version struct {
	Source   string   `json:"source"`
	Previous string   `json:"previous,omitempty"`
	History  []string `json:"history,omitempty"` // Older than Previous, newest first.
}
type siteRecord struct {
	Settings
	Refs map[string][]string `json:"_themes,omitempty"`
}
type Themes struct {
	Version atomic.Uint64
	DB      storage.Store
	built   map[string]string
	mu      sync.Mutex
	cache   map[string]theme.Theme
}

func NewThemes(db storage.Store) *Themes {
	return &Themes{DB: db, built: theme.Builtins(), cache: map[string]theme.Theme{}}
}
func (t *Themes) record(ctx context.Context) (siteRecord, error) {
	rec, e := Read[siteRecord](ctx, t.DB, "site", "settings")
	if errors.Is(e, storage.ErrMissing) {
		rec.Settings = Settings{Name: "th0ughts", Lang: "en", Theme: "paper", Options: map[string]map[string]string{}}
		e = nil
	}
	if rec.Refs == nil {
		rec.Refs = map[string][]string{}
	}
	if rec.Comments.Mode == "" {
		rec.Comments = CommentPolicy{Mode: "off", Markdown: true, Images: 4}
	}
	rec.Settings.refs = rec.Refs
	return rec, e
}
func (t *Themes) legacy(ctx context.Context, id string) (Version, error) {
	v, e := Read[Version](ctx, t.DB, "theme", id)
	if errors.Is(e, storage.ErrMissing) {
		if b, ok := t.built[id]; ok {
			return Version{Source: b}, nil
		}
	}
	return v, e
}
func (t *Themes) sourceAt(ctx context.Context, id string, refs map[string][]string) (Version, error) {
	hashes, ok := refs[id]
	if !ok {
		return t.legacy(ctx, id)
	}
	if len(hashes) == 0 {
		return Version{}, storage.ErrMissing
	}
	var v Version
	for i, h := range hashes {
		src, e := Read[string](ctx, t.DB, "theme-source", h)
		if e != nil {
			return Version{}, e
		}
		switch i {
		case 0:
			v.Source = src
		case 1:
			v.Previous = src
		default:
			v.History = append(v.History, src)
		}
	}
	return v, nil
}
func (t *Themes) Source(ctx context.Context, id string) (Version, error) {
	rec, e := t.record(ctx)
	if e != nil {
		return Version{}, e
	}
	return t.sourceAt(ctx, id, rec.Refs)
}
func (t *Themes) load(ctx context.Context, id string, refs map[string][]string) (theme.Theme, error) {
	key := "legacy/" + id
	hashes, ok := refs[id]
	if ok {
		if len(hashes) == 0 {
			return theme.Theme{}, storage.ErrMissing
		}
		key = hashes[0]
	}
	if v, ok := t.cache[key]; ok {
		return v, nil
	}
	var src string
	var e error
	if ok {
		src, e = Read[string](ctx, t.DB, "theme-source", key)
	} else {
		var v Version
		v, e = t.legacy(ctx, id)
		src = v.Source
	}
	if e != nil {
		return theme.Theme{}, e
	}
	v, e := theme.Compile(src)
	if e == nil {
		if len(t.cache) >= 128 {
			clear(t.cache)
		}
		t.cache[key] = v
	}
	return v, e
}
func (t *Themes) LoadAt(ctx context.Context, id string, site Settings) (theme.Theme, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.load(ctx, id, site.refs)
}
func (t *Themes) Load(ctx context.Context, id string) (theme.Theme, error) {
	rec, e := t.record(ctx)
	if e != nil {
		return theme.Theme{}, e
	}
	return t.LoadAt(ctx, id, rec.Settings)
}
func (t *Themes) ids(ctx context.Context, refs map[string][]string, built bool) ([]string, error) {
	rows, e := t.DB.List(ctx, storage.Query{Kind: "theme"})
	if e != nil {
		return nil, e
	}
	ids := map[string]bool{}
	if built {
		for id := range t.built {
			ids[id] = true
		}
	}
	for _, r := range rows {
		ids[r.ID] = true
	}
	for id, h := range refs {
		ids[id] = len(h) > 0
	}
	var out []string
	for id, ok := range ids {
		if ok {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out, nil
}
func (t *Themes) List(ctx context.Context) ([]theme.Theme, error) {
	rec, e := t.record(ctx)
	if e != nil {
		return nil, e
	}
	ids, e := t.ids(ctx, rec.Refs, true)
	if e != nil {
		return nil, e
	}
	var out []theme.Theme
	for _, id := range ids {
		v, e := t.LoadAt(ctx, id, rec.Settings)
		if e != nil {
			return nil, e
		}
		if hashes, ok := rec.Refs[id]; ok {
			v.Revisions = len(hashes) - 1
		} else {
			previous, e := t.legacy(ctx, id)
			if e != nil {
				return nil, e
			}
			v.Revisions = len(sources(previous)) - 1
		}
		_, v.Builtin = t.built[id]
		out = append(out, v)
	}
	return out, nil
}
func sources(v Version) []string {
	out := []string{v.Source}
	if v.Previous != "" {
		out = append(out, v.Previous)
	}
	return append(out, v.History...)
}

// Sources are immutable. A single record publishes both the new revision and its preferences.
func (t *Themes) stage(ctx context.Context, rec *siteRecord, id, source string) error {
	hashes, ok := rec.Refs[id]
	if !ok {
		v, e := t.legacy(ctx, id)
		if e != nil && !errors.Is(e, storage.ErrMissing) {
			return e
		}
		if e == nil {
			for _, s := range sources(v) {
				h := Digest(s)
				if !slices.Contains(hashes, h) {
					if e = Write(ctx, t.DB, "theme-source", h, s, 0, false); e != nil {
						return e
					}
					hashes = append(hashes, h)
				}
			}
		}
	}
	h := Digest(source)
	if !slices.Contains(hashes, h) {
		if e := Write(ctx, t.DB, "theme-source", h, source, 0, false); e != nil {
			return e
		}
	}
	rec.Refs[id] = append([]string{h}, slices.DeleteFunc(hashes, func(s string) bool { return s == h })...)
	return nil
}
func (t *Themes) commit(ctx context.Context, rec siteRecord) error {
	if e := Write(ctx, t.DB, "site", "settings", rec, 0, true); e != nil {
		return e
	}
	t.Version.Add(1)
	return nil
}
func (t *Themes) Publish(ctx context.Context, a Actor, prefs Settings, src string) error {
	if !a.Can("settings") || src != "" && !a.Can("themes") {
		return Denied
	}
	if e := validateSettings(&prefs); e != nil {
		return e
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	rec, e := t.record(ctx)
	if e != nil {
		return e
	}
	if prefs.Comments.Mode == "" {
		prefs.Comments = rec.Comments
	}
	var design theme.Theme
	if src != "" {
		design, e = theme.Compile(src)
		if e != nil {
			return e
		}
		if design.ID != prefs.Theme {
			return Invalid
		}
	} else {
		design, e = t.load(ctx, prefs.Theme, rec.Refs)
		if e != nil {
			return e
		}
	}
	for id, values := range prefs.Options {
		schema := design
		if id != design.ID {
			schema, e = t.load(ctx, id, rec.Refs)
			if e != nil {
				return e
			}
		}
		if key := schema.InvalidOption(values); key != "" {
			return fmt.Errorf("%w: %s/%s", Invalid, id, key)
		}
	}
	if src != "" {
		if e = t.stage(ctx, &rec, design.ID, src); e != nil {
			return e
		}
	}
	rec.Settings = prefs
	return t.commit(ctx, rec)
}
func (t *Themes) Install(ctx context.Context, a Actor, source, rollback string) (theme.Theme, error) {
	if !a.Can("themes") {
		return theme.Theme{}, Denied
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	rec, e := t.record(ctx)
	if e != nil {
		return theme.Theme{}, e
	}
	if rollback != "" {
		v, e := t.sourceAt(ctx, rollback, rec.Refs)
		if e != nil {
			return theme.Theme{}, e
		}
		if v.Previous == "" {
			return theme.Theme{}, Invalid
		}
		source = v.Previous
	}
	v, e := theme.Compile(source)
	if e != nil {
		return v, e
	}
	if e = t.stage(ctx, &rec, v.ID, source); e != nil {
		return v, e
	}
	v.Normalize(rec.Options[v.ID])
	return v, t.commit(ctx, rec)
}
func (t *Themes) Delete(ctx context.Context, a Actor, id string) error {
	if !a.Can("themes") {
		return Denied
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	rec, e := t.record(ctx)
	if e != nil {
		return e
	}
	if _, ok := t.built[id]; ok || id == rec.Theme {
		return Conflict
	}
	if _, e = t.load(ctx, id, rec.Refs); e != nil {
		return e
	}
	rec.Refs[id] = []string{}
	delete(rec.Options, id)
	return t.commit(ctx, rec)
}
func (t *Themes) clearCache() { t.mu.Lock(); defer t.mu.Unlock(); clear(t.cache) }

// Earlier releases accepted arbitrary option maps. Discard stale overrides when reading them.
func (t *Themes) normalize(ctx context.Context, site *Settings) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, values := range site.Options {
		v, e := t.load(ctx, id, site.refs)
		if errors.Is(e, storage.ErrMissing) {
			delete(site.Options, id)
			continue
		}
		if e != nil {
			return e
		}
		v.Normalize(values)
	}
	return nil
}

// Reclaim removes source blobs left by interrupted publications or deleted themes.
// Call it while opening the single-writer site, before serving snapshot readers.
func (t *Themes) Reclaim(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	rec, e := t.record(ctx)
	if e != nil {
		return e
	}
	keep := map[string]bool{}
	for _, hashes := range rec.Refs {
		for _, h := range hashes {
			keep[h] = true
		}
	}
	q := storage.Query{Kind: "theme-source", Limit: 64}
	for {
		rows, e := t.DB.List(ctx, q)
		if e != nil {
			return e
		}
		for _, r := range rows {
			if !keep[r.ID] {
				if e = t.DB.Delete(ctx, r.Kind, r.ID); e != nil {
					return e
				}
			}
		}
		if len(rows) < q.Limit {
			break
		}
		last := rows[len(rows)-1]
		q.Before = &storage.Cursor{Rank: last.Rank, ID: last.ID}
	}
	if len(rec.Refs) > 0 {
		legacy, e := t.DB.List(ctx, storage.Query{Kind: "theme"})
		if e != nil {
			return e
		}
		for _, r := range legacy {
			if _, superseded := rec.Refs[r.ID]; superseded {
				if e = t.DB.Delete(ctx, "theme", r.ID); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
