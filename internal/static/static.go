package static

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist
var dist embed.FS

func FS() http.FileSystem {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic("static: failed to sub dist: " + err.Error())
	}
	return http.FS(sub)
}

// SPAHandler returns an http.Handler that serves static files from the embedded
// dist directory, falling back to index.html for paths that don't match a file
// (enabling client-side routing).
func SPAHandler() http.Handler {
	filesystem := FS()
	fileServer := http.FileServer(filesystem)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Vite-hashed assets are immutable — cache forever.
		if strings.HasPrefix(path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			fileServer.ServeHTTP(w, r)
			return
		}

		if path == "/" {
			w.Header().Set("Cache-Control", "no-cache")
			fileServer.ServeHTTP(w, r)
			return
		}

		// Try to open the file. If it exists and isn't a directory, serve it.
		f, err := filesystem.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			stat, statErr := f.Stat()
			_ = f.Close()
			if statErr == nil && !stat.IsDir() {
				w.Header().Set("Cache-Control", "no-cache")
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// File not found — serve index.html for client-side routing.
		w.Header().Set("Cache-Control", "no-cache")
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
