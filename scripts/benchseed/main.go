// Command benchseed populates an empty, disposable benchmark instance.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/server"
	"github.com/sametcn99/th0ughts/storage"
)

func main() {
	dir := flag.String("data", "", "Empty benchmark directory")
	driver := flag.String("driver", "sqlite", "Storage driver")
	addr := flag.String("address", "", "Dedicated benchmark database")
	posts := flag.Int("posts", 10000, "Number of posts")
	image := flag.String("image", "", "Image fixture to attach")
	every := flag.Int("image-every", 50, "Attach an image to every Nth post")
	flag.Parse()
	if *posts < 1 || *every < 1 {
		panic("posts and image-every must be positive")
	}
	if *dir == "" {
		panic("provide a fresh benchmark directory")
	}
	if _, e := os.Stat(filepath.Join(*dir, "connection.enc")); !os.IsNotExist(e) {
		panic("benchmark instance must be new")
	}
	if *addr == "" {
		*addr = storage.LocalAddress(*dir, *driver)
	}
	ctx := context.Background()
	s, e := server.Open(ctx, *dir, os.Getenv("TH0UGHTS_MASTER_KEY"), storage.Config{Driver: *driver, Address: *addr})
	must(e)
	defer s.Close()
	for _, k := range []string{"post", "auth", "media", "site", "token"} {
		rows, e := s.S.DB.List(ctx, storage.Query{Kind: k, Limit: 1})
		must(e)
		if len(rows) > 0 {
			panic("database must be empty")
		}
	}
	must(s.A.Setup(ctx, feed.ID(24), false))
	owner := feed.Actor{Owner: true, Fresh: time.Now()}
	token, e := s.A.Token(ctx, owner, feed.Access{Name: "Benchmark", Scopes: []string{"posts:write", "posts:read"}, Expires: time.Now().Add(2 * time.Hour)})
	must(e)
	must(storage.Atomic(filepath.Join(*dir, "bench-token"), []byte(token)))
	sample, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+X2ZkAAAAASUVORK5CYII=")
	if *image != "" {
		sample, e = os.ReadFile(*image)
		must(e)
	}
	for i := 0; i < *posts; i++ {
		lang := []string{"en", "tr"}[i%2]
		p := feed.Post{ID: fmt.Sprintf("seed-%05d", i), Original: lang, Created: 1700000000000 + int64(i), Updated: 1700000000000 + int64(i), Revision: 1, Tags: []string{fmt.Sprintf("tag-%d", i%31)}, Text: map[string]feed.Translation{lang: {Body: fmt.Sprintf("Field note %d. İstanbul, İzmir and the morning light. Notes about **gardens** and software. Group %d.", i, i%127), Published: true}}}
		if i%*every == 0 {
			m, e := s.L.Upload(ctx, owner, bytes.NewReader(sample))
			must(e)
			m.Post = p.ID
			p.Media = []string{m.ID}
			must(feed.Write(ctx, s.S.DB, "media", m.ID, m, 0, false))
		}
		must(feed.Write(ctx, s.S.DB, "post", p.ID, p, p.Created, true))
	}
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}
