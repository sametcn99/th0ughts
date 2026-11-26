package storage_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sametcn99/th0ughts/storage"
)

// Measure an indexed first page and a durable update with the same fixture size.
// For external drivers TEST_* must name a disposable database.
func BenchmarkStore(b *testing.B) {
	for _, d := range []string{"redb", "lmdb", "sqlite", "postgres", "mysql", "mariadb", "mongodb", "json", "json-dir"} {
		b.Run(d, func(b *testing.B) {
			if (d == "redb" || d == "lmdb") && !storage.NativeKVEnabled {
				b.Skip("nativekv build required")
			}
			addr := os.Getenv("TEST_" + d)
			if storage.LocalAddress(".", d) != "" {
				addr = filepath.Join(b.TempDir(), "bench")
			} else if addr == "" {
				b.Skip("no disposable database")
			}
			ctx := context.Background()
			s, e := storage.Open(ctx, storage.Config{Driver: d, Address: addr})
			if e != nil {
				b.Fatal(e)
			}
			defer s.Close()
			const kind = "storage_bench"
			payload := json.RawMessage(`{"en":"A small benchmark payload","tr":"Küçük bir ölçüm kaydı"}`)
			for i := 0; i < 500; i++ {
				e = s.Put(ctx, storage.Record{Kind: kind, ID: fmt.Sprintf("%04d", i), Rank: int64(i), Published: true, Data: payload})
				if e != nil {
					b.Fatal(e)
				}
			}
			b.Run("page20", func(b *testing.B) {
				for b.Loop() {
					rows, e := s.List(ctx, storage.Query{Kind: kind, Limit: 20, Public: true})
					if e != nil || len(rows) != 20 {
						b.Fatal(e)
					}
				}
			})
			b.Run("update", func(b *testing.B) {
				for b.Loop() {
					if e := s.Put(ctx, storage.Record{Kind: kind, ID: "0000", Rank: 0, Published: true, Data: payload}); e != nil {
						b.Fatal(e)
					}
				}
			})
			for i := 0; i < 500; i++ {
				if e = s.Delete(ctx, kind, fmt.Sprintf("%04d", i)); e != nil {
					b.Fatal(e)
				}
			}
		})
	}
}
