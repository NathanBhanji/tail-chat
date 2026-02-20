// Package themes provides an http.Handler that serves either the embedded
// default frontend or a user-installed theme from the filesystem.
package themes

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/NathanBhanji/tail-chat/internal/config"
)

// Handler returns an http.Handler that serves the active theme.
// If the active theme is "default", it serves from the embedded FS.
// Otherwise it serves from ~/.tailchat/themes/<name>/.
func Handler(embeddedAssets fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := config.Load()

		if cfg.ActiveTheme == "default" || cfg.ActiveTheme == "" {
			serveEmbedded(embeddedAssets, w, r)
			return
		}

		// Try filesystem theme
		themeDir := filepath.Join(config.ThemesDir(), cfg.ActiveTheme)
		indexPath := filepath.Join(themeDir, "index.html")
		if _, err := os.Stat(indexPath); err != nil {
			// Theme missing, fall back to default
			serveEmbedded(embeddedAssets, w, r)
			return
		}

		serveFilesystem(themeDir, w, r)
	})
}

func serveEmbedded(assets fs.FS, w http.ResponseWriter, r *http.Request) {
	// Strip leading slash
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// Try to open the file
	f, err := assets.Open(path)
	if err != nil {
		// SPA fallback: serve index.html for any unknown path
		path = "index.html"
		f, err = assets.Open(path)
		if err != nil {
			http.Error(w, "not found", 404)
			return
		}
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "stat error", 500)
		return
	}

	if stat.IsDir() {
		path = path + "/index.html"
		f.Close()
		f, err = assets.Open(path)
		if err != nil {
			http.Error(w, "not found", 404)
			return
		}
		defer f.Close()
		stat, _ = f.Stat()
	}

	setContentType(w, path)

	if seeker, ok := f.(http.File); ok {
		http.ServeContent(w, r, path, stat.ModTime(), seeker)
	} else {
		// fs.File doesn't implement io.ReadSeeker, read all
		data := make([]byte, stat.Size())
		f.Read(data)
		w.Write(data)
	}
}

func serveFilesystem(themeDir string, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	fullPath := filepath.Join(themeDir, filepath.Clean(path))

	// Security: ensure we don't escape the theme directory
	if !strings.HasPrefix(fullPath, themeDir) {
		http.Error(w, "forbidden", 403)
		return
	}

	stat, err := os.Stat(fullPath)
	if err != nil {
		// SPA fallback
		fullPath = filepath.Join(themeDir, "index.html")
		stat, err = os.Stat(fullPath)
		if err != nil {
			http.Error(w, "not found", 404)
			return
		}
	}

	if stat.IsDir() {
		fullPath = filepath.Join(fullPath, "index.html")
	}

	setContentType(w, fullPath)
	http.ServeFile(w, r, fullPath)
}

func setContentType(w http.ResponseWriter, path string) {
	switch {
	case strings.HasSuffix(path, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(path, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(path, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case strings.HasSuffix(path, ".json"):
		w.Header().Set("Content-Type", "application/json")
	case strings.HasSuffix(path, ".svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
	case strings.HasSuffix(path, ".png"):
		w.Header().Set("Content-Type", "image/png")
	case strings.HasSuffix(path, ".woff2"):
		w.Header().Set("Content-Type", "font/woff2")
	case strings.HasSuffix(path, ".woff"):
		w.Header().Set("Content-Type", "font/woff")
	}
}
