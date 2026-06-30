//go:build parity

// Package parity runs the Go wdk binary and the legacy Ruby CLI against the same
// stub server and asserts identical stdout/stderr/exit code, guarding against
// behavioral drift while both implementations are maintained.
//
// Run with: go test -tags parity ./test/parity/
// Prereqs: Go toolchain, Ruby + `bundle install` in ruby-legacy/.
package parity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot is two levels up from this test file (test/parity/ -> repo root).
func repoRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// stub answers /api/v1/resolve deterministically in request-alias order:
// MISSING_KEY -> error; TLS_KEY -> a multi-line value; otherwise "val-<ALIAS>".
func stub() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Aliases []string `json:"aliases"`
		}
		body, _ := readAll(r)
		_ = json.Unmarshal(body, &req)

		var resolved []string
		var errs []string
		for _, a := range req.Aliases {
			switch a {
			case "MISSING_KEY":
				errs = append(errs, fmt.Sprintf(`{"reference":%q,"code":"not_found","message":"Alias not found"}`, a))
			case "TLS_KEY":
				resolved = append(resolved, fmt.Sprintf("%q:%q", a, "-----BEGIN-----\nx\n-----END-----"))
			default:
				resolved = append(resolved, fmt.Sprintf("%q:%q", a, "val-"+a))
			}
		}
		w.Header().Set("Content-Type", "application/json")
		// Aliases-mode probe (project "__verify__") with no aliases never reaches
		// here in these scenarios; all scenarios send a real project.
		fmt.Fprintf(w, `{"resolved":{%s},"errors":[%s],"ttl_seconds":300,"request_id":"req_x"}`,
			strings.Join(resolved, ","), strings.Join(errs, ","))
	}))
}

func readAll(r *http.Request) ([]byte, error) {
	var b bytes.Buffer
	_, err := b.ReadFrom(r.Body)
	return b.Bytes(), err
}

type result struct {
	stdout string
	stderr string
	code   int
}

func run(t *testing.T, name string, dir string, env []string, argv ...string) result {
	t.Helper()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if ok := asExit(err, &ee); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("%s: run error: %v", name, err)
		}
	}
	return result{stdout: out.String(), stderr: errb.String(), code: code}
}

func asExit(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

// rubyToolchain captures the real Ruby executable and gem path using the
// inherited environment, so the CLI runs can override HOME (for the temp config
// dir) without breaking asdf's gem resolution.
func rubyToolchain(t *testing.T) (rubyExe, gemPath string) {
	t.Helper()
	exeOut, err := exec.Command("ruby", "-e", "print RbConfig.ruby").Output()
	if err != nil {
		t.Fatalf("locate ruby: %v", err)
	}
	gpOut, err := exec.Command("gem", "environment", "gempath").Output()
	if err != nil {
		t.Fatalf("gem gempath: %v", err)
	}
	return strings.TrimSpace(string(exeOut)), strings.TrimSpace(string(gpOut))
}

// baseEnv returns the inherited environment with the variables we set per-run
// removed, so each scenario can supply its own clean values.
func baseEnv() []string {
	var out []string
	for _, e := range os.Environ() {
		switch {
		case strings.HasPrefix(e, "HOME="),
			strings.HasPrefix(e, "GEM_HOME="),
			strings.HasPrefix(e, "GEM_PATH="),
			strings.HasPrefix(e, "WDK_ENV="),
			strings.HasPrefix(e, "KAMAL_DESTINATION="):
			continue
		}
		out = append(out, e)
	}
	return out
}

func TestParity(t *testing.T) {
	root := repoRoot(t)
	rubyLegacy := filepath.Join(root, "ruby-legacy")
	version := readVersion(t, rubyLegacy)
	rubyExe, gemPath := rubyToolchain(t)
	rubyScript := filepath.Join(rubyLegacy, "bin", "wdk")

	// Build the Go binary with the same version the Ruby CLI reports.
	goBin := filepath.Join(t.TempDir(), "wdk")
	build := exec.Command("go", "build",
		"-ldflags", "-X github.com/OpenRangeDevs/wedokeys-cli/internal/version.Version="+version,
		"-o", goBin, "./cmd/wdk")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	srv := stub()
	defer srv.Close()

	scenarios := []struct {
		name    string
		secrets []string // wdk.yml secrets (nil = none)
		env     map[string]string
		args    []string
	}{
		{name: "version", args: []string{"version"}},
		{name: "env-export", secrets: []string{"A", "B"}, env: map[string]string{"WDK_ENV": "production"}, args: []string{"env", "export"}},
		{name: "env-export-partial", secrets: []string{"A", "MISSING_KEY"}, env: map[string]string{"WDK_ENV": "production"}, args: []string{"env", "export"}},
		{name: "env-export-allow-missing", secrets: []string{"A", "MISSING_KEY"}, env: map[string]string{"WDK_ENV": "production"}, args: []string{"env", "export", "--allow-missing"}},
		{name: "env-export-no-project", env: map[string]string{"WDK_ENV": "production"}, args: []string{"env", "export"}},
		{name: "kamal-fetch", env: map[string]string{"KAMAL_DESTINATION": "production"}, args: []string{"kamal-fetch", "A", "B"}},
		{name: "kamal-fetch-multiline", env: map[string]string{"KAMAL_DESTINATION": "production"}, args: []string{"kamal-fetch", "TLS_KEY"}},
		{name: "kamal-fetch-no-aliases", env: map[string]string{"KAMAL_DESTINATION": "production"}, args: []string{"kamal-fetch"}},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			home := t.TempDir()
			seedConfig(t, home, srv.URL)
			project := t.TempDir()
			if sc.name != "env-export-no-project" {
				writeWdkYml(t, project, sc.secrets)
			}

			env := baseEnv()
			env = append(env, "HOME="+home, "GEM_PATH="+gemPath)
			for k, v := range sc.env {
				env = append(env, k+"="+v)
			}

			goRes := run(t, "go", project, env, append([]string{goBin}, sc.args...)...)
			rubyArgv := append([]string{rubyExe, rubyScript}, sc.args...)
			rubyRes := run(t, "ruby", project, env, rubyArgv...)

			if goRes.stdout != rubyRes.stdout {
				t.Errorf("stdout differs:\n go:   %q\n ruby: %q", goRes.stdout, rubyRes.stdout)
			}
			if goRes.stderr != rubyRes.stderr {
				t.Errorf("stderr differs:\n go:   %q\n ruby: %q", goRes.stderr, rubyRes.stderr)
			}
			if goRes.code != rubyRes.code {
				t.Errorf("exit code differs: go=%d ruby=%d", goRes.code, rubyRes.code)
			}
		})
	}
}

func readVersion(t *testing.T, rubyLegacy string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(rubyLegacy, "lib", "wdk", "version.rb"))
	if err != nil {
		t.Fatal(err)
	}
	// VERSION = "0.3.0"
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "VERSION") {
			if i := strings.Index(line, `"`); i >= 0 {
				if j := strings.Index(line[i+1:], `"`); j >= 0 {
					return line[i+1 : i+1+j]
				}
			}
		}
	}
	t.Fatal("could not read VERSION")
	return ""
}

func seedConfig(t *testing.T, home, apiURL string) {
	t.Helper()
	dir := filepath.Join(home, ".wedokeys")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("token: wdk_sat_test\napi_url: %s\n", apiURL)
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeWdkYml(t *testing.T, dir string, secrets []string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("project: my-app\n")
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
