package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/OpenRangeDevs/wedokeys-cli/internal/client"
	"github.com/OpenRangeDevs/wedokeys-cli/internal/config"
)

// App holds the I/O streams and overridable seams the commands run against, so
// they can be driven directly in tests the way the Ruby command objects were.
type App struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer

	// HomeDir / StartDir override config discovery; "" uses the process defaults.
	HomeDir  string
	StartDir string

	// Exec replaces the current process (env exec / subshell). Defaults to
	// syscall.Exec; tests inject a capturing stub.
	Exec func(argv0 string, argv []string, envv []string) error
}

func (a *App) in() io.Reader {
	if a.In != nil {
		return a.In
	}
	return os.Stdin
}

func (a *App) out() io.Writer {
	if a.Out != nil {
		return a.Out
	}
	return os.Stdout
}

func (a *App) err() io.Writer {
	if a.Err != nil {
		return a.Err
	}
	return os.Stderr
}

func (a *App) execFn() func(string, []string, []string) error {
	if a.Exec != nil {
		return a.Exec
	}
	return syscall.Exec
}

// newConfig builds a config.Config honoring the App's home/start overrides and
// the optional explicit --env value.
func (a *App) newConfig(envOption string) (*config.Config, error) {
	var opts []config.Option
	if a.HomeDir != "" {
		opts = append(opts, config.WithHomeDir(a.HomeDir))
	}
	if a.StartDir != "" {
		opts = append(opts, config.WithStartDir(a.StartDir))
	}
	if envOption != "" {
		opts = append(opts, config.WithEnvOption(envOption))
	}
	return config.New(opts...)
}

// wrapErr maps client/config errors to the prefixed messages the Ruby CLI
// printed. Guard errors (resolve.*Error) and other plain errors pass through
// unchanged so their messages print verbatim.
func wrapErr(err error) error {
	var authErr *client.AuthError
	var netErr *client.NetworkError
	var apiErr *client.APIError
	switch {
	case errors.As(err, &authErr):
		return fmt.Errorf("Authentication error: %s", authErr)
	case errors.As(err, &netErr):
		return fmt.Errorf("Network error: %s", netErr)
	case errors.As(err, &apiErr):
		return fmt.Errorf("API error: %s", apiErr)
	case errors.Is(err, config.ErrNotConfigured),
		errors.Is(err, config.ErrMissingProject),
		errors.Is(err, config.ErrMissingEnvironment):
		return fmt.Errorf("Configuration error: %s", err)
	default:
		return err
	}
}

// envPairs renders resolved secrets as NAME=value strings for exec environments.
func envPairs(secrets []client.Secret) []string {
	pairs := make([]string, 0, len(secrets))
	for _, s := range secrets {
		pairs = append(pairs, s.Name+"="+s.Value)
	}
	return pairs
}

// shellEscape mirrors Ruby's Shellwords.escape: backslash-escape every byte
// outside the safe set, and wrap newlines in single quotes.
func shellEscape(s string) string {
	if s == "" {
		return "''"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n':
			b.WriteString("'\n'")
		case isShellSafe(r):
			b.WriteRune(r)
		default:
			b.WriteByte('\\')
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isShellSafe(r rune) bool {
	switch {
	case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		return true
	default:
		return strings.ContainsRune("_-.,:/@", r)
	}
}
