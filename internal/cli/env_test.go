package cli

import (
	"bytes"
	"strings"
	"testing"
)

// envSetup builds an App wired to a resolve server returning (status, body),
// with home (token+api_url) and a project wdk.yml listing secrets, and
// WDK_ENV=production. Returns the app and its stdout/stderr buffers.
func envSetup(t *testing.T, secrets []string, status int, body string) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	home, project := t.TempDir(), t.TempDir()
	srv := resolveServer(t, status, body)
	seedConfig(t, home, map[string]string{"token": "wdk_sat_test", "api_url": srv.URL})
	writeWdkYml(t, project, "my-app", secrets)
	t.Setenv("WDK_ENV", "production")
	return newTestApp(t, home, project, "")
}

func TestEnvExecAbortsWithUsageWhenNoArgs(t *testing.T) {
	app, _, _ := newTestApp(t, t.TempDir(), t.TempDir(), "")
	err := app.EnvExec(EnvOptions{}, nil)
	if err == nil || err.Error() != "Usage: wdk env exec -- COMMAND [ARGS]" {
		t.Fatalf("err = %v, want usage error", err)
	}
}

func TestEnvExecAbortsWhenSomeAliasesUnresolved(t *testing.T) {
	app, _, _ := envSetup(t, []string{"STRIPE_KEY", "MISSING_KEY"}, 200, bodyPartialResolve)
	err := app.EnvExec(EnvOptions{}, []string{"true"})
	if err == nil || !strings.Contains(err.Error(), "could not be resolved") {
		t.Fatalf("err = %v, want unresolved-abort", err)
	}
}

func TestEnvExecNetworkErrorOnConnectionRefused(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	srv := resolveServer(t, 200, "{}")
	url := srv.URL
	srv.Close()
	seedConfig(t, home, map[string]string{"token": "wdk_sat_test", "api_url": url})
	writeWdkYml(t, project, "my-app", []string{"STRIPE_KEY"})
	t.Setenv("WDK_ENV", "production")

	app, _, _ := newTestApp(t, home, project, "")
	err := app.EnvExec(EnvOptions{}, []string{"true"})
	if err == nil || !strings.HasPrefix(err.Error(), "Network error:") {
		t.Fatalf("err = %v, want Network error prefix", err)
	}
}

func TestEnvExecInjectsSecretsAndExecs(t *testing.T) {
	app, _, _ := envSetup(t, []string{"STRIPE_KEY"}, 200,
		`{"resolved":{"STRIPE_KEY":"sk_live_abc"},"errors":[]}`)

	var gotArgv0 string
	var gotEnv []string
	app.Exec = func(argv0 string, _ []string, env []string) error {
		gotArgv0, gotEnv = argv0, env
		return nil
	}

	if err := app.EnvExec(EnvOptions{}, []string{"true"}); err != nil {
		t.Fatalf("EnvExec err = %v", err)
	}
	if !strings.HasSuffix(gotArgv0, "true") {
		t.Errorf("exec argv0 = %q, want a path ending in 'true'", gotArgv0)
	}
	if !envContains(gotEnv, "STRIPE_KEY=sk_live_abc") {
		t.Errorf("exec env missing STRIPE_KEY=sk_live_abc: %v", lastEnv(gotEnv))
	}
}

func TestEnvExportAbortsWhenSomeAliasesUnresolved(t *testing.T) {
	app, _, errBuf := envSetup(t, []string{"STRIPE_KEY", "MISSING_KEY"}, 200, bodyPartialResolve)
	err := app.EnvExport(EnvOptions{})
	if err == nil || !strings.Contains(err.Error(), "could not be resolved") {
		t.Fatalf("err = %v, want unresolved-abort", err)
	}
	// denials print to stderr before aborting
	if s := errBuf.String(); !strings.Contains(s, "MISSING_KEY") || !strings.Contains(s, "Alias not found") {
		t.Errorf("stderr = %q, want MISSING_KEY / Alias not found", s)
	}
}

func TestEnvExportAllowMissingPrintsPartialResults(t *testing.T) {
	app, out, _ := envSetup(t, []string{"STRIPE_KEY", "MISSING_KEY"}, 200, bodyPartialResolve)
	if err := app.EnvExport(EnvOptions{AllowMissing: true}); err != nil {
		t.Fatalf("EnvExport err = %v", err)
	}
	if s := out.String(); !strings.Contains(s, "export STRIPE_KEY=sk_live_abc") {
		t.Errorf("stdout = %q, want export STRIPE_KEY=sk_live_abc", s)
	}
}

func TestEnvExportAbortsCleanlyOnAPIError(t *testing.T) {
	app, _, _ := envSetup(t, []string{"STRIPE_KEY"}, 400,
		`{"error":"project_not_found","message":"Project not found"}`)
	err := app.EnvExport(EnvOptions{})
	if err == nil || err.Error() != "API error: Project not found" {
		t.Fatalf("err = %v, want 'API error: Project not found'", err)
	}
}

func envContains(env []string, want string) bool {
	for _, e := range env {
		if e == want {
			return true
		}
	}
	return false
}

func lastEnv(env []string) []string {
	if len(env) > 5 {
		return env[len(env)-5:]
	}
	return env
}
