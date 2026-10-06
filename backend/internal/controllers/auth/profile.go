package authcontroller

import (
	"errors"
	"mime"
	"net/http"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/avatar"
	"uptime-app/backend/internal/httpapi/response"
)

func (a *Controller) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	raw, e := accessToken(r)
	if e != nil {
		serviceError(w, e)
		return
	}
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType == "multipart/form-data" {
		a.updateProfileImage(w, r, raw)
		return
	}
	var v struct {
		Name         string `json:"name"`
		RemoveAvatar bool   `json:"remove_avatar"`
	}
	if e = decode(w, r, &v); e != nil {
		serviceError(w, e)
		return
	}
	var previous auth.User
	var avatarURL []string
	if v.RemoveAvatar {
		previous, e = a.service.Me(r.Context(), raw)
		if e != nil {
			serviceError(w, e)
			return
		}
		avatarURL = []string{""}
	}
	u, e := a.service.UpdateProfile(r.Context(), raw, v.Name, avatarURL...)
	if e != nil {
		serviceError(w, e)
		return
	}
	if v.RemoveAvatar {
		avatar.Remove(a.cfg.AvatarDir, previous.AvatarURL)
	}
	response.JSON(w, 200, authResponse{User: u})
}

func (a *Controller) updateProfileImage(w http.ResponseWriter, r *http.Request, raw string) {
	previous, err := a.service.Me(r.Context(), raw)
	if err != nil {
		serviceError(w, err)
		return
	}
	// Allow bounded multipart metadata in addition to the 500 KiB file.
	r.Body = http.MaxBytesReader(w, r.Body, avatar.MaxSize+16*1024)
	err = r.ParseMultipartForm(avatar.MaxSize + 16*1024)
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			response.Error(w, 413, "file_too_large", "Avatar must be at most 500 KiB")
			return
		}
		serviceError(w, auth.ErrInvalid)
		return
	}
	f := r.MultipartForm
	if len(f.Value) != 1 || len(f.Value["name"]) != 1 || len(f.File) != 1 || len(f.File["avatar"]) != 1 {
		serviceError(w, auth.ErrInvalid)
		return
	}
	file, err := f.File["avatar"][0].Open()
	if err != nil {
		serviceError(w, err)
		return
	}
	defer file.Close()
	url, err := avatar.Save(a.cfg.AvatarDir, file)
	if err != nil {
		serviceError(w, err)
		return
	}
	u, err := a.service.UpdateProfile(r.Context(), raw, f.Value["name"][0], url)
	if err != nil {
		avatar.Remove(a.cfg.AvatarDir, url)
		serviceError(w, err)
		return
	}
	avatar.Remove(a.cfg.AvatarDir, previous.AvatarURL)
	response.JSON(w, 200, authResponse{User: u})
}
