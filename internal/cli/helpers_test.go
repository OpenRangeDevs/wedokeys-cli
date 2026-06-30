package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// resolveServer returns a test server that answers POST /api/v1/resolve with the
// given status and body.
func resolveServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// seedConfig writes ~/.wedokeys/config.yml under home with the given keys.
func seedConfig(t *testing.T, home string, kv map[string]string) {
	t.Helper()
	dir := filepath.Join(home, ".wedokeys")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for k, v := range kv {
		fmt.Fprintf(&b, "%s: %s\n", k, v)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeWdkYml writes a project wdk.yml under dir.
func writeWdkYml(t *testing.T, dir, project string, secrets []string) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "project: %s\n", project)
	if secrets != nil {
		b.WriteString("secrets:\n")
		for _, s := range secrets {
			fmt.Fprintf(&b, "  - %s\n", s)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "wdk.yml"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// savedToken reads the token from home's config.yml, or "" if absent.
func savedToken(t *testing.T, home string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(home, ".wedokeys", "config.yml"))
	if err != nil {
		return ""
	}
	var m map[string]any
	if err := yaml.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	if s, ok := m["token"].(string); ok {
		return s
	}
	return ""
}

// newTestApp builds an App writing to buffers, rooted at home/start. Its Exec
// fails the test if invoked (commands that should abort never reach exec); pass
// a capturing func via app.Exec to test the exec path.
func newTestApp(t *testing.T, home, start, stdin string) (app *App, out, errBuf *bytes.Buffer) {
	t.Helper()
	out, errBuf = &bytes.Buffer{}, &bytes.Buffer{}
	app = &App{
		In:       strings.NewReader(stdin),
		Out:      out,
		Err:      errBuf,
		HomeDir:  home,
		StartDir: start,
		Exec: func(argv0 string, argv []string, _ []string) error {
			t.Fatalf("Exec called unexpectedly: %s %v", argv0, argv)
			return nil
		},
	}
	return app, out, errBuf
}

// Common resolve response bodies used across command tests.
const (
	bodyPartialResolve = `{"resolved":{"STRIPE_KEY":"sk_live_abc"},` +
		`"errors":[{"reference":"MISSING_KEY","code":"not_found","message":"Alias not found"}],` +
		`"ttl_seconds":300,"request_id":"req_x"}`
)
