package cli

import (
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
