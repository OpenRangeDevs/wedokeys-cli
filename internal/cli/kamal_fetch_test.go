package cli

import (
	"bytes"
	"strings"
	"testing"
)

// kamalApp builds an App wired to a resolve server returning (status, body),
// rooted at start, with home holding token+api_url.
func kamalApp(t *testing.T, start string, status int, body string) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	home := t.TempDir()
	srv := resolveServer(t, status, body)
	seedConfig(t, home, map[string]string{"token": "wdk_sat_test", "api_url": srv.URL})
	return newTestApp(t, home, start, "")
}

func projectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeWdkYml(t, dir, "my-app", nil)
	return dir
}

func TestKamalOutputsNameEqualsValueLines(t *testing.T) {
	t.Setenv("KAMAL_DESTINATION", "production")
	app, out, _ := kamalApp(t, projectDir(t), 200,
		`{"resolved":{"POSTGRES_PASSWORD":"pg_secret","STRIPE_KEY":"sk_live_abc"},"errors":[]}`)

	if err := app.KamalFetch(KamalOptions{}, []string{"POSTGRES_PASSWORD", "STRIPE_KEY"}); err != nil {
		t.Fatalf("KamalFetch err = %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "POSTGRES_PASSWORD=pg_secret") || !strings.Contains(s, "STRIPE_KEY=sk_live_abc") {
		t.Fatalf("stdout = %q, want both NAME=value lines", s)
	}
}

func TestKamalParsesProjectAndEnvFromFromOption(t *testing.T) {
	app, out, _ := kamalApp(t, t.TempDir(), 200,
		`{"resolved":{"DB_URL":"postgres://localhost"},"errors":[]}`)

	if err := app.KamalFetch(KamalOptions{From: "other-app/staging"}, []string{"DB_URL"}); err != nil {
		t.Fatalf("KamalFetch err = %v", err)
	}
	if s := out.String(); !strings.Contains(s, "DB_URL=postgres://localhost") {
		t.Fatalf("stdout = %q, want DB_URL line", s)
	}
}

func TestKamalAbortsWhenSomeAliasesUnresolved(t *testing.T) {
	t.Setenv("KAMAL_DESTINATION", "production")
	app, _, errBuf := kamalApp(t, projectDir(t), 200, bodyPartialResolve)

	err := app.KamalFetch(KamalOptions{}, []string{"STRIPE_KEY", "MISSING_KEY"})
	if err == nil || !strings.Contains(err.Error(), "could not be resolved") {
		t.Fatalf("err = %v, want unresolved-abort", err)
	}
	if !strings.Contains(errBuf.String(), "MISSING_KEY") {
		t.Errorf("stderr = %q, want MISSING_KEY", errBuf.String())
	}
}

func TestKamalAllowMissingProceeds(t *testing.T) {
	t.Setenv("KAMAL_DESTINATION", "production")
	app, out, _ := kamalApp(t, projectDir(t), 200, bodyPartialResolve)

	if err := app.KamalFetch(KamalOptions{AllowMissing: true}, []string{"STRIPE_KEY", "MISSING_KEY"}); err != nil {
		t.Fatalf("KamalFetch err = %v", err)
	}
	if s := out.String(); !strings.Contains(s, "STRIPE_KEY=sk_live_abc") {
		t.Fatalf("stdout = %q, want STRIPE_KEY line", s)
	}
}

func TestKamalAbortsWhenValueContainsNewline(t *testing.T) {
	t.Setenv("KAMAL_DESTINATION", "production")
	app, out, errBuf := kamalApp(t, projectDir(t), 200,
		`{"resolved":{"TLS_KEY":"-----BEGIN KEY-----\nabc\n-----END KEY-----"},"errors":[]}`)

	err := app.KamalFetch(KamalOptions{}, []string{"TLS_KEY"})
	if err == nil {
		t.Fatal("err = nil, want multiline abort")
	}
	if !strings.Contains(errBuf.String(), "TLS_KEY") {
		t.Errorf("stderr = %q, want TLS_KEY", errBuf.String())
	}
	if strings.Contains(out.String(), "BEGIN KEY") {
		t.Errorf("stdout = %q, must not emit a partial/corrupt line", out.String())
	}
}

func TestKamalAbortsWhenValueIsWholeSecretHash(t *testing.T) {
	t.Setenv("KAMAL_DESTINATION", "production")
	app, _, errBuf := kamalApp(t, projectDir(t), 200,
		`{"resolved":{"DB":{"user":"u","password":"p"}},"errors":[]}`)

	err := app.KamalFetch(KamalOptions{}, []string{"DB"})
	if err == nil {
		t.Fatal("err = nil, want abort for whole-secret hash value")
	}
	if !strings.Contains(errBuf.String(), "DB") {
		t.Errorf("stderr = %q, want DB", errBuf.String())
	}
}

func TestKamalErrorsWhenNoAliasesGiven(t *testing.T) {
	app, _, _ := kamalApp(t, projectDir(t), 200, "{}")
	err := app.KamalFetch(KamalOptions{}, nil)
	if err == nil || err.Error() != "Error: at least one secret name is required." {
		t.Fatalf("err = %v, want no-aliases error", err)
	}
}
