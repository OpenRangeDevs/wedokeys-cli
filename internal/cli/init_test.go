package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// initServer stubs GET /api/v1/projects and /api/v1/projects/:slug/aliases.
func initServer(t *testing.T, projectsJSON string, aliasesBySlug map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/projects":
			fmt.Fprint(w, projectsJSON)
		case strings.HasSuffix(r.URL.Path, "/aliases"):
			slug := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/aliases")
			body, ok := aliasesBySlug[slug]
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":"project_not_found","message":"Project not found"}`)
				return
			}
			fmt.Fprint(w, body)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func readWdkYML(t *testing.T, dir string) (project string, secrets []string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "wdk.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Project string   `yaml:"project"`
		Secrets []string `yaml:"secrets"`
	}
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	return parsed.Project, parsed.Secrets
}

// initApp wires an App against an init stub, with home (token+api_url) and a
// project dir to write wdk.yml into. interactive controls prompt mode.
func initApp(t *testing.T, projectsJSON string, aliasesBySlug map[string]string, stdin string, interactive bool) (*App, string) {
	t.Helper()
	home := t.TempDir()
	project := t.TempDir()
	srv := initServer(t, projectsJSON, aliasesBySlug)
	seedConfig(t, home, map[string]string{"token": "wdk_sat_test", "api_url": srv.URL})
	app, _, _ := newTestApp(t, home, project, stdin)
	app.Interactive = &interactive
	return app, project
}

const (
	oneProject  = `{"projects":[{"slug":"demo-store","name":"Demo Store"}]}`
	twoProjects = `{"projects":[{"slug":"demo-store","name":"Demo Store"},{"slug":"demo-analytics","name":"Demo Analytics"}]}`
)

var demoAliases = map[string]string{
	"demo-store":     `{"aliases":[{"name":"STRIPE_KEY","environment":"development"},{"name":"POSTGRES_PASSWORD","environment":"development"}]}`,
	"demo-analytics": `{"aliases":[{"name":"ANALYTICS_KEY","environment":"development"}]}`,
}

func TestInitAutoSelectsSingleProjectAndPicksSecret(t *testing.T) {
	app, dir := initApp(t, oneProject, demoAliases, "1\n", true) // pick secret #1
	if err := app.Init(InitOptions{}); err != nil {
		t.Fatalf("Init err = %v", err)
	}
	project, secrets := readWdkYML(t, dir)
	if project != "demo-store" || !reflect.DeepEqual(secrets, []string{"STRIPE_KEY"}) {
		t.Fatalf("wdk.yml = %q %v, want demo-store [STRIPE_KEY]", project, secrets)
	}
}

func TestInitMultipleProjectsPrompt(t *testing.T) {
	app, dir := initApp(t, twoProjects, demoAliases, "2\nall\n", true) // project #2, then all secrets
	if err := app.Init(InitOptions{}); err != nil {
		t.Fatalf("Init err = %v", err)
	}
	project, secrets := readWdkYML(t, dir)
	if project != "demo-analytics" || !reflect.DeepEqual(secrets, []string{"ANALYTICS_KEY"}) {
		t.Fatalf("wdk.yml = %q %v, want demo-analytics [ANALYTICS_KEY]", project, secrets)
	}
}

func TestInitWithFlags(t *testing.T) {
	app, dir := initApp(t, oneProject, demoAliases, "", false)
	if err := app.Init(InitOptions{Project: "demo-store", Secrets: []string{"STRIPE_KEY"}}); err != nil {
		t.Fatalf("Init err = %v", err)
	}
	project, secrets := readWdkYML(t, dir)
	if project != "demo-store" || !reflect.DeepEqual(secrets, []string{"STRIPE_KEY"}) {
		t.Fatalf("wdk.yml = %q %v", project, secrets)
	}
}

func TestInitAllFlag(t *testing.T) {
	app, dir := initApp(t, oneProject, demoAliases, "", false)
	if err := app.Init(InitOptions{Project: "demo-store", All: true}); err != nil {
		t.Fatalf("Init err = %v", err)
	}
	_, secrets := readWdkYML(t, dir)
	if !reflect.DeepEqual(secrets, []string{"STRIPE_KEY", "POSTGRES_PASSWORD"}) {
		t.Fatalf("secrets = %v, want all", secrets)
	}
}

func TestInitNudgesWhenNotLoggedIn(t *testing.T) {
	home, project := t.TempDir(), t.TempDir() // no config
	app, _, _ := newTestApp(t, home, project, "")
	err := app.Init(InitOptions{})
	if err == nil || !strings.Contains(err.Error(), "wdk login") {
		t.Fatalf("err = %v, want a 'wdk login' nudge", err)
	}
}

func TestInitRefusesToOverwrite(t *testing.T) {
	app, dir := initApp(t, oneProject, demoAliases, "", false)
	if err := os.WriteFile(filepath.Join(dir, "wdk.yml"), []byte("project: existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := app.Init(InitOptions{Project: "demo-store", All: true})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want overwrite refusal", err)
	}
}
