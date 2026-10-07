package postgres_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/avatar"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/httpapi"
)

func TestAvatarUpload(t *testing.T) {
	ctx, _, service, _ := setup(t)
	account := register(t, ctx, service)
	dir := t.TempDir()
	handler := httpapi.New(service, nil, config.Config{AvatarDir: dir})
	var data bytes.Buffer
	png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	upload := func(content []byte, name, token, csrf string, extra bool) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		writer.WriteField("name", name)
		file, _ := writer.CreateFormFile("avatar", "../../untrusted.svg")
		file.Write(content)
		if extra {
			writer.WriteField("unknown", "value")
		}
		writer.Close()
		r := httptest.NewRequest("POST", "/api/v1/auth/profile", &body)
		r.Header.Set("Content-Type", writer.FormDataContentType())
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-CSRF-Protection", csrf)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, tc := range []struct {
		data              []byte
		name, token, csrf string
		extra             bool
		status            int
	}{
		{data.Bytes(), "Name", "invalid", "1", false, 401},
		{data.Bytes(), "Name", account.AccessToken, "", false, 403},
		{[]byte("<svg/>"), "Name", account.AccessToken, "1", false, 400},
		{data.Bytes()[:33], "Name", account.AccessToken, "1", false, 400},
		{bytes.Repeat([]byte{0}, avatar.MaxSize+1), "Name", account.AccessToken, "1", false, 400},
		{bytes.Repeat([]byte{0}, avatar.MaxSize+32*1024), "Name", account.AccessToken, "1", false, 413},
		{data.Bytes(), "x", account.AccessToken, "1", false, 400},
		{data.Bytes(), "Name", account.AccessToken, "1", true, 400},
	} {
		w := upload(tc.data, tc.name, tc.token, tc.csrf, tc.extra)
		if w.Code != tc.status {
			t.Fatal(w.Code, tc.status, w.Body.String())
		}
		files, _ := os.ReadDir(dir)
		if len(files) != 0 {
			t.Fatal("failed upload left files")
		}
	}
	var oldURL string
	for range 2 {
		w := upload(data.Bytes(), " Updated ", account.AccessToken, "1", false)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result struct {
			User auth.User `json:"user"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		u, err := service.Me(ctx, account.AccessToken)
		if err != nil || u != result.User || u.Name != "Updated" || !strings.HasSuffix(u.AvatarURL, ".png") || u.AvatarURL == oldURL {
			t.Fatal(u, err)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", u.AvatarURL, nil))
		if response.Code != 200 || !bytes.Equal(response.Body.Bytes(), data.Bytes()) {
			t.Fatal(response)
		}
		files, _ := os.ReadDir(dir)
		if len(files) != 1 {
			t.Fatal("replacement left old files")
		}
		if oldURL != "" {
			response = httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", oldURL, nil))
			if response.Code != 404 {
				t.Fatal("old avatar still served")
			}
		}
		oldURL = u.AvatarURL
	}
	u, err := service.UpdateProfile(ctx, account.AccessToken, "Name only")
	if err != nil || u.AvatarURL != oldURL {
		t.Fatal("name update lost avatar", u, err)
	}
	r, err := service.Refresh(ctx, account.RefreshToken)
	if err != nil || r.User.AvatarURL != oldURL {
		t.Fatal("refresh lost avatar", err)
	}
	r, err = service.Login(ctx, account.User.Email, " long password with spaces ")
	if err != nil || r.User.AvatarURL != oldURL {
		t.Fatal("login lost avatar", err)
	}
	deleteAvatar := func(name string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/auth/profile", strings.NewReader(`{"name":"`+name+`","remove_avatar":true}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+account.AccessToken)
		r.Header.Set("X-CSRF-Protection", "1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := deleteAvatar("x"); w.Code != 400 {
		t.Fatal(w.Code)
	}
	u, err = service.Me(ctx, account.AccessToken)
	if err != nil || u.AvatarURL != oldURL {
		t.Fatal("failed deletion changed profile", u, err)
	}
	for range 2 {
		w := deleteAvatar("Name only")
		if w.Code != 200 || strings.Contains(w.Body.String(), "avatar_url") {
			t.Fatal(w.Code, w.Body.String())
		}
		u, err = service.Me(ctx, account.AccessToken)
		if err != nil || u.AvatarURL != "" {
			t.Fatal("deletion did not persist", u, err)
		}
		files, _ := os.ReadDir(dir)
		if len(files) != 0 {
			t.Fatal("deleted avatar file remains")
		}
	}

}
