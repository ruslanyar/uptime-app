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

// Save returns an API-relative URL and stores the original bytes without re-encoding.
// Invalid or oversized images return auth.ErrInvalid; I/O errors pass through.
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
	// Valid dimensions alone do not guarantee an intact image payload.
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

// Remove is best-effort: invalid URLs and filesystem errors are ignored.
func Remove(dir, url string) {
	if name, ok := filename(url); ok {
		_ = os.Remove(filepath.Join(Directory(dir), name))
	}
}

// Serve exposes only generated image names; directory listings and arbitrary files are excluded.
// @Summary Get an avatar
// @Description Serves generated avatar files. Supports HTTP conditional requests and ranges; missing or invalid filenames return plain-text 404 responses.
// @Tags avatars
// @ID avatarGet
// @Produce image/jpeg,image/png,image/gif,image/webp
// @Param filename path string true "Generated UUID filename with .jpg, .png, .gif or .webp extension"
// @Success 200 {file} file "Avatar image"
// @Success 206 {file} file "Partial content"
// @Success 304 "Not Modified"
// @Header 200 {string} Cache-Control "public, max-age=31536000, immutable"
// @Failure 403 {object} response.ErrorResponse "Origin rejected (application/json)"
// @Failure 404 {string} string "File not found (text/plain)"
// @Failure 416 {string} string "Range not satisfiable (text/plain)"
// @Router /avatars/{filename} [get]
func Serve(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, ok := filename(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Replacements get new URLs, so cached images never need revalidation.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeFile(w, r, filepath.Join(Directory(dir), name))
	}
}
