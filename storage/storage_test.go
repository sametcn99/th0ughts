package storage_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sametcn99/th0ughts/storage"
)

func TestDrivers(t *testing.T) {
	for _, driver := range []string{"sqlite", "json", "json-dir", "postgres", "mysql", "mariadb", "mongodb"} {
		t.Run(driver, func(t *testing.T) {
			addr := os.Getenv("TEST_" + driver)
			switch driver {
			case "sqlite", "json", "json-dir":
				addr = filepath.Join(t.TempDir(), "database")
			default:
				if addr == "" {
					t.Skip("set TEST_" + driver + " to a disposable database")
				}
			}
			ctx := context.Background()
			cfg := storage.Config{Driver: driver, Address: addr}
			s, e := storage.Open(ctx, cfg)
			if e != nil {
				t.Fatal(e)
			}
			for _, id := range []string{"a", "b", "c"} {
				if e = s.Put(ctx, storage.Record{Kind: "contract", ID: id, Rank: 1, Published: id != "b", Data: json.RawMessage(`{"text":"İstanbul"}`)}); e != nil {
					t.Fatal(e)
				}
			}
			rows, e := s.List(ctx, storage.Query{Kind: "contract", Public: true, Limit: 1})
			if e != nil || len(rows) != 1 || rows[0].ID != "c" {
				t.Fatalf("page: %+v %v", rows, e)
			}
			rows, e = s.List(ctx, storage.Query{Kind: "contract", Public: true, Before: &storage.Cursor{Rank: 1, ID: "c"}})
			if e != nil || len(rows) != 1 || rows[0].ID != "a" {
				t.Fatalf("next page: %+v %v", rows, e)
			}
			// A warm ordering index must see changes to both rank and visibility.
			updated := storage.Record{Kind: "contract", ID: "b", Rank: 3, Published: true, Data: json.RawMessage(`{"text":"İstanbul"}`)}
			if e = s.Put(ctx, updated); e != nil {
				t.Fatal(e)
			}
			rows, e = s.List(ctx, storage.Query{Kind: "contract", Public: true, Limit: 1})
			if e != nil || len(rows) != 1 || rows[0].ID != "b" {
				t.Fatalf("updated page: %+v %v", rows, e)
			}
			if e = s.Delete(ctx, "contract", "c"); e != nil {
				t.Fatal(e)
			}
			rows, e = s.List(ctx, storage.Query{Kind: "contract", Before: &storage.Cursor{Rank: 3, ID: "b"}})
			if e != nil || len(rows) != 1 || rows[0].ID != "a" {
				t.Fatalf("page after deletion: %+v %v", rows, e)
			}
			if driver == "json" || driver == "json-dir" {
				if duplicate, e := storage.Open(ctx, cfg); e == nil {
					duplicate.Close()
					t.Fatal("second writer acquired store")
				}
			}
			if e = s.Close(); e != nil {
				t.Fatal(e)
			}
			s, e = storage.Open(ctx, cfg)
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			r, e := s.Get(ctx, "contract", "b")
			if e != nil || string(r.Data) != `{"text":"İstanbul"}` {
				t.Fatalf("reopen: %+v %v", r, e)
			}
			for _, id := range []string{"a", "b", "c"} {
				if e = s.Delete(ctx, "contract", id); e != nil {
					t.Fatal(e)
				}
			}
			if _, e = s.Get(ctx, "contract", "a"); !errors.Is(e, storage.ErrMissing) {
				t.Fatalf("deleted: %v", e)
			}
		})
	}
}
