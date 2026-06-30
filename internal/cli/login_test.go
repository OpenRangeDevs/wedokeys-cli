package cli

import (
	"strings"
	"testing"
)

func TestLoginSavesTokenWhenProbeReturns400(t *testing.T) {
	home, start := t.TempDir(), t.TempDir()
	srv := resolveServer(t, 400, `{"error":"missing_aliases"}`)
	seedConfig(t, home, map[string]string{"api_url": srv.URL})

	app, _, _ := newTestApp(t, home, start, "")
	if err := app.Login(LoginOptions{Token: "wdk_sat_valid"}); err != nil {
		t.Fatalf("Login err = %v, want nil", err)
	}
	if got := savedToken(t, home); got != "wdk_sat_valid" {
		t.Fatalf("saved token = %q, want wdk_sat_valid", got)
	}
}

func TestLoginAbortsWithoutSavingOnServerError(t *testing.T) {
	home, start := t.TempDir(), t.TempDir()
	srv := resolveServer(t, 500, "")
	seedConfig(t, home, map[string]string{"api_url": srv.URL})

	app, _, _ := newTestApp(t, home, start, "")
	if err := app.Login(LoginOptions{Token: "wdk_sat_x"}); err == nil {
		t.Fatal("Login err = nil, want error on server error")
	}
	if got := savedToken(t, home); got != "" {
		t.Fatalf("saved token = %q, want none", got)
	}
}

func TestLoginAbortsWithoutSavingOnInvalidToken(t *testing.T) {
	home, start := t.TempDir(), t.TempDir()
	srv := resolveServer(t, 401, `{"error":"unauthorized"}`)
	seedConfig(t, home, map[string]string{"api_url": srv.URL})

	app, _, _ := newTestApp(t, home, start, "")
	err := app.Login(LoginOptions{Token: "wdk_sat_bad"})
	if err == nil || err.Error() != "Invalid token — authentication failed." {
		t.Fatalf("Login err = %v, want invalid-token error", err)
	}
	if got := savedToken(t, home); got != "" {
		t.Fatalf("saved token = %q, want none", got)
	}
}

func TestLoginAbortsWithoutSavingOnNetworkError(t *testing.T) {
	home, start := t.TempDir(), t.TempDir()
	srv := resolveServer(t, 200, "{}")
	url := srv.URL
	srv.Close() // nothing listening → connection refused
	seedConfig(t, home, map[string]string{"api_url": url})

	app, _, _ := newTestApp(t, home, start, "")
	if err := app.Login(LoginOptions{Token: "wdk_sat_x"}); err == nil {
		t.Fatal("Login err = nil, want network error")
	}
	if got := savedToken(t, home); got != "" {
		t.Fatalf("saved token = %q, want none", got)
	}
}

func TestLoginPromptsForTokenWhenNotProvided(t *testing.T) {
	home, start := t.TempDir(), t.TempDir()
	srv := resolveServer(t, 400, `{"error":"missing_aliases"}`)
	seedConfig(t, home, map[string]string{"api_url": srv.URL})

	app, _, errBuf := newTestApp(t, home, start, "wdk_sat_typed\n")
	if err := app.Login(LoginOptions{}); err != nil {
		t.Fatalf("Login err = %v", err)
	}
	if got := savedToken(t, home); got != "wdk_sat_typed" {
		t.Fatalf("saved token = %q, want wdk_sat_typed", got)
	}
	if errBuf.Len() == 0 {
		t.Error("expected a prompt on stderr")
	}
}

func TestLoginBlankTokenAborts(t *testing.T) {
	home, start := t.TempDir(), t.TempDir()
	app, _, _ := newTestApp(t, home, start, "\n")
	err := app.Login(LoginOptions{})
	if err == nil || err.Error() != "Token cannot be blank." {
		t.Fatalf("err = %v, want blank-token error", err)
	}
}

func TestLoginWithAPIURLFlagSavesServerAndToken(t *testing.T) {
	home, start := t.TempDir(), t.TempDir() // fresh home, no seeded config
	srv := resolveServer(t, 400, `{"error":"missing_aliases"}`)

	app, _, _ := newTestApp(t, home, start, "")
	if err := app.Login(LoginOptions{APIURL: srv.URL, Token: "wdk_sat_valid"}); err != nil {
		t.Fatalf("Login err = %v", err)
	}
	if got := savedToken(t, home); got != "wdk_sat_valid" {
		t.Fatalf("saved token = %q, want wdk_sat_valid", got)
	}
	if got := savedAPIURL(t, home); got != srv.URL {
		t.Fatalf("saved api_url = %q, want %q", got, srv.URL)
	}
}

func TestLoginInteractivePromptsForServerAndToken(t *testing.T) {
	home, start := t.TempDir(), t.TempDir() // fresh home
	srv := resolveServer(t, 400, `{"error":"missing_aliases"}`)

	// Interactive: stdin supplies the server URL, the browser-open answer (no), then the token.
	app, _, errBuf := newTestApp(t, home, start, srv.URL+"\nn\nwdk_sat_typed\n")
	yes := true
	app.Interactive = &yes

	if err := app.Login(LoginOptions{}); err != nil {
		t.Fatalf("Login err = %v", err)
	}
	if got := savedToken(t, home); got != "wdk_sat_typed" {
		t.Fatalf("saved token = %q, want wdk_sat_typed", got)
	}
	if got := savedAPIURL(t, home); got != srv.URL {
		t.Fatalf("saved api_url = %q, want %q", got, srv.URL)
	}
	s := errBuf.String()
	if !strings.Contains(s, "WeDoKeys server") || !strings.Contains(s, "/service_accounts/new") {
		t.Errorf("expected server prompt + token URL on stderr, got %q", s)
	}
}

func TestLoginOffersToOpenBrowser(t *testing.T) {
	home, start := t.TempDir(), t.TempDir()
	srv := resolveServer(t, 400, `{"error":"missing_aliases"}`)

	// Answer "y" to the browser prompt; capture the URL the seam is asked to open.
	app, _, _ := newTestApp(t, home, start, srv.URL+"\ny\nwdk_sat_typed\n")
	yes := true
	app.Interactive = &yes
	var opened string
	app.OpenURL = func(url string) error { opened = url; return nil }

	if err := app.Login(LoginOptions{}); err != nil {
		t.Fatalf("Login err = %v", err)
	}
	if want := srv.URL + "/service_accounts/new"; opened != want {
		t.Fatalf("opened = %q, want %q", opened, want)
	}
}
