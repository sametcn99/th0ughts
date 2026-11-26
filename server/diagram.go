package server

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"io"
	"net/http"
)

//go:embed vendor/mermaid.js.gz
var diagramLibrary []byte
var diagramDigest = fmt.Sprintf(`W/"%x"`, sha256.Sum256(diagramLibrary))

// Keep the renderer out of the asset cache built on the first public page read.
func serveDiagramLibrary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, no-cache")
	w.Header().Set("Vary", "Accept-Encoding")
	w.Header().Set("ETag", diagramDigest)
	if r.Header.Get("If-None-Match") == diagramDigest {
		w.WriteHeader(304)
		return
	}
	if acceptsGzip(r.Header.Get("Accept-Encoding")) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Write(diagramLibrary)
		return
	}
	z, err := gzip.NewReader(bytes.NewReader(diagramLibrary))
	if err != nil {
		http.Error(w, "Invalid diagram asset", 500)
		return
	}
	defer z.Close()
	io.Copy(w, z)
}
