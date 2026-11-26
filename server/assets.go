package server

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"
)

type asset struct {
	raw, gz []byte
	digest  string
}

var content = sync.OnceValue(func() map[string]asset {
	files, _ := assets.ReadDir("assets")
	out := map[string]asset{}
	for _, entry := range files {
		b, e := assets.ReadFile("assets/" + entry.Name())
		if e != nil {
			panic(e)
		}
		var compressed bytes.Buffer
		w := gzip.NewWriter(&compressed)
		w.Write(b)
		w.Close()
		h := sha256.Sum256(b)
		out[entry.Name()] = asset{b, compressed.Bytes(), hex.EncodeToString(h[:8])}
	}
	return out
})

func assetURL(name string) string { return "/assets/" + name + "?v=" + content()[name].digest }
func static(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	a, ok := content()[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.URL.Query().Get("v") == a.digest {
		w.Header().Set("Cache-Control", "public,max-age=31536000,immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(name)))
	w.Header().Set("Vary", "Accept-Encoding")
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Write(a.gz)
	} else {
		w.Write(a.raw)
	}
}
