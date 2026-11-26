package server

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
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
func (s *Server) thumbnail(w http.ResponseWriter, r *http.Request, m feed.Media, original func() (feed.ReadSeekCloser, error), width int) error {
	ext, typ := ".png", "image/png"
	if m.Type == "image/jpeg" {
		ext, typ = ".jpg", m.Type
	}
	key := m.ID + "-" + m.Object + "-" + strconv.Itoa(width) + ext
	file := filepath.Join(s.dir, "thumbnails", key)
	open := func() bool {
		f, e := os.Open(file)
		if e != nil {
			return false
		}
		defer f.Close()
		w.Header().Set("Content-Type", typ)
		http.ServeContent(w, r, m.ID, time.Time{}, f)
		return true
	}
	if m.Comment == "" && open() {
		return nil
	}
	encoded, e := renderThumbnail(r.Context(), m, original, width, file)
	if e != nil {
		return e
	}
	if encoded == nil {
		if open() {
			return nil
		}
		return os.ErrNotExist
	}
	w.Header().Set("Content-Type", typ)
	http.ServeContent(w, r, m.ID, time.Time{}, bytes.NewReader(encoded))
	return nil
}

// Release the CPU slot before streaming bytes to a slow client.
func renderThumbnail(ctx context.Context, m feed.Media, original func() (feed.ReadSeekCloser, error), width int, file string) ([]byte, error) {
	select {
	case thumbnailWorkers <- struct{}{}:
		defer func() { <-thumbnailWorkers }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if m.Comment == "" {
		if _, err := os.Stat(file); err == nil {
			return nil, nil
		}
	}
	f, e := original()
	if e != nil {
		return nil, e
	}
	defer f.Close()
	src, _, e := image.Decode(f)
	if e != nil {
		return nil, feed.Invalid
	}
	if width != m.Width {
		height := max(1, m.Height*width/m.Width)
		dst := image.NewRGBA(image.Rect(0, 0, width, height))
		draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
		src = dst
	}
	var out bytes.Buffer
	if m.Type == "image/jpeg" {
		e = jpeg.Encode(&out, src, &jpeg.Options{Quality: 82})
	} else {
		e = png.Encode(&out, src)
	}
	if e != nil {
		return nil, e
	}
	if m.Comment == "" {
		if e = storage.Atomic(file, out.Bytes()); e != nil {
			return nil, e
		}
	}
	return out.Bytes(), nil
}
