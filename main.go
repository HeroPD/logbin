package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed web/dist/*
var gzFiles embed.FS

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	fsys, err := fs.Sub(gzFiles, "web/dist")
	if err != nil {
		panic(err)
	}

	slog.Info("Embedded files:")
	fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			slog.Info("embedded file", "path", path)
		}
		return nil
	})

	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		slog.Info("incoming request", "path", path)

		if path == "/" {
			path = "/index.html"
			slog.Info("root path detected", "changed_to", path)
		}

		gzPath := strings.TrimPrefix(path, "/") + ".gz"
		slog.Info("looking for file", "gzPath", gzPath)

		data, err := fs.ReadFile(fsys, gzPath)
		if err != nil {
			slog.Error("file read error", "gzPath", gzPath, "error", err)
			http.NotFound(w, r)
			return
		}

		slog.Info("file read success", "gzPath", gzPath, "size_bytes", len(data))

		switch ext := filepath.Ext(path); ext {
		case ".js":
			w.Header().Set("Content-Type", "application/javascript")
		case ".css":
			w.Header().Set("Content-Type", "text/css")
		case ".html":
			w.Header().Set("Content-Type", "text/html")
		case ".svg":
			w.Header().Set("Content-Type", "image/svg+xml")
		default:
			w.Header().Set("Content-Type", "application/octet-stream")
		}

		w.Header().Set("Content-Encoding", "gzip")
		w.Write(data)
	})

	db, err := badger.Open(badger.DefaultOptions("/tmp/badger"))
	if err != nil {
		slog.Error("failed to open badger", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	slog.Info("server starting", "port", 8080)
	if err := http.ListenAndServe(":8080", r); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
