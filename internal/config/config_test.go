package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// --- User config (token + api_url) ---

func TestLoadsTokenFromConfigFile(t *testing.T) {
	home := t.TempDir()
	writeUserConfig(t, home, map[string]string{"token": "wdk_sat_abc123"})

	c := newConfig(t, WithHomeDir(home))
	if got := c.Token(); got != "wdk_sat_abc123" {
		t.Fatalf("Token() = %q, want %q", got, "wdk_sat_abc123")
	}
}

func TestLoadsAPIURLFromConfigFile(t *testing.T) {
	home := t.TempDir()
	writeUserConfig(t, home, map[string]string{"api_url": "https://example.wedokeys.app"})

	c := newConfig(t, WithHomeDir(home))
	if got := c.APIURL(); got != "https://example.wedokeys.app" {
		t.Fatalf("APIURL() = %q, want %q", got, "https://example.wedokeys.app")
	}
}

func TestDefaultAPIURLIsProduction(t *testing.T) {
	home := t.TempDir()
	writeUserConfig(t, home, map[string]string{"token": "wdk_sat_abc"})

	c := newConfig(t, WithHomeDir(home))
	if got := c.APIURL(); got != DefaultAPIURL {
		t.Fatalf("APIURL() = %q, want %q", got, DefaultAPIURL)
	}
}

func TestRequireTokenErrorsWhenNoConfig(t *testing.T) {
	home := t.TempDir()
	c := newConfig(t, WithHomeDir(home))
	if _, err := c.RequireToken(); err != ErrNotConfigured {
		t.Fatalf("RequireToken() err = %v, want ErrNotConfigured", err)
	}
}

func TestConfigFileHas0600Permissions(t *testing.T) {
	home := t.TempDir()
	writeUserConfig(t, home, map[string]string{"token": "wdk_sat_abc"})

	if mode := statMode(t, userConfigPath(home)); mode != 0o600 {
		t.Fatalf("config.yml mode = %#o, want 0600", mode)
	}
}

// --- Save ---

func TestSavePreservesExistingAPIURL(t *testing.T) {
	home := t.TempDir()
	writeUserConfig(t, home, map[string]string{"token": "wdk_sat_old", "api_url": "http://localhost:3000"})

	if err := newConfig(t, WithHomeDir(home)).Save("wdk_sat_new", ""); err != nil {
		t.Fatal(err)
	}

	reloaded := newConfig(t, WithHomeDir(home))
	if got := reloaded.Token(); got != "wdk_sat_new" {
		t.Fatalf("Token() = %q, want wdk_sat_new", got)
	}
	if got := reloaded.APIURL(); got != "http://localhost:3000" {
		t.Fatalf("APIURL() = %q, want http://localhost:3000 (preserved)", got)
	}
}

func TestSaveWithExplicitAPIURLOverrides(t *testing.T) {
	home := t.TempDir()
	writeUserConfig(t, home, map[string]string{"token": "wdk_sat_old", "api_url": "http://localhost:3000"})

	if err := newConfig(t, WithHomeDir(home)).Save("wdk_sat_new", "https://other.example.com"); err != nil {
		t.Fatal(err)
	}

	if got := newConfig(t, WithHomeDir(home)).APIURL(); got != "https://other.example.com" {
		t.Fatalf("APIURL() = %q, want https://other.example.com", got)
	}
}

func TestSaveOnFreshHomeWritesTokenOnly(t *testing.T) {
	home := t.TempDir()
	if err := newConfig(t, WithHomeDir(home)).Save("wdk_sat_fresh", ""); err != nil {
		t.Fatal(err)
	}

	path := userConfigPath(home)
	var parsed map[string]any
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if want := map[string]any{"token": "wdk_sat_fresh"}; !reflect.DeepEqual(parsed, want) {
		t.Fatalf("written config = %v, want %v", parsed, want)
	}
	if mode := statMode(t, path); mode != 0o600 {
		t.Fatalf("config.yml mode = %#o, want 0600", mode)
	}
}

// --- Project config (wdk.yml discovery) ---

func TestDiscoversWdkYmlInCurrentDir(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	writeProjectConfig(t, proj, "my-app", nil)

	c := newConfig(t, WithHomeDir(home), WithStartDir(proj))
	if got := c.ProjectSlug(); got != "my-app" {
		t.Fatalf("ProjectSlug() = %q, want my-app", got)
	}
}

func TestDiscoversWdkYmlInParentDir(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	sub := filepath.Join(proj, "app", "controllers")
	mustMkdirAll(t, sub)
	writeProjectConfig(t, proj, "parent-app", nil)

	c := newConfig(t, WithHomeDir(home), WithStartDir(sub))
	if got := c.ProjectSlug(); got != "parent-app" {
		t.Fatalf("ProjectSlug() = %q, want parent-app", got)
	}
}

func TestEmptyProjectSlugWhenNoWdkYml(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	c := newConfig(t, WithHomeDir(home), WithStartDir(proj))
	if got := c.ProjectSlug(); got != "" {
		t.Fatalf("ProjectSlug() = %q, want empty", got)
	}
}

func TestRequireProjectSlugErrorsWhenMissing(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	c := newConfig(t, WithHomeDir(home), WithStartDir(proj))
	if _, err := c.RequireProjectSlug(); err != ErrMissingProject {
		t.Fatalf("RequireProjectSlug() err = %v, want ErrMissingProject", err)
	}
}

// --- Environment resolution ---

func TestEnvFromExplicitOption(t *testing.T) {
	c := newConfig(t, WithHomeDir(t.TempDir()), WithEnvOption("staging"))
	if got := c.Environment(); got != "staging" {
		t.Fatalf("Environment() = %q, want staging", got)
	}
}

func TestEnvFromWDKEnv(t *testing.T) {
	t.Setenv("WDK_ENV", "development")
	t.Setenv("KAMAL_DESTINATION", "")
	c := newConfig(t, WithHomeDir(t.TempDir()))
	if got := c.Environment(); got != "development" {
		t.Fatalf("Environment() = %q, want development", got)
	}
}

func TestEnvFromKamalDestination(t *testing.T) {
	t.Setenv("WDK_ENV", "")
	t.Setenv("KAMAL_DESTINATION", "production")
	c := newConfig(t, WithHomeDir(t.TempDir()))
	if got := c.Environment(); got != "production" {
		t.Fatalf("Environment() = %q, want production", got)
	}
}

func TestExplicitEnvBeatsWDKEnv(t *testing.T) {
	t.Setenv("WDK_ENV", "staging")
	c := newConfig(t, WithHomeDir(t.TempDir()), WithEnvOption("production"))
	if got := c.Environment(); got != "production" {
		t.Fatalf("Environment() = %q, want production", got)
	}
}

func TestWDKEnvBeatsKamalDestination(t *testing.T) {
	t.Setenv("WDK_ENV", "staging")
	t.Setenv("KAMAL_DESTINATION", "production")
	c := newConfig(t, WithHomeDir(t.TempDir()))
	if got := c.Environment(); got != "staging" {
		t.Fatalf("Environment() = %q, want staging", got)
	}
}

func TestRequireEnvironmentErrorsWhenNoSource(t *testing.T) {
	t.Setenv("WDK_ENV", "")
	t.Setenv("KAMAL_DESTINATION", "")
	c := newConfig(t, WithHomeDir(t.TempDir()))
	if _, err := c.RequireEnvironment(); err != ErrMissingEnvironment {
		t.Fatalf("RequireEnvironment() err = %v, want ErrMissingEnvironment", err)
	}
}

// --- Secrets list ---

func TestLoadsSecretsList(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	writeProjectConfig(t, proj, "my-app", []string{"POSTGRES_PASSWORD", "STRIPE_KEY"})

	c := newConfig(t, WithHomeDir(home), WithStartDir(proj))
	if got := c.Secrets(); !reflect.DeepEqual(got, []string{"POSTGRES_PASSWORD", "STRIPE_KEY"}) {
		t.Fatalf("Secrets() = %v, want [POSTGRES_PASSWORD STRIPE_KEY]", got)
	}
}

func TestSecretsDefaultsToEmpty(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	writeProjectConfig(t, proj, "my-app", nil)

	c := newConfig(t, WithHomeDir(home), WithStartDir(proj))
	if got := c.Secrets(); len(got) != 0 {
		t.Fatalf("Secrets() = %v, want empty", got)
	}
}

// --- helpers ---

func newConfig(t *testing.T, opts ...Option) *Config {
	t.Helper()
	c, err := New(opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func userConfigPath(home string) string {
	return filepath.Join(home, ".wedokeys", "config.yml")
}

func statMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Mode().Perm()
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeUserConfig(t *testing.T, home string, kv map[string]string) {
	t.Helper()
	dir := filepath.Join(home, ".wedokeys")
	mustMkdirAll(t, dir)
	var b strings.Builder
	for k, v := range kv {
		fmt.Fprintf(&b, "%s: %s\n", k, v)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeProjectConfig(t *testing.T, dir, project string, secrets []string) {
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
