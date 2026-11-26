package server

import (
	"net/http"
	"strconv"

	"github.com/sametcn99/th0ughts/feed"
)

func (s *Server) postResource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a := s.actor(r)
	id := r.PathValue("id")
	q := r.URL.Query()
	var result any
	var e error
	switch r.Method {
	case "GET":
		if id != "" {
			result, e = s.S.Get(ctx, a, id, q.Get("private") == "true")
		} else {
			n, _ := strconv.Atoi(q.Get("limit"))
			result, e = s.S.List(ctx, a, feed.Filter{Search: q.Get("q"), Tag: q.Get("tag"), Lang: q.Get("lang"), Cursor: q.Get("cursor"), Private: q.Get("private") == "true", Limit: n})
		}
	case "POST", "PUT":
		var p feed.Post
		if e = decode(w, r, &p); e == nil {
			if p.ID != "" && p.ID != id {
				e = feed.Invalid
			} else {
				p.ID = id
				result, e = s.S.Save(ctx, a, p)
			}
		}
	case "DELETE":
		e = s.L.DeletePost(ctx, a, id)
		result = true
	}
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, result)
}
func (s *Server) toolList(w http.ResponseWriter, r *http.Request) {
	a := s.actor(r)
	lang := r.URL.Query().Get("lang")
	if !feed.Locale(lang) {
		lang = "en"
	}
	var out []map[string]any
	for _, op := range commands {
		if visible(a, op) {
			out = append(out, map[string]any{"name": op.Name, "description": s.catalog[lang][op.Name], "inputSchema": op.Schema})
		}
	}
	respond(w, out)
}
