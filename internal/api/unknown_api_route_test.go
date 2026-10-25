package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/peasant-labs/peasant/internal/api"
)

// TestUnknownAPIRouteIsAJSON404 guards #699: a path under /api/ that no route
// serves answers 404 with a JSON error body, not the dashboard's HTML page
// with 200, so an API client sees the mistake. Paths outside /api/ still get
// the dashboard.
func TestUnknownAPIRouteIsAJSON404(t *testing.T) {
	t.Parallel()
	srv := api.NewServer(api.ServerConfig{Port: 0})
	if err := srv.Listen(t.Context()); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go srv.Serve(t.Context()) //nolint:errcheck
	base := "http://" + srv.Addr().String()

	for _, path := range []string{"/api/v1/does-not-exist", "/api/v1/sessions/x/does-not-exist", "/api/v2/sessions"} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: status %d, want 404", path, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("GET %s: Content-Type %q, want application/json", path, ct)
		}
		var envelope struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil || !strings.Contains(envelope.Error, path) {
			t.Errorf("GET %s: body %.200q, want a JSON error naming the path", path, body)
		}
	}

	resp, err := http.Get(base + "/sessions/abc")
	if err != nil {
		t.Fatalf("GET /sessions/abc: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Errorf("GET /sessions/abc: %d %q, want the dashboard page (200 text/html)", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}
