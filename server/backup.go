package server

import (
	"archive/zip"
	"io"
	"net/http"
	"os"

	"github.com/sametcn99/th0ughts/feed"
)

func (s *Server) backup(w http.ResponseWriter, r *http.Request) {
	a := s.actor(r)
	if !a.Recent() {
		fail(w, feed.Denied)
		return
	}
	f, e := os.CreateTemp(s.dir, ".backup-")
	if e != nil {
		fail(w, e)
		return
	}
	defer f.Close()
	defer os.Remove(f.Name())
	if e = s.L.Export(r.Context(), a, f); e != nil {
		fail(w, e)
		return
	}
	f.Seek(0, 0)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=th0ughts.zip")
	io.Copy(w, f)
}
func (s *Server) restore(w http.ResponseWriter, r *http.Request) {
	a := s.actor(r)
	if !a.Recent() {
		fail(w, feed.Denied)
		return
	}
	f, e := os.CreateTemp(s.dir, ".restore-")
	if e != nil {
		fail(w, e)
		return
	}
	defer f.Close()
	defer os.Remove(f.Name())
	n, e := io.Copy(f, http.MaxBytesReader(w, r.Body, 4<<30))
	if e != nil {
		fail(w, feed.Invalid)
		return
	}
	z, e := zip.NewReader(f, n)
	if e != nil {
		fail(w, feed.Invalid)
		return
	}
	if e = s.L.Import(r.Context(), a, z); e != nil {
		fail(w, e)
		return
	}
	s.cache = &pageCache{}
	s.S = feed.New(s.S.DB)
	s.L.Service = s.S
	s.T = s.S.Themes
	respond(w, true)
}
