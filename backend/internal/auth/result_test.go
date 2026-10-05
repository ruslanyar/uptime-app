package auth

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResultJSONContainsOnlyUser(t *testing.T) {
	result := Result{
		AccessToken:    "secret-access-token",
		TokenType:      "Bearer",
		ExpiresIn:      86400,
		User:           User{ID: NewID(), Name: "User", Email: "user@example.com"},
		RefreshToken:   "secret-refresh-token",
		SessionExpires: time.Now().Add(SessionTTL),
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields["user"] == nil {
		t.Fatal("Result JSON must contain only the public user")
	}
	var user User
	if err := json.Unmarshal(fields["user"], &user); err != nil {
		t.Fatal(err)
	}
	if user != result.User {
		t.Fatal("Result JSON lost public user data")
	}
}
