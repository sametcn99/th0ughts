package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/storage"
)

func fixture(t *testing.T, driver string) *Server {
	t.Helper()
	dir := t.TempDir()
	s, e := Open(context.Background(), dir, feed.ID(32), storage.Config{Driver: driver, Address: filepath.Join(dir, "records")})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func owner(t *testing.T, s *Server) (feed.Actor, string) {
	t.Helper()
	ctx := context.Background()
	if e := s.A.Setup(ctx, "a-local-test-password", false); e != nil {
		t.Fatal(e)
	}
	v, e := s.A.Session(ctx)
	if e != nil {
		t.Fatal(e)
	}
	return s.A.Resolve(ctx, "session", v), v
}
func get(s *Server, path, session string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", path, nil)
	if session != "" {
		r.AddCookie(&http.Cookie{Name: "__Host-session", Value: session})
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func TestPublicVisibility(t *testing.T) {
	s := fixture(t, "sqlite")
	ctx := context.Background()
	a, session := owner(t, s)
	p, e := s.S.Save(ctx, a, feed.Post{Original: "en", Text: map[string]feed.Translation{"en": {Body: "Original text", Published: true}, "tr": {Body: "SECRET TRANSLATION"}}})
	if e != nil {
		t.Fatal(e)
	}
	path := "/?post=" + p.ID + "&lang=tr"
	w := get(s, path, session)
	if !strings.Contains(w.Body.String(), "SECRET TRANSLATION") || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("owner preview missing or cached")
	}
	for _, path := range []string{path, "/feed.xml?lang=tr", "/post/" + p.ID + ".md?lang=tr", "/feed.md", "/sitemap.xml", "/?q=SECRET", "/api/v1/list_posts"} {
		w = get(s, path, "")
		if w.Code != 200 || strings.Contains(w.Body.String(), "SECRET TRANSLATION") {
			t.Fatalf("public %s status %d", path, w.Code)
		}
	}
	w = get(s, path, "")
	if !strings.Contains(w.Body.String(), "Original text") || strings.Contains(w.Body.String(), `hreflang="tr"`) || !strings.Contains(w.Body.String(), `property="og:title"`) {
		t.Fatal("incorrect localized metadata")
	}
	tr := p.Text["tr"]
	tr.Published = true
	p.Text["tr"] = tr
	p, e = s.S.Save(ctx, a, p)
	if e != nil {
		t.Fatal(e)
	}
	w = get(s, path, "")
	if !strings.Contains(w.Body.String(), "SECRET TRANSLATION") || !strings.Contains(w.Body.String(), `hreflang="tr"`) {
		t.Fatal("translation not published")
	}
	req := httptest.NewRequest("POST", "/api/v1/delete_post", strings.NewReader(`{"id":"`+p.ID+`"}`))
	req.AddCookie(&http.Cookie{Name: "__Host-session", Value: session})
	w = httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("missing CSRF check")
	}
	if e = s.S.Delete(ctx, a, p.ID); e != nil {
		t.Fatal(e)
	}
	if w = get(s, path, ""); w.Code != 404 {
		t.Fatal("withdrawn content still available")
	}
}
func TestPortableMediaBackup(t *testing.T) {
	src := fixture(t, "sqlite")
	a, _ := owner(t, src)
	ctx := context.Background()
	b, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+X2ZkAAAAASUVORK5CYII=")
	m, e := src.L.Upload(ctx, a, bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	p, e := src.S.Save(ctx, a, feed.Post{Original: "tr", Text: map[string]feed.Translation{"tr": {Body: "İzmir", Published: true}}, Media: []string{m.ID}})
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = src.L.Export(ctx, a, &buf); e != nil {
		t.Fatal(e)
	}
	dst := fixture(t, "json-dir")
	a, _ = owner(t, dst)
	z, e := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if e != nil {
		t.Fatal(e)
	}
	filter := feed.Filter{Search: "izmir", Lang: "tr"}
	if _, e = dst.S.List(ctx, feed.Actor{}, filter); e != nil {
		t.Fatal(e)
	}
	if e = dst.L.Import(ctx, a, z); e != nil {
		t.Fatal(e)
	}
	results, e := dst.S.List(ctx, feed.Actor{}, filter)
	if e != nil || len(results.Posts) != 1 || results.Posts[0].ID != p.ID {
		t.Fatal("restored post missing from the existing search service", results, e)
	}
	w := get(dst, "/media/"+m.ID, "")
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), b) {
		t.Fatal("incorrect media restore")
	}
	q, e := dst.S.Get(ctx, a, p.ID, true)
	if e != nil {
		t.Fatal(e)
	}
	q.Text["tr"] = feed.Translation{Body: "İzmir"}
	if _, e = dst.S.Save(ctx, a, q); e != nil {
		t.Fatal(e)
	}
	if w = get(dst, "/media/"+m.ID, ""); w.Code != 404 {
		t.Fatal("private media exposed")
	}
	if e = dst.L.Import(ctx, a, z); e != feed.Conflict {
		t.Fatal("restore into nonempty store", e)
	}
}
func TestMCPClient(t *testing.T) {
	s := fixture(t, "sqlite")
	h := httptest.NewServer(s)
	defer h.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	sess, e := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: h.URL + "/mcp"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer sess.Close()
	list, e := sess.ListTools(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, tool := range list.Tools {
		if tool.Name == "save_post" {
			t.Fatal("write tool exposed to anonymous client")
		}
	}
	res, e := sess.CallTool(ctx, &mcp.CallToolParams{Name: "list_posts", Arguments: map[string]any{}})
	if e != nil || res.IsError {
		t.Fatal(e, res)
	}
}
func TestCatalogParity(t *testing.T) {
	en, tr := Labels("en"), Labels("tr")
	if len(en) != len(tr) {
		t.Fatal("catalog lengths differ")
	}
	for k := range en {
		if tr[k] == "" {
			t.Fatal(k)
		}
	}
	for _, op := range commands {
		if en[op.Name] == "" {
			t.Fatal(op.Name)
		}
	}
}
func FuzzArchive(f *testing.F) {
	var sample bytes.Buffer
	z := zip.NewWriter(&sample)
	dst, _ := z.Create("manifest.json")
	json.NewEncoder(dst).Encode(feed.Manifest{Version: 1, Site: feed.Settings{Name: "Test", Lang: "en", Theme: "paper"}})
	z.Close()
	f.Add(sample.Bytes())
	f.Add([]byte("zip"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		z, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if e != nil {
			return
		}
		for _, v := range z.File {
			if v.UncompressedSize64 > 1<<20 {
				return
			}
		}
		s := fixture(t, "json-dir")
		_ = s.L.Import(context.Background(), feed.Actor{Owner: true, Fresh: time.Now()}, z)
	})
}

func TestDraftTranslationAttachment(t *testing.T) {
	s := fixture(t, "sqlite")
	a, _ := owner(t, s)
	ctx := context.Background()
	image, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+X2ZkAAAAASUVORK5CYII=")
	m, e := s.L.Upload(ctx, a, bytes.NewReader(image))
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.S.Save(ctx, a, feed.Post{Original: "en", Text: map[string]feed.Translation{"en": {Body: "Public", Published: true}, "tr": {Body: "Private", Media: []string{m.ID}}}})
	if e != nil {
		t.Fatal(e)
	}
	if get(s, "/media/"+m.ID+"?w=480", "").Code != 404 {
		t.Fatal("translation attachment visible before publication")
	}
	tr := p.Text["tr"]
	tr.Published = true
	p.Text["tr"] = tr
	p, e = s.S.Save(ctx, a, p)
	if e != nil {
		t.Fatal(e)
	}
	if get(s, "/media/"+m.ID, "").Code != 200 {
		t.Fatal("published attachment missing")
	}
	tr.Published = false
	p.Text["tr"] = tr
	if _, e = s.S.Save(ctx, a, p); e != nil {
		t.Fatal(e)
	}
	if get(s, "/media/"+m.ID, "").Code != 404 {
		t.Fatal("withdrawn attachment visible")
	}
}

func TestTimelineNotesAndArticlePreviews(t *testing.T) {
	s := fixture(t, "sqlite")
	a, _ := owner(t, s)
	for _, tc := range []struct {
		title, body string
		preview     bool
	}{
		{"", strings.Repeat("ğ", 280), false},
		{"", strings.Repeat("ğ", 280) + "Z", true},
		{"Earlier article", "A short introduction.", true},
	} {
		p, err := s.S.Save(context.Background(), a, feed.Post{Original: "en", Text: map[string]feed.Translation{"en": {Title: tc.title, Body: tc.body, Published: true}}})
		if err != nil {
			t.Fatal(err)
		}
		timeline := get(s, "/?lang=tr", "").Body.String()
		href := esc(link("", p.ID, "tr"))
		cta := `class="read-post" href="` + href + `"`
		if strings.Contains(timeline, cta) != tc.preview {
			t.Fatal("Unexpected preview behavior", tc.title)
		}
		if tc.preview && tc.title == "" && strings.Contains(timeline, tc.body) {
			t.Fatal("Long body leaked into timeline")
		}
		if !tc.preview && !strings.Contains(timeline, tc.body) {
			t.Fatal("Short post was truncated")
		}
		detail := get(s, link("", p.ID, "tr"), "").Body.String()
		if strings.Contains(detail, cta) || !strings.Contains(detail, tc.body) {
			t.Fatal("Missing full post on permalink")
		}
		md := get(s, "/post/"+p.ID+".md", "").Body.String()
		if !strings.Contains(md, tc.body) {
			t.Fatal("Markdown export was truncated")
		}
	}
}
