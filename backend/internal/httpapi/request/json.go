package request

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"

	"uptime-app/backend/internal/auth"
)

// Decode rejects null, unknown fields, trailing JSON values and bodies over 16 KiB.
// Input errors are normalized to auth.ErrInvalid.
func Decode(w http.ResponseWriter, r *http.Request, v any) error {
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		return auth.ErrInvalid
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	var raw json.RawMessage
	if e = d.Decode(&raw); e != nil || string(raw) == "null" {
		return auth.ErrInvalid
	}
	if e = d.Decode(new(any)); e != io.EOF {
		return auth.ErrInvalid
	}
	inner := json.NewDecoder(strings.NewReader(string(raw)))
	inner.DisallowUnknownFields()
	if e = inner.Decode(v); e != nil {
		return auth.ErrInvalid
	}
	return nil
}
