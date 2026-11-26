package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/sametcn99/th0ughts/feed"
)

func TestColdPublicPageVariants(t *testing.T) {
	s := fixture(t, "sqlite")
	owner(t, s)
	const readers = 32
	responses := make(chan *httptest.ResponseRecorder, readers)
	var ready sync.WaitGroup
	start := make(chan struct{})
	ready.Add(readers)
	for range readers {
		go func() {
			req := httptest.NewRequest("GET", "/?lang=tr", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			w := httptest.NewRecorder()
			ready.Done()
			<-start
			s.ServeHTTP(w, req)
			responses <- w
		}()
	}
	ready.Wait()
	close(start)
	var misses int
	var tag string
	for range readers {
		w := <-responses
		if w.Code != 200 || w.Header().Get("Content-Encoding") != "gzip" || len(w.Result().Cookies()) != 1 || !strings.Contains(w.Header().Get("Content-Security-Policy"), "nonce-") {
			t.Fatal(w.Code, w.Header())
		}
		if w.Header().Get("X-Cache") == "MISS" {
			misses++
		}
		if tag != "" && tag != w.Header().Get("ETag") {
			t.Fatal("representation changed across a single cold burst")
		}
		tag = w.Header().Get("ETag")
		z, e := gzip.NewReader(w.Body)
		if e != nil {
			t.Fatal(e)
		}
		raw, e := io.ReadAll(z)
		z.Close()
		if e != nil || !bytes.Contains(raw, []byte(`lang="tr"`)) {
			t.Fatal("invalid Turkish response", e)
		}
	}
	if misses != 1 {
		t.Fatalf("rendered %d times", misses)
	}
	for _, v := range []struct {
		encoding string
		status   int
	}{{"gzip", 304}, {"*;q=1, gzip;q=0", 200}} {
		req := httptest.NewRequest("GET", "/?lang=tr", nil)
		req.Header.Set("Accept-Encoding", v.encoding)
		req.Header.Set("If-None-Match", tag)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		if w.Code != v.status || v.status == 304 && w.Body.Len() != 0 || v.status == 200 && w.Header().Get("ETag") == tag {
			t.Fatal(w.Code, w.Header())
		}
	}
}

func TestThumbnailRevalidationFollowsVisibility(t *testing.T) {
	s := fixture(t, "sqlite")
	a, session := owner(t, s)
	f, e := os.Open("../scripts/fixtures/great-wave.jpg")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	ctx := context.Background()
	m, e := s.L.Upload(ctx, a, f)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.S.Save(ctx, a, feed.Post{Original: "en", Media: []string{m.ID}, Text: map[string]feed.Translation{"en": {Body: "Photograph", Published: true}}})
	if e != nil {
		t.Fatal(e)
	}
	path := "/media/" + m.ID + "?w=480"
	w := get(s, path, "")
	tag := w.Header().Get("ETag")
	conf, format, e := image.DecodeConfig(bytes.NewReader(w.Body.Bytes()))
	if e != nil || tag == "" || format != "jpeg" || conf.Width != 480 || int64(w.Body.Len()) >= m.Size {
		t.Fatal(conf, format, e, w.Code)
	}
	if raw := get(s, "/media/"+m.ID, ""); raw.Header().Get("ETag") == tag {
		t.Fatal("original and derivative share validator")
	}

	conditional := func() int {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("If-None-Match", tag)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		return w.Code
	}
	if conditional() != http.StatusNotModified {
		t.Fatal("validated photo downloaded again")
	}
	p.Text["en"] = feed.Translation{Body: "Withdrawn"}
	if _, e = s.S.Save(ctx, a, p); e != nil {
		t.Fatal(e)
	}
	if get(s, path, session).Code != 200 {
		t.Fatal("owner lost access to a draft")
	}
	if conditional() != 404 || get(s, path, "").Code != 404 {
		t.Fatal("cached draft visible to a reader")
	}
}

func TestThumbnailPreservesTransparency(t *testing.T) {
	s := fixture(t, "json-dir")
	a, _ := owner(t, s)
	ctx := context.Background()
	src := image.NewNRGBA(image.Rect(0, 0, 640, 4))
	src.SetNRGBA(320, 2, color.NRGBA{R: 255, A: 255})
	var data bytes.Buffer
	if e := png.Encode(&data, src); e != nil {
		t.Fatal(e)
	}
	m, e := s.L.Upload(ctx, a, &data)
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.S.Save(ctx, a, feed.Post{Original: "en", Media: []string{m.ID}, Text: map[string]feed.Translation{"en": {Body: "Drawing", Published: true}}})
	if e != nil {
		t.Fatal(e)
	}
	w := get(s, "/media/"+m.ID+"?w=480", "")
	result, e := png.Decode(w.Body)
	if e != nil {
		t.Fatal(e)
	}
	_, _, _, alpha := result.At(0, 0).RGBA()
	if alpha != 0 || result.Bounds().Dx() != 480 {
		t.Fatal("invalid transparent derivative", result.Bounds(), alpha)
	}
}
