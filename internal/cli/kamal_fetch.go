package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/OpenRangeDevs/wedokeys-cli/internal/client"
	"github.com/OpenRangeDevs/wedokeys-cli/internal/config"
	"github.com/OpenRangeDevs/wedokeys-cli/internal/resolve"
)

// KamalOptions configures the kamal-fetch command.
type KamalOptions struct {
	Account      string // ignored; account is derived from the token
	From         string // project/environment (or just project)
	Env          string
	AllowMissing bool
}

// KamalFetch is the Kamal secrets adapter: it resolves the named aliases and
// prints them as NAME=value lines for Kamal to parse.
func (a *App) KamalFetch(opts KamalOptions, aliases []string) error {
	if len(aliases) == 0 {
		return errors.New("Error: at least one secret name is required.")
	}

	cfg, err := a.newConfig(opts.Env)
	if err != nil {
		return err
	}
	project, environment, err := resolveProjectAndEnv(cfg, opts.From)
	if err != nil {
		return wrapErr(err)
	}
	token, err := cfg.RequireToken()
	if err != nil {
		return wrapErr(err)
	}

	result, err := client.New(cfg.APIURL(), token).ResolveByAliases(aliases, project, environment)
	if err != nil {
		return wrapErr(err)
	}

	// Kamal expects every requested secret; missing ones must fail the fetch.
	if err := resolve.Check(a.err(), result, opts.AllowMissing); err != nil {
		return err
	}
	// Kamal parses one NAME=value per line; multi-line or non-string values
	// cannot be expressed that way, so fail hard rather than emit a corrupt line.
	if err := resolve.AssertSingleLine(a.err(), result.Resolved); err != nil {
		return err
	}

	for _, s := range result.Resolved {
		fmt.Fprintf(a.out(), "%s=%s\n", s.Name, s.Value)
	}
	return nil
}

func resolveProjectAndEnv(cfg *config.Config, from string) (project, environment string, err error) {
	if strings.Contains(from, "/") {
		parts := strings.SplitN(from, "/", 2)
		return parts[0], parts[1], nil
	}
	if from != "" {
		environment, err = cfg.RequireEnvironment()
		return from, environment, err
	}
	project, err = cfg.RequireProjectSlug()
	if err != nil {
		return "", "", err
	}
	environment, err = cfg.RequireEnvironment()
	if err != nil {
		return "", "", err
	}
	return project, environment, nil
}

func newKamalFetchCmd(app *App) *cobra.Command {
	var opts KamalOptions
	cmd := &cobra.Command{
		Use:    "kamal-fetch ALIAS...",
		Short:  "Kamal secrets adapter — fetch secrets by alias (internal use)",
		Hidden: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return app.KamalFetch(opts, args)
		},
	}
	cmd.Flags().StringVar(&opts.Account, "account", "", "Ignored; account is derived from token")
	cmd.Flags().StringVar(&opts.From, "from", "", "project/environment (falls back to wdk.yml + env inference)")
	cmd.Flags().StringVarP(&opts.Env, "env", "e", "", "Environment override")
	cmd.Flags().BoolVar(&opts.AllowMissing, "allow-missing", false, "Proceed even if some aliases fail to resolve")
	return cmd
}
