package authcontroller

import (
	"errors"
	"mime"
	"net/http"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/avatar"
	"uptime-app/backend/internal/httpapi/request"
	"uptime-app/backend/internal/httpapi/response"
)

// UpdateProfile handles POST /auth/profile.
// @Summary Update profile
// @Description Accepts access_token cookie or Authorization: Bearer JWT. A present Authorization header takes precedence. Accepts JSON name and optional remove_avatar, or multipart/form-data with exactly one name and one avatar file (JPEG, PNG, GIF or WebP, at most 500 KiB and 16 million pixels). Multipart schema is in x-multipart-request; the body schema describes JSON.
// @Tags auth
// @ID authUpdateProfile
// @Produce json
// @Accept json,mpfd
// @Param body body profileRequest true "Request body"
// @x-multipart-request {"type":"object","required":["name","avatar"],"properties":{"name":{"type":"string","minLength":2,"maxLength":50},"avatar":{"type":"file","description":"JPEG, PNG, GIF or WebP; at most 500 KiB and 16 million pixels"}}}
// @Param X-CSRF-Protection header string true "CSRF protection" Enums(1)
// @Security BearerAuth
// @Success 200 {object} authResponse
// @Header 200 {string} Cache-Control "no-store"
// @Failure 400 {object} response.ErrorResponse "Invalid input"
// @Failure 401 {object} response.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} response.ErrorResponse "Origin or CSRF rejected"
// @Failure 413 {object} response.ErrorResponse "Multipart body too large"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /auth/profile [post]
func (a *Controller) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	raw, e := request.AccessToken(r)
	if e != nil {
		serviceError(w, e)
		return
	}
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType == "multipart/form-data" {
		a.updateProfileImage(w, r, raw)
		return
	}
	var v profileRequest
	if e = request.Decode(w, r, &v); e != nil {
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
		// Keep the file until the database no longer references it.
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
		// Avoid orphaning an upload when the profile update fails.
		avatar.Remove(a.cfg.AvatarDir, url)
		serviceError(w, err)
		return
	}
	// A failed update must leave the previous avatar available.
	avatar.Remove(a.cfg.AvatarDir, previous.AvatarURL)
	response.JSON(w, 200, authResponse{User: u})
}
