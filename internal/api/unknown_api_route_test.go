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

// TestWrongMethodOnAPIRouteIs405 proves a route that exists, asked with a
// method it does not serve, answers 405 with an Allow header naming the
// methods it does serve, instead of the catch-all's "no API route" 404. A bare
// /api keeps the dashboard page instead of a redirect to /api/.
func TestWrongMethodOnAPIRouteIs405(t *testing.T) {
	t.Parallel()
	srv := api.NewServer(api.ServerConfig{Port: 0})
	if err := srv.Listen(t.Context()); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go srv.Serve(t.Context()) //nolint:errcheck
	base := "http://" + srv.Addr().String()
	for _, c := range []struct {
		method, path string
		allow        []string
	}{
		{http.MethodGet, "/api/v1/shutdown", []string{"POST"}},
		{http.MethodDelete, "/api/v1/settings", []string{"GET", "PATCH"}},
	} {
		req, err := http.NewRequest(c.method, base+c.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", c.method, c.path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: status %d, want 405; body %.200q", c.method, c.path, resp.StatusCode, body)
			continue
		}
		allow := resp.Header.Get("Allow")
		for _, m := range c.allow {
			if !strings.Contains(allow, m) {
				t.Errorf("%s %s: Allow %q lacks %s", c.method, c.path, allow, m)
			}
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("%s %s: Content-Type %q, want application/json", c.method, c.path, ct)
		}
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(base + "/api")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Errorf("GET /api: %d %q, want the dashboard page (200 text/html)", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}
