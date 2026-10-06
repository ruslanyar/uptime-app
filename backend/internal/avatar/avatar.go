// Package avatar validates and stores uploaded raster images.
package avatar

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp"
	"uptime-app/backend/internal/auth"
)

const MaxSize = 500 * 1024
const Prefix = "/api/v1/avatars/"

func Directory(dir string) string {
	if dir == "" {
		return "uploads/avatars"
	}
	return dir
}

// Save validates actual image data, regardless of the supplied name or MIME type.
func Save(dir string, reader io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxSize+1))
	if err != nil {
		return "", err
	}
	if len(data) == 0 || len(data) > MaxSize {
		return "", auth.ErrInvalid
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	// Bound decoded memory as well as the compressed file size.
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
		return "", auth.ErrInvalid
	}
	ext := map[string]string{"jpeg": ".jpg", "png": ".png", "gif": ".gif", "webp": ".webp"}[format]
	if ext == "" {
		return "", auth.ErrInvalid
	}
	if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
		return "", auth.ErrInvalid
	}
	dir = Directory(dir)
	if err = os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}
	name := auth.NewID() + ext
	if err = os.WriteFile(filepath.Join(dir, name), data, 0640); err != nil {
		return "", err
	}
	return Prefix + name, nil
}

func filename(url string) (string, bool) {
	name, ok := strings.CutPrefix(url, Prefix)
	if !ok {
		return "", false
	}
	ext := filepath.Ext(name)
	return name, auth.ValidID(strings.TrimSuffix(name, ext)) && (ext == ".jpg" || ext == ".png" || ext == ".gif" || ext == ".webp")
}

func Remove(dir, url string) {
	if name, ok := filename(url); ok {
		_ = os.Remove(filepath.Join(Directory(dir), name))
	}
}

// Serve exposes only generated image names; directory listings and arbitrary files are excluded.
func Serve(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, ok := filename(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeFile(w, r, filepath.Join(Directory(dir), name))
	}
}
