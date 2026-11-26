package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/storage"
)

// Resume from the durable state left between writing imported records and committing the manifest.
func TestRestartRollsBackInterruptedRestore(t *testing.T) {
	ctx := context.Background()
	dir, key := t.TempDir(), feed.ID(32)
	s, err := Open(ctx, dir, key, storage.Config{})
	if err != nil {
		t.Fatal(err)
	}
	a, token := owner(t, s)
	before, err := s.S.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	after := before
	after.Name = "Interrupted restore"
	if err = s.S.SaveSettings(ctx, a, after); err != nil {
		t.Fatal(err)
	}
	post, err := s.S.Save(ctx, a, feed.Post{Original: "en", Text: map[string]feed.Translation{"en": {Body: "Temporary", Published: true}}})
	if err != nil {
		t.Fatal(err)
	}
	media := feed.Media{ID: "incomplete", Object: "temporary", Location: "disk"}
	if err = os.MkdirAll(s.L.Dir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(s.L.Dir, media.Object)
	if err = os.WriteFile(file, []byte("uncommitted"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = feed.Write(ctx, s.S.DB, "pending_media", media.ID, media, 0, false); err != nil {
		t.Fatal(err)
	}
	journal := map[string]any{"Site": before, "Media": []string{media.ID}, "Posts": []string{post.ID}}
	if err = feed.Write(ctx, s.S.DB, "system", "import", journal, 0, false); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		reopened, err := Open(ctx, dir, key, storage.Config{})
		if err != nil {
			t.Fatal(err)
		}
		settings, err := reopened.S.Settings(ctx)
		if err != nil || settings.Name != before.Name {
			t.Fatal("previous settings lost", settings, err)
		}
		if !reopened.A.Resolve(ctx, "session", token).Owner {
			t.Fatal("owner session lost")
		}
		for _, record := range [][2]string{{"post", post.ID}, {"media", media.ID}, {"pending_media", media.ID}, {"system", "import"}} {
			if _, err := reopened.S.DB.Get(ctx, record[0], record[1]); !errors.Is(err, storage.ErrMissing) {
				t.Fatal("unexpected record after recovery", record, err)
			}
		}
		if _, err = os.Stat(file); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("uncommitted object survived", err)
		}
		if err = reopened.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
