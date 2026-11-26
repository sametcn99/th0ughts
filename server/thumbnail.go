package server

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/storage"
	"golang.org/x/image/draw"
)

var thumbnailWorkers = make(chan struct{}, 2)

// The caller authorizes access to the original before every derivative read.
func (s *Server) thumbnail(w http.ResponseWriter, r *http.Request, m feed.Media, original io.Reader, width int) error {
	key := m.ID + "-" + m.Object + "-" + strconv.Itoa(width) + ".png"
	file := filepath.Join(s.dir, "thumbnails", key)
	open := func() bool {
		f, e := os.Open(file)
		if e != nil {
			return false
		}
		defer f.Close()
		w.Header().Set("Content-Type", "image/png")
		http.ServeContent(w, r, m.ID, time.Time{}, f)
		return true
	}
	if m.Comment == "" && open() {
		return nil
	}
	select {
	case thumbnailWorkers <- struct{}{}:
		defer func() { <-thumbnailWorkers }()
	case <-r.Context().Done():
		return context.Canceled
	}
	if m.Comment == "" && open() {
		return nil
	}
	src, _, e := image.Decode(original)
	if e != nil {
		return feed.Invalid
	}
	height := max(1, m.Height*width/m.Width)
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	var out bytes.Buffer
	if e = png.Encode(&out, dst); e != nil {
		return e
	}
	if m.Comment == "" {
		if e = storage.Atomic(file, out.Bytes()); e != nil {
			return e
		}
	}
	w.Header().Set("Content-Type", "image/png")
	http.ServeContent(w, r, m.ID, time.Time{}, bytes.NewReader(out.Bytes()))
	return nil
}
