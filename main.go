package main

import (
	"bytes"
	"compress/gzip"
	"io"
	// "encoding/json"
	"embed"
	"encoding/hex"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type GELFMessage struct {
	Version      string                 `json:"version"`
	Host         string                 `json:"host"`
	ShortMessage string                 `json:"short_message"`
	FullMessage  string                 `json:"full_message"`
	Timestamp    float64                `json:"timestamp"`
	Level        int                    `json:"level"`
	Extra        map[string]interface{} `json:"-"`
}

func GelfDetectType(data []byte) string {
	if len(data) < 2 {
		return "unknown"
	}

	// gzip magic number
	if data[0] == 0x1f && data[1] == 0x8b {
		return "gzip"
	}

	// zlib
	if data[0] == 0x78 && (data[1] == 0x01 || data[1] == 0x9c || data[1] == 0xda) {
		return "zlib"
	}

	// chunk magic number
	if data[0] == 0x1e && data[1] == 0x0f {
		return "chunk"
	}

	return "none" // plain JSON
}

func StartUDPEcho(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}

	slog.Info("UDP echo server started", "addr", addr)

	go func() {
		buf := make([]byte, 65535)
		for {
			n, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				slog.Error("UDP read error", "error", err)
				continue
			}

			switch GelfDetectType(buf[:2]) {
			case "gzip":
				r, _ := gzip.NewReader(bytes.NewReader(buf[:n]))
				defer r.Close()
				decompressedData, _ := io.ReadAll(r)
				slog.Info("Decompressed", "data hex", hex.EncodeToString(decompressedData))
				slog.Info("Decompressed", "data string", string(decompressedData))
			}

			msg := string(buf[:n])
			slog.Info("UDP packet received", "from", remoteAddr.String(), "size", n, "msg", msg, "hex", hex.EncodeToString(buf[:n]))

			// Echo back
			_, err = conn.WriteToUDP(buf[:n], remoteAddr)
			if err != nil {
				slog.Error("UDP write error", "error", err)
			}
		}
	}()

	return nil
}

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

	if err := StartUDPEcho(":12201"); err != nil {
		slog.Error("failed to start UDP echo", "error", err)
		os.Exit(1)
	}
	slog.Info("server starting", "port", 8080)
	if err := http.ListenAndServe(":9101", r); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
