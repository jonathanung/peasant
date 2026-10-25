package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/peasant-labs/peasant/internal/api"
)

// TestJSONErrorBodiesAreLabelledJSON guards #699: an error the API writes as a
// JSON body must carry Content-Type application/json. http.Error stamped
// text/plain over these bodies, so a client that branches on the content type
// treated them as text.
func TestJSONErrorBodiesAreLabelledJSON(t *testing.T) {
	t.Parallel()
	// No store and no data provider: each route below answers with an error.
	srv := api.NewServer(api.ServerConfig{Port: 0})
	if err := srv.Listen(t.Context()); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go srv.Serve(t.Context()) //nolint:errcheck
	base := "http://" + srv.Addr().String()

	for _, path := range []string{
		"/api/v1/sessions/nonexistent",
		"/api/v1/annotations?session_id=any",
		"/api/v1/annotation-types",
	} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode < 400 {
			t.Errorf("GET %s: status %d, want an error status for this fixture", path, resp.StatusCode)
			continue
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("GET %s: %d with Content-Type %q, want application/json", path, resp.StatusCode, ct)
		}
		var envelope struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil || envelope.Error == "" {
			t.Errorf("GET %s: body %.200q is not a JSON error envelope", path, body)
		}
	}
}
