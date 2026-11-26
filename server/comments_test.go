package server

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/storage"
)

func TestCodeOnlyWorksOnceForItsPost(t *testing.T) {
	var g guestMail
	id, e := g.issue("post", "reader@example.org", "123456")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = g.reserve(id, "other", "123456"); !errors.Is(e, invalidCode) {
		t.Fatal(e)
	}
	if _, e = g.reserve(id, "post", "000000"); !errors.Is(e, invalidCode) {
		t.Fatal(e)
	}
	finish, e := g.reserve(id, "post", "123456")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = g.reserve(id, "post", "123456"); !errors.Is(e, invalidCode) {
		t.Fatal("concurrent reuse", e)
	}
	finish(true)
	if _, e = g.reserve(id, "post", "123456"); !errors.Is(e, invalidCode) {
		t.Fatal("reuse", e)
	}
	if _, e = g.issue("post", "reader@example.org", "010101"); !errors.Is(e, commentRate) {
		t.Fatal(e)
	}
	id, _ = g.issue("post", "next@example.org", "111111")
	g.challenges[id].until = time.Now().Add(-time.Minute)
	if _, e = g.reserve(id, "post", "111111"); !errors.Is(e, invalidCode) {
		t.Fatal("expired", e)
	}
	for _, email := range []string{"missing", "Person <a@example.org>", "x@bad..org", "x@example.org\nBcc:foo@example.org", "x@127.0.0.1"} {
		if _, e := mailbox(email); e == nil {
			t.Fatal(email)
		}
	}
	if v, e := mailbox(" User@EXAMPLE.org "); e != nil || v != "User@example.org" {
		t.Fatal(v, e)
	}
}
func TestThreadLifecycleAcrossBackup(t *testing.T) {
	for _, driver := range []string{"sqlite", "json", "json-dir"} {
		t.Run(driver, func(t *testing.T) {
			s := fixture(t, driver)
			a, _ := owner(t, s)
			ctx := context.Background()
			site, _ := s.S.Settings(ctx)
			site.Comments = feed.CommentPolicy{Mode: "review", Images: 2, Markdown: true}
			if e := s.S.SaveSettings(ctx, a, site); e != nil {
				t.Fatal(e)
			}
			p, e := s.S.Save(ctx, a, feed.Post{Original: "en", Text: map[string]feed.Translation{"en": {Body: "Timeline", Published: true}}})
			if e != nil {
				t.Fatal(e)
			}
			var photo bytes.Buffer
			jpeg.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 3, 3)), nil)
			c, e := s.L.SubmitVerifiedComment(ctx, p.ID, "", "", "**Hello**", "markdown", [][]byte{photo.Bytes(), photo.Bytes()})
			if e != nil {
				t.Fatal(e)
			}
			if _, e = s.commentContext(ctx, c.ID); !errors.Is(e, storage.ErrMissing) {
				t.Fatal(e)
			}
			if _, e = s.L.SubmitVerifiedComment(ctx, p.ID, c.ID, "", "nested", "text", nil); e == nil {
				t.Fatal("pending parent")
			}
			if e = s.L.Moderate(ctx, feed.Actor{}, c.ID, "approve"); !errors.Is(e, feed.Denied) {
				t.Fatal(e)
			}
			if e = s.L.Moderate(ctx, a, c.ID, "edit"); !errors.Is(e, feed.Invalid) {
				t.Fatal(e)
			}
			if e = s.L.Moderate(ctx, a, c.ID, "approve"); e != nil {
				t.Fatal(e)
			}
			root, e := s.threadPage(ctx, feed.Actor{}, feed.CommentQuery{Post: p.ID}, false)
			if e != nil || len(root.Items) != 1 || !strings.Contains(root.Items[0].HTML, "<strong>Hello</strong>") {
				t.Fatal(root, e)
			}
			child, e := s.L.SubmitVerifiedComment(ctx, p.ID, c.ID, "Reader", "plain", "text", nil)
			if e != nil {
				t.Fatal(e)
			}
			var backup bytes.Buffer
			if e = s.L.Export(ctx, a, &backup); e != nil {
				t.Fatal(e)
			}
			z, e := zip.NewReader(bytes.NewReader(backup.Bytes()), int64(backup.Len()))
			if e != nil {
				t.Fatal(e)
			}
			restored := fixture(t, driver)
			if e = restored.L.Import(ctx, a, z); e != nil {
				t.Fatal(e)
			}
			pending, e := restored.S.CommentList(ctx, a, feed.CommentQuery{}, true)
			if e != nil || len(pending.Items) != 1 || pending.Items[0].ID != child.ID {
				t.Fatal(pending, e)
			}
			if e = restored.L.Moderate(ctx, a, child.ID, "approve"); e != nil {
				t.Fatal(e)
			}
			if e = restored.L.Moderate(ctx, a, c.ID, "delete"); e != nil {
				t.Fatal(e)
			}
			path, e := restored.commentContext(ctx, child.ID)
			if e != nil || len(path) != 2 || path[0].Status != "deleted" || len(path[0].Images) > 0 || path[1].Parent != c.ID {
				t.Fatal(path, e)
			}
			if e = restored.L.DeletePost(ctx, a, p.ID); e != nil {
				t.Fatal(e)
			}
			if _, e = feed.Read[feed.Comment](ctx, restored.S.DB, "comment", child.ID); !errors.Is(e, storage.ErrMissing) {
				t.Fatal(e)
			}
		})
	}
}
func TestThreadRoutesRejectInvalidEmailAndUnverifiedUploads(t *testing.T) {
	s := fixture(t, "sqlite")
	a, _ := owner(t, s)
	ctx := context.Background()
	site, _ := s.S.Settings(ctx)
	site.Comments.Mode = "review"
	site.Comments.RequireEmailCode = true
	s.S.SaveSettings(ctx, a, site)
	p, _ := s.S.Save(ctx, a, feed.Post{Original: "en", Text: map[string]feed.Translation{"en": {Body: "A note", Published: true}}})
	for _, tc := range []struct{ path, body, code string }{{"/api/v1/comment-code", `{"post":"` + p.ID + `","email":"invalid"}`, "invalid_email"}, {"/api/v1/comments/" + p.ID, "", "invalid_comment_code"}} {
		r := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 400 || !strings.Contains(w.Body.String(), tc.code) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

func TestCommentCursorSurvivesLeafRemoval(t *testing.T) {
	s := fixture(t, "sqlite")
	a, _ := owner(t, s)
	ctx := context.Background()
	site, _ := s.S.Settings(ctx)
	site.Comments.Mode = "open"
	if e := s.S.SaveSettings(ctx, a, site); e != nil {
		t.Fatal(e)
	}
	p, e := s.S.Save(ctx, a, feed.Post{Original: "en", Text: map[string]feed.Translation{"en": {Body: "Notes", Published: true}}})
	if e != nil {
		t.Fatal(e)
	}
	for range 22 {
		if _, e := s.L.SubmitVerifiedComment(ctx, p.ID, "", "", "A comment", "text", nil); e != nil {
			t.Fatal(e)
		}
	}
	q := feed.CommentQuery{Post: p.ID}
	first, e := s.S.CommentList(ctx, feed.Actor{}, q, false)
	if e != nil || len(first.Items) != 20 || first.Next == "" {
		t.Fatal(first, e)
	}
	if e = s.L.Moderate(ctx, a, first.Items[19].ID, "delete"); e != nil {
		t.Fatal(e)
	}
	q.Cursor = first.Next
	page, e := s.S.CommentList(ctx, feed.Actor{}, q, false)
	if e != nil || len(page.Items) != 2 || page.Next != "" {
		t.Fatal(page, e)
	}
}
