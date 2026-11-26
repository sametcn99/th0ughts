package feed

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/sametcn99/th0ughts/storage"
	theme "github.com/sametcn99/th0ughts/themes"
)

type Version struct {
	Source   string `json:"source"`
	Previous string `json:"previous"`
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
func (t *Themes) Source(ctx context.Context, id string) (Version, error) {
	v, e := Read[Version](ctx, t.DB, "theme", id)
	if errors.Is(e, storage.ErrMissing) {
		if b, ok := t.built[id]; ok {
			return Version{Source: b}, nil
		}
	}
	return v, e
}
func (t *Themes) Load(ctx context.Context, id string) (theme.Theme, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.load(ctx, id)
}
func (t *Themes) load(ctx context.Context, id string) (theme.Theme, error) {
	if v, ok := t.cache[id]; ok {
		return v, nil
	}
	v, e := t.Source(ctx, id)
	if e != nil {
		return theme.Theme{}, e
	}
	parsed, e := theme.Compile(v.Source)
	if e == nil {
		t.cache[id] = parsed
	}
	return parsed, e
}
func (t *Themes) List(ctx context.Context) ([]theme.Theme, error) {
	rows, e := t.DB.List(ctx, storage.Query{Kind: "theme"})
	if e != nil {
		return nil, e
	}
	ids := map[string]bool{}
	for k := range t.built {
		ids[k] = true
	}
	for _, r := range rows {
		ids[r.ID] = true
	}
	var items []theme.Theme
	for id := range ids {
		v, e := t.Load(ctx, id)
		if e != nil {
			return nil, e
		}
		items = append(items, v)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}
func (t *Themes) Install(ctx context.Context, a Actor, source string, rollback string) (theme.Theme, error) {
	if !a.Can("themes") {
		return theme.Theme{}, Denied
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if rollback != "" {
		v, e := t.Source(ctx, rollback)
		if e != nil || v.Previous == "" {
			return theme.Theme{}, Invalid
		}
		source = v.Previous
	}
	v, e := theme.Compile(source)
	if e != nil {
		return v, Invalid
	}
	old, e := t.Source(ctx, v.ID)
	if e != nil && !errors.Is(e, storage.ErrMissing) {
		return v, e
	}
	if old.Source != source {
		e = Write(ctx, t.DB, "theme", v.ID, Version{source, old.Source}, 0, false)
		if e != nil {
			return v, e
		}
	}
	t.cache[v.ID] = v
	t.Version.Add(1)
	return v, nil
}
