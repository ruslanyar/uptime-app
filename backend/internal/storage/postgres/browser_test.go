package postgres_test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/httpapi"
)

func TestBrowserCrossSite(t *testing.T) {
	if os.Getenv("AUTH_BROWSER_CHECK") != "1" {
		t.Skip("set AUTH_BROWSER_CHECK=1 to check browser HTTPS cross-site flow")
	}
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Fatal("browser check requires TEST_DATABASE_URL")
	}
	_, _, service, _ := setup(t)
	var apiBase string
	frontend := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<!doctype html><title>Auth contract check</title><script>window.apiBase=%q;</script>", apiBase)
	}))
	defer frontend.Close()
	_, frontendPort, e := net.SplitHostPort(frontend.Listener.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	frontendURL := "https://frontend.auth-client.test:" + frontendPort
	cfg := config.Config{AllowedOrigins: map[string]bool{frontendURL: true}, CookieSecure: true, CookieSameSite: http.SameSiteNoneMode}
	api := httptest.NewTLSServer(httpapi.New(service, cfg))
	defer api.Close()
	_, apiPort, e := net.SplitHostPort(api.Listener.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	apiBase = "https://api.auth-service.test:" + apiPort
	frontend.StartTLS()
	command := exec.CommandContext(t.Context(), "node", "../../../scripts/browser-check.cjs")
	command.Env = append(os.Environ(), "BROWSER_FRONTEND_URL="+frontendURL, "BROWSER_API_URL="+apiBase)
	output, e := command.CombinedOutput()
	if e != nil {
		t.Fatalf("browser check: %v\n%s", e, output)
	}
	t.Log(string(output))
}
