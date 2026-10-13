package postgres_test

import (
	"net/http/httptest"
	"testing"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/httpapi"
	"uptime-app/backend/internal/testdb"
)

func TestReadinessDatabase(t *testing.T) {
	_, pool, _ := testdb.New(t)
	handler := httpapi.New(nil, nil, nil, pool.Ping, config.Config{})
	check := func(want int) {
		t.Helper()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/health/ready", nil))
		if w.Code != want {
			t.Fatalf("status %d, want %d: %s", w.Code, want, w.Body.String())
		}
	}
	check(200)
	pool.Close()
	check(503)
}
