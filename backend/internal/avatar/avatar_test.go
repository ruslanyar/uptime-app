package avatar

import (
	"bytes"
	"errors"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http/httptest"
	"os"
	"testing"

	"uptime-app/backend/internal/auth"
)

func TestValidationAndServing(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for _, format := range []string{"jpeg", "png", "gif"} {
		t.Run(format, func(t *testing.T) {
			var data bytes.Buffer
			switch format {
			case "jpeg":
				jpeg.Encode(&data, img, nil)
			case "png":
				png.Encode(&data, img)
			case "gif":
				gif.Encode(&data, img, nil)
			}
			dir := t.TempDir()
			url, err := Save(dir, bytes.NewReader(data.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			Serve(dir)(w, httptest.NewRequest("GET", url, nil))
			if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), data.Bytes()) || w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal(w)
			}
			Remove(dir, url)
			w = httptest.NewRecorder()
			Serve(dir)(w, httptest.NewRequest("GET", url, nil))
			if w.Code != 404 {
				t.Fatal(w.Code)
			}
		})
	}
	var pngData bytes.Buffer
	png.Encode(&pngData, img)
	for _, data := range [][]byte{nil, []byte("<svg xmlns='http://www.w3.org/2000/svg'/>"), []byte("fake.jpg"), pngData.Bytes()[:33], bytes.Repeat([]byte{0}, MaxSize+1)} {
		dir := t.TempDir()
		if _, err := Save(dir, bytes.NewReader(data)); !errors.Is(err, auth.ErrInvalid) {
			t.Fatal(err)
		}
		files, _ := os.ReadDir(dir)
		if len(files) != 0 {
			t.Fatal("invalid upload left files")
		}
	}
	// The limit is inclusive, including files with trailing metadata.
	boundary := append(pngData.Bytes(), make([]byte, MaxSize-pngData.Len())...)
	if _, err := Save(t.TempDir(), bytes.NewReader(boundary)); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{Prefix, Prefix + "file.svg", Prefix + "../secret", Prefix + "not-a-uuid.png"} {
		w := httptest.NewRecorder()
		Serve(t.TempDir())(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal(path, w.Code)
		}
	}
}
