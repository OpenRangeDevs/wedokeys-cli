package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultAPIURL is the production WeDoKeys endpoint used when the user config
// does not override api_url.
const DefaultAPIURL = "https://app.wedokeys.com"

// Configuration errors. ErrNotConfigured / ErrMissingEnvironment match the Ruby
// CLI verbatim (parity); ErrMissingProject nudges the Go-only `wdk init`.
var (
	ErrNotConfigured      = errors.New("No token found. Run `wdk login` first.")
	ErrMissingProject     = errors.New("No wdk.yml found. Run `wdk init` to create one.")
	ErrMissingEnvironment = errors.New("Environment not set. Pass --env, or set WDK_ENV / KAMAL_DESTINATION.")
)

// Config resolves user settings (~/.wedokeys/config.yml), project settings
// (wdk.yml), and the active environment.
type Config struct {
	homeDir   string
	startDir  string
	envOption string

	userLoaded bool
	user       map[string]any

	projLoaded bool
	proj       *projectFile
}

type projectFile struct {
	Project string   `yaml:"project"`
	Secrets []string `yaml:"secrets"`
}

// Option configures a Config.
type Option func(*Config)

// WithHomeDir overrides the home directory (default os.UserHomeDir).
func WithHomeDir(dir string) Option { return func(c *Config) { c.homeDir = dir } }

// WithStartDir overrides the directory wdk.yml discovery starts from (default cwd).
func WithStartDir(dir string) Option { return func(c *Config) { c.startDir = dir } }

// WithEnvOption sets the explicit --env value (highest precedence).
func WithEnvOption(env string) Option { return func(c *Config) { c.envOption = env } }

// New builds a Config, defaulting home to the user's home directory and the
// start directory to the current working directory.
func New(opts ...Option) (*Config, error) {
	c := &Config{}
	for _, o := range opts {
		o(c)
	}
	if c.homeDir == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		c.homeDir = h
	}
	if c.startDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		c.startDir = wd
	}
	return c, nil
}

// --- User config ---

// Token returns the stored token, or "" if not configured.
func (c *Config) Token() string {
	if v, ok := c.userConfig()["token"].(string); ok {
		return v
	}
	return ""
}

// RequireToken returns the token or ErrNotConfigured.
func (c *Config) RequireToken() (string, error) {
	if t := c.Token(); t != "" {
		return t, nil
	}
	return "", ErrNotConfigured
}

// APIURL returns the configured api_url or DefaultAPIURL.
func (c *Config) APIURL() string {
	if v, ok := c.userConfig()["api_url"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return DefaultAPIURL
}

// Save writes the token to the user config, merging over any existing file so
// settings like api_url survive a re-login. An empty apiURL leaves the existing
// value untouched. The file is written 0600.
func (c *Config) Save(token, apiURL string) error {
	data := c.userConfig()
	data["token"] = token
	if apiURL != "" {
		data["api_url"] = apiURL
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	path := c.configFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return err
	}
	// WriteFile only applies perms on creation; enforce 0600 on re-save too.
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}

	c.user = data
	c.userLoaded = true
	return nil
}

func (c *Config) userConfig() map[string]any {
	if c.userLoaded {
		return c.user
	}
	c.userLoaded = true
	c.user = map[string]any{}

	raw, err := os.ReadFile(c.configFilePath())
	if err != nil {
		return c.user // missing file → empty config
	}
	var m map[string]any
	if err := yaml.Unmarshal(raw, &m); err == nil && m != nil {
		c.user = m
	}
	return c.user
}

func (c *Config) configFilePath() string {
	return filepath.Join(c.homeDir, ".wedokeys", "config.yml")
}

// --- Project config ---

// ProjectSlug returns the project slug from wdk.yml, or "" if none is found.
func (c *Config) ProjectSlug() string {
	if p := c.projectConfig(); p != nil {
		return p.Project
	}
	return ""
}

// RequireProjectSlug returns the slug or ErrMissingProject.
func (c *Config) RequireProjectSlug() (string, error) {
	if s := c.ProjectSlug(); s != "" {
		return s, nil
	}
	return "", ErrMissingProject
}

// Secrets returns the alias list from wdk.yml, or an empty slice.
func (c *Config) Secrets() []string {
	if p := c.projectConfig(); p != nil && p.Secrets != nil {
		return p.Secrets
	}
	return []string{}
}

func (c *Config) projectConfig() *projectFile {
	if c.projLoaded {
		return c.proj
	}
	c.projLoaded = true

	path := c.findWdkYml()
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pf projectFile
	if err := yaml.Unmarshal(raw, &pf); err != nil {
		return nil
	}
	c.proj = &pf
	return c.proj
}

// findWdkYml walks up from startDir looking for wdk.yml, like .git discovery.
func (c *Config) findWdkYml() string {
	dir, err := filepath.Abs(c.startDir)
	if err != nil {
		dir = c.startDir
	}
	for {
		candidate := filepath.Join(dir, "wdk.yml")
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// --- Environment ---

// Environment resolves the active environment: --env, then WDK_ENV, then
// KAMAL_DESTINATION (first non-empty). Returns "" if none is set.
func (c *Config) Environment() string {
	return firstNonEmpty(c.envOption, os.Getenv("WDK_ENV"), os.Getenv("KAMAL_DESTINATION"))
}

// RequireEnvironment returns the environment or ErrMissingEnvironment.
func (c *Config) RequireEnvironment() (string, error) {
	if e := c.Environment(); e != "" {
		return e, nil
	}
	return "", ErrMissingEnvironment
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
