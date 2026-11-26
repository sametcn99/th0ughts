package feed

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sametcn99/th0ughts/storage"
)

type failingPublication struct {
	storage.Store
	fail bool
}

func (s *failingPublication) Put(ctx context.Context, r storage.Record) error {
	if s.fail && r.Kind == "site" {
		return errors.New("publication interrupted")
	}
	return s.Store.Put(ctx, r)
}
func TestAtomicThemePublication(t *testing.T) {
	for _, driver := range []string{"sqlite", "json", "json-dir"} {
		t.Run(driver, func(t *testing.T) {
			ctx := context.Background()
			open := func() *failingPublication {
				db, e := storage.Open(ctx, storage.Config{Driver: driver, Address: filepath.Join(t.TempDir(), "db")})
				if e != nil {
					t.Fatal(e)
				}
				t.Cleanup(func() { db.Close() })
				return &failingPublication{Store: db}
			}
			db := open()
			s := New(db)
			a := Actor{Owner: true, Fresh: time.Now()}
			makeSource := func(name string) string {
				return `<meta name="theme" content="custom" title="` + name + `"><script type="application/json" id="options">[{"key":"gap","type":"range","min":0,"max":2,"step":0.25,"default":"0.5","labels":{"en":"Gap"}}]</script>`
			}
			prefs, e := s.Settings(ctx)
			if e != nil {
				t.Fatal(e)
			}
			prefs.Theme = "custom"
			prefs.Options = map[string]map[string]string{"custom": {"gap": "0.75"}}
			publish := func(name string) {
				t.Helper()
				if e := s.Themes.Publish(ctx, a, prefs, makeSource(name)); e != nil {
					t.Fatal(e)
				}
			}
			publish("one")
			snapshot, e := s.Settings(ctx)
			if e != nil {
				t.Fatal(e)
			}
			prefs.Name = "new name"
			db.fail = true
			if s.Themes.Publish(ctx, a, prefs, makeSource("two")) == nil {
				t.Fatal("expected write failure")
			}
			db.fail = false
			// A fresh service reads only the committed preferences and sources.
			restarted := New(db)
			got, e := restarted.Settings(ctx)
			if e != nil || got.Name != snapshot.Name {
				t.Fatal(got, e)
			}
			old, e := restarted.Themes.Load(ctx, "custom")
			if e != nil || old.Name != "one" {
				t.Fatal(old, e)
			}
			prefs.Options["custom"]["gap"] = "0.8"
			if e = s.SaveSettings(ctx, a, prefs); !errors.Is(e, Invalid) {
				t.Fatal("off-step value", e)
			}
			prefs.Options["custom"]["gap"] = "0.75"
			if e = s.Themes.Publish(ctx, Actor{Scopes: []string{"settings"}}, prefs, makeSource("two")); !errors.Is(e, Denied) {
				t.Fatal("missing themes scope", e)
			}
			publish("two")
			publish("three")
			publish("three")
			old, e = s.Themes.LoadAt(ctx, "custom", snapshot)
			if e != nil || old.Name != "one" {
				t.Fatal("mixed reader snapshot", old, e)
			}
			history, e := s.Themes.Source(ctx, "custom")
			if e != nil || history.Previous != makeSource("two") || len(history.History) != 1 || history.History[0] != makeSource("one") {
				t.Fatal(history, e)
			}
			var archive bytes.Buffer
			lib := Library{Service: s, Dir: t.TempDir()}
			if e = lib.Export(ctx, a, &archive); e != nil {
				t.Fatal(e)
			}
			z, e := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
			if e != nil {
				t.Fatal(e)
			}
			imported := Library{Service: New(open()), Dir: t.TempDir()}
			if e = imported.Import(ctx, a, z); e != nil {
				t.Fatal(e)
			}
			h, e := imported.Service.Themes.Source(ctx, "custom")
			if e != nil || len(h.History) != 1 || h.History[0] != makeSource("one") {
				t.Fatal(h, e)
			}
			if _, e = s.Themes.Install(ctx, a, history.History[0], ""); e != nil {
				t.Fatal(e)
			}
			if e = s.Themes.Delete(ctx, a, "custom"); !errors.Is(e, Conflict) {
				t.Fatal(e)
			}
			prefs.Theme = "paper"
			if e = s.SaveSettings(ctx, a, prefs); e != nil {
				t.Fatal(e)
			}
			if e = s.Themes.Delete(ctx, a, "paper"); !errors.Is(e, Conflict) {
				t.Fatal(e)
			}
			if e = s.Themes.Delete(ctx, a, "custom"); e != nil {
				t.Fatal(e)
			}
			if _, e = New(db).Themes.Load(ctx, "custom"); !errors.Is(e, storage.ErrMissing) {
				t.Fatal(e)
			}
		})
	}
}
