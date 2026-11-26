package feed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sametcn99/th0ughts/storage"
)

func TestWebhookRecoveryAndRetry(t *testing.T) {
	ctx := context.Background()
	driver := storage.Config{Driver: "json-dir", Address: filepath.Join(t.TempDir(), "data")}
	vault, _ := NewVault(ID(32))
	open := func() *Library {
		db, err := storage.Open(ctx, driver)
		if err != nil {
			t.Fatal(err)
		}
		return &Library{Service: New(db), Vault: vault}
	}
	lib := open()
	defer func() { lib.Service.DB.Close() }()
	owner := Actor{Owner: true, Fresh: time.Now()}
	var bodies [][]byte
	var requests atomic.Int32
	var redirected atomic.Bool
	received := make(chan []byte, 10)
	receiver := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events" {
			redirected.Store(true)
			return
		}
		body, _ := io.ReadAll(r.Body)
		received <- body
		if requests.Add(1) == 1 {
			http.Redirect(w, r, "/other", http.StatusTemporaryRedirect)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer receiver.Close()
	cfg := CommentWebhookSettings{Enabled: true, URL: receiver.URL + "/events", Secret: ID(32)}
	requireOK := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	requireOK(lib.ConfigureCommentWebhook(ctx, owner, cfg))
	c := Comment{ID: "comment1", Status: "pending", Post: "post1", Body: "A private draft"}
	stage := func() { t.Helper(); requireOK(lib.stageCommentEvent(ctx, "comment.created", c)) }
	deliver := func() time.Duration {
		t.Helper()
		d, e := lib.DeliverCommentWebhook(ctx, receiver.Client().Transport)
		requireOK(e)
		for len(received) > 0 {
			bodies = append(bodies, <-received)
		}
		return d
	}
	restart := func() { t.Helper(); requireOK(lib.Service.DB.Close()); lib = open() }
	// Crash between preparing the event and committing its comment.
	stage()
	restart()
	deliver()
	if len(bodies) != 0 {
		t.Fatal("delivered an uncommitted comment")
	}
	stage()
	requireOK(Write(ctx, lib.Service.DB, "comment", c.ID, c, 0, false))
	restart()
	deliver()
	info, e := lib.CommentWebhook(ctx, owner)
	requireOK(e)
	if redirected.Load() || len(bodies) != 1 || info.Last == nil || info.Last.State != "retrying" || info.Last.Code != 307 {
		t.Fatal("redirect or retry handling", info.Last)
	}
	restart()
	if delay := deliver(); delay <= 0 || delay > 10*time.Second || len(bodies) != 1 {
		t.Fatal("unexpected retry delay", delay)
	}
	rows, e := lib.Service.DB.List(ctx, storage.Query{Kind: "hook_queue"})
	requireOK(e)
	if len(rows) != 1 {
		t.Fatalf("expected one persisted retry, got %d", len(rows))
	}
	rows[0].Rank = 0 // Move its persisted deadline forward without slowing the test.
	requireOK(lib.Service.DB.Put(ctx, rows[0]))
	deliver()
	info, e = lib.CommentWebhook(ctx, owner)
	requireOK(e)
	if info.Last == nil || info.Last.State != "delivered" || info.Last.Attempt != 2 || len(bodies) != 2 || !bytes.Equal(bodies[0], bodies[1]) {
		t.Fatal("retry did not preserve the event", info.Last)
	}
	deliver()
	if len(bodies) != 2 {
		t.Fatal("success was retried")
	}

	requireOK(lib.Moderate(ctx, owner, c.ID, "approve"))
	requireOK(lib.Moderate(ctx, owner, c.ID, "approve"))
	deliver()
	deliver()
	if len(bodies) != 3 {
		t.Fatal("duplicate approval events")
	}
	requireOK(lib.Moderate(ctx, owner, c.ID, "delete"))
	deliver()
	if len(bodies) != 4 || bytes.Contains(bodies[3], []byte(c.Body)) {
		t.Fatal("leaf deletion missing or disclosed removed text")
	}

	// A queued event cannot be forwarded after switching the destination.
	stage()
	requireOK(Write(ctx, lib.Service.DB, "comment", c.ID, c, 0, false))
	cfg.URL = receiver.URL + "/changed"
	requireOK(lib.ConfigureCommentWebhook(ctx, owner, cfg))
	deliver()
	if redirected.Load() || len(bodies) != 4 {
		t.Fatal("old event forwarded to a new generation")
	}
}

func TestWebhookConfiguration(t *testing.T) {
	ctx := context.Background()
	db, e := storage.Open(ctx, storage.Config{Driver: "sqlite", Address: filepath.Join(t.TempDir(), "feed.db")})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	v, _ := NewVault(ID(32))
	l := Library{Service: New(db), Vault: v}
	owner := Actor{Owner: true, Fresh: time.Now()}
	cfg := CommentWebhookSettings{Enabled: true, URL: "https://example.org/events", Secret: ID(32)}
	if err := l.ConfigureCommentWebhook(ctx, Actor{Owner: true}, cfg); !errors.Is(err, Denied) {
		t.Fatal(err)
	}
	if _, err := l.CommentWebhook(ctx, Actor{}); !errors.Is(err, Denied) {
		t.Fatal(err)
	}
	for _, addr := range []string{"http://example.org", "https://user:pass@example.org/", "https://example.org/path#fragment", "/relative", ""} {
		bad := cfg
		bad.URL = addr
		if err := l.ConfigureCommentWebhook(ctx, owner, bad); !errors.Is(err, Invalid) {
			t.Fatalf("accepted %q: %v", addr, err)
		}
	}
	if err := l.ConfigureCommentWebhook(ctx, owner, cfg); err != nil {
		t.Fatal(err)
	}
	secret := cfg.Secret
	cfg.Secret = ""
	if err := l.ConfigureCommentWebhook(ctx, owner, cfg); err != nil {
		t.Fatal(err)
	}
	info, e := l.CommentWebhook(ctx, owner)
	if e != nil || !info.HasSecret {
		t.Fatal(info, e)
	}
	view, _ := json.Marshal(info)
	record, e := Read[[]byte](ctx, db, "config", hookConfigKey)
	if e != nil || strings.Contains(string(view), secret) || bytes.Contains(record, []byte(secret)) || bytes.Contains(record, []byte(cfg.URL)) {
		t.Fatal("settings leaked", e)
	}
	cfg.URL += "/new"
	if err := l.ConfigureCommentWebhook(ctx, owner, cfg); !errors.Is(err, Invalid) {
		t.Fatal("reused secret at a different URL", err)
	}
}
