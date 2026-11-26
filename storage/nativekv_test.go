//go:build nativekv && cgo

package storage_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/sametcn99/th0ughts/storage"
)

func TestNativeOrderAndVisibility(t *testing.T) {
	for _, driver := range []string{"redb", "lmdb"} {
		t.Run(driver, func(t *testing.T) {
			ctx := context.Background()
			db, err := storage.Open(ctx, storage.Config{Driver: driver, Address: filepath.Join(t.TempDir(), "data")})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			rows := []storage.Record{
				{ID: "a0", Rank: math.MinInt64, Published: true},
				{ID: "a1", Rank: -1, Published: true},
				{ID: "a2", Rank: -1, Published: false},
				{ID: "a3", Rank: math.MaxInt64, Published: true},
				{ID: "b1", Rank: math.MaxInt64, Published: true},
			}
			for i := range rows {
				rows[i].Kind = "one"
				rows[i].Data = json.RawMessage(`{"value":true}`)
				if err := db.Put(ctx, rows[i]); err != nil {
					t.Fatal(err)
				}
				other := rows[i]
				other.Kind = "two"
				if err := db.Put(ctx, other); err != nil {
					t.Fatal(err)
				}
			}
			// Include negative ranks, ties and a page boundary at each sign extreme.
			for _, q := range []storage.Query{
				{Public: true, Prefix: "a"}, {Unpublished: true}, {Limit: 2},
				{Before: &storage.Cursor{Rank: -1, ID: "a2"}}, {Before: &storage.Cursor{Rank: math.MinInt64, ID: "a0"}},
			} {
				q.Kind = "one"
				var expected []string
				for _, id := range []int{4, 3, 2, 1, 0} {
					r := rows[id]
					if q.Public && !r.Published || q.Unpublished && r.Published || q.Prefix != "" && r.ID[0] != 'a' {
						continue
					}
					if c := q.Before; c != nil && (r.Rank > c.Rank || r.Rank == c.Rank && r.ID >= c.ID) {
						continue
					}
					expected = append(expected, r.ID)
					if len(expected) == q.Limit {
						break
					}
				}
				got, err := db.List(ctx, q)
				if err != nil {
					t.Fatal(err)
				}
				var ids []string
				for _, r := range got {
					ids = append(ids, r.ID)
				}
				if !reflect.DeepEqual(ids, expected) {
					t.Fatalf("%+v: %v, want %v", q, ids, expected)
				}
			}
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					r := rows[1]
					for n := 0; n < 16; n++ {
						r.Rank = int64(i*16 + n)
						if err := db.Put(ctx, r); err != nil {
							t.Error(err)
							return
						}
						page, err := db.List(ctx, storage.Query{Kind: "one", Prefix: r.ID})
						if err != nil || len(page) != 1 {
							t.Errorf("non-atomic rank update: %+v, %v", page, err)
							return
						}
					}
				}(i)
			}
			wg.Wait()
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			if err := db.Delete(canceled, "one", "a0"); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if _, err := db.Get(ctx, "one", "a0"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
