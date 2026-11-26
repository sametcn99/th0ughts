package feed_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/storage"
)

func TestPublishingMatrix(t *testing.T) {
	drivers := []string{"sqlite", "postgres", "mysql", "mariadb", "mongodb", "json", "json-dir"}
	if storage.NativeKVEnabled {
		drivers = append(drivers, "redb", "lmdb")
	}
	for _, driver := range drivers {
		t.Run(driver, func(t *testing.T) {
			addr := os.Getenv("TEST_" + driver)
			if local := storage.LocalAddress(t.TempDir(), driver); local != "" {
				addr = local
			} else if addr == "" {
				t.Skip("provide a disposable database")
			}
			ctx := context.Background()
			db, e := storage.Open(ctx, storage.Config{Driver: driver, Address: addr})
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			s := feed.New(db)
			a := feed.Actor{Scopes: []string{"posts:write", "posts:read"}}
			key := feed.ID(10)
			p, e := s.Save(ctx, a, feed.Post{Original: "tr", Tags: []string{key}, Text: map[string]feed.Translation{"tr": {Body: "IĞDIR " + key, Published: true}, "en": {Body: "private " + key}}})
			if e != nil {
				t.Fatal(e)
			}
			defer s.Delete(ctx, a, p.ID)
			pub, e := s.Get(ctx, feed.Actor{}, p.ID, false)
			if e != nil || len(pub.Text) != 1 {
				t.Fatal(pub, e)
			}
			rows, e := s.List(ctx, feed.Actor{}, feed.Filter{Search: "ığdır " + key, Lang: "tr", Tag: key})
			if e != nil || len(rows.Posts) != 1 {
				t.Fatal(rows, e)
			}
			var wg sync.WaitGroup
			results := make(chan error, 2)
			for range 2 {
				wg.Add(1)
				go func() { defer wg.Done(); _, e := s.Save(ctx, a, p); results <- e }()
			}
			wg.Wait()
			close(results)
			ok, conflicts := 0, 0
			for e := range results {
				if e == nil {
					ok++
				} else if errors.Is(e, feed.Conflict) {
					conflicts++
				} else {
					t.Fatal(e)
				}
			}
			if ok != 1 || conflicts != 1 {
				t.Fatal(ok, conflicts)
			}
			if e = s.Delete(ctx, feed.Actor{}, p.ID); !errors.Is(e, feed.Denied) {
				t.Fatal(e)
			}
		})
	}
}
func TestTOTPAndRevocation(t *testing.T) {
	ctx := context.Background()
	db, e := storage.Open(ctx, storage.Config{Driver: "json", Address: filepath.Join(t.TempDir(), "store")})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	v, _ := feed.NewVault(feed.ID(32))
	auth := feed.NewAuth(db, v)
	pwd := "sample-test-password"
	if e = auth.Setup(ctx, pwd, false); e != nil {
		t.Fatal(e)
	}
	raw, e := auth.Session(ctx)
	if e != nil {
		t.Fatal(e)
	}
	a := auth.Resolve(ctx, "session", raw)
	k, e := totp.Generate(totp.GenerateOpts{Issuer: "test", AccountName: "owner"})
	if e != nil {
		t.Fatal(e)
	}
	code, _ := totp.GenerateCode(k.Secret(), time.Now())
	codes, e := auth.EnableOTP(ctx, a, k.Secret(), code)
	if e != nil {
		t.Fatal(e)
	}
	if e = auth.Check(ctx, pwd, ""); e == nil {
		t.Fatal("missing second factor accepted")
	}
	if e = auth.Check(ctx, pwd, code); e != nil {
		t.Fatal("first TOTP login", e)
	}
	if e = auth.Check(ctx, pwd, code); !errors.Is(e, feed.Denied) {
		t.Fatal("replayed TOTP accepted", e)
	}
	if e = auth.Check(ctx, pwd, codes[0]); e != nil {
		t.Fatal(e)
	}
	if e = auth.Check(ctx, pwd, codes[0]); e == nil {
		t.Fatal("recovery code reused")
	}
	key, e := auth.Token(ctx, a, feed.Access{Name: "writer", Scopes: []string{"posts:write"}, Expires: time.Now().Add(time.Hour)})
	if e != nil {
		t.Fatal(e)
	}
	if !auth.Resolve(ctx, "token", key).Can("posts:write") {
		t.Fatal("valid token refused")
	}
	if e = db.Delete(ctx, "token", feed.Digest(key)); e != nil {
		t.Fatal(e)
	}
	if auth.Resolve(ctx, "token", key).Can("posts:write") {
		t.Fatal("revoked token accepted")
	}
	if e = auth.Setup(ctx, "new-test-password", true); e != nil {
		t.Fatal(e)
	}
	if auth.Resolve(ctx, "session", raw).Owner {
		t.Fatal("session survived recovery")
	}
}
func TestS3RoundTrip(t *testing.T) {
	endpoint := os.Getenv("TEST_S3")
	if endpoint == "" {
		t.Skip("provide a local S3-compatible service with bucket thoughts-test")
	}
	ctx := context.Background()
	db, e := storage.Open(ctx, storage.Config{Driver: "json-dir", Address: filepath.Join(t.TempDir(), "store")})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	v, _ := feed.NewVault(feed.ID(32))
	s := feed.New(db)
	l := feed.Library{Service: s, Dir: t.TempDir(), Vault: v}
	a := feed.Actor{Owner: true, Fresh: time.Now()}
	e = l.Configure(ctx, a, feed.S3{Endpoint: endpoint, Bucket: "thoughts-test", Access: os.Getenv("TEST_S3_ACCESS"), Secret: os.Getenv("TEST_S3_SECRET")})
	if e != nil {
		t.Fatal(e)
	}
	b, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+X2ZkAAAAASUVORK5CYII=")
	m, e := l.Upload(ctx, a, bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = l.Open(ctx, feed.Actor{}, m.ID); !errors.Is(e, storage.ErrMissing) {
		t.Fatal("unpublished attachment accessible")
	}
	_, f, e := l.Open(ctx, a, m.ID)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	out, e := io.ReadAll(f)
	if e != nil || !bytes.Equal(out, b) {
		t.Fatal("S3 content mismatch", e)
	}
}

func TestSearchCursorAfterPublicationChanges(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, storage.Config{Driver: "json-dir", Address: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := feed.New(db)
	actor := feed.Actor{Owner: true}
	for _, id := range []string{"a", "b", "c", "d"} {
		p := feed.Post{ID: id, Revision: 1, Created: 1, Original: "en", Text: map[string]feed.Translation{"en": {Body: "Search me", Published: true}}}
		if err := feed.Write(ctx, db, "post", id, p, 1, true); err != nil {
			t.Fatal(err)
		}
	}
	filter := feed.Filter{Search: "search", Limit: 2}
	check := func(f feed.Filter, ids string) feed.Page {
		t.Helper()
		page, e := s.List(ctx, feed.Actor{}, f)
		if e != nil {
			t.Fatal(e)
		}
		var got string
		for _, p := range page.Posts {
			got += p.ID
		}
		if got != ids {
			t.Fatalf("got %s, want %s", got, ids)
		}
		return page
	}
	page := check(filter, "dc")
	next := filter
	next.Cursor = page.Next
	if check(next, "ba").Next != "" {
		t.Fatal("unexpected next cursor")
	}
	p, e := s.Get(ctx, actor, "a", true)
	if e != nil {
		t.Fatal(e)
	}
	p.Pinned = true
	p, e = s.Save(ctx, actor, p)
	if e != nil {
		t.Fatal(e)
	}
	check(filter, "ad")
	p.Text["en"] = feed.Translation{Body: "hidden"}
	if _, e = s.Save(ctx, actor, p); e != nil {
		t.Fatal(e)
	}
	if e = s.Delete(ctx, actor, "c"); e != nil {
		t.Fatal(e)
	}
	if check(filter, "db").Next != "" {
		t.Fatal("stale search index")
	}
}
