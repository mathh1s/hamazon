package server

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// assetHandler serves the extracted game assets. They are public on purpose:
// the access screen itself is drawn with these sprites and the font.
func assetHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := path.Clean("/" + r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		// A 404 must not be cached, the files appear once the download finishes.
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			w.Header().Set("Cache-Control", "no-store")
			http.NotFound(w, r)
			return
		}
		if name == "/manifest.json" {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}
		files.ServeHTTP(w, r)
	})
}
