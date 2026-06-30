package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/OpenRangeDevs/wedokeys-cli/internal/client"
	"github.com/OpenRangeDevs/wedokeys-cli/internal/resolve"
)

// EnvOptions configures the env subcommands.
type EnvOptions struct {
	Env          string
	AllowMissing bool
}

// resolveFromProject runs the shared config→resolve→guard pipeline used by the
// project-scoped commands (env exec, env export, subshell). emptyAliasesMsg is
// the error returned when wdk.yml lists no secrets.
func (a *App) resolveFromProject(envOption string, allowMissing bool, emptyAliasesMsg string) (result *client.Result, project, environment string, err error) {
	cfg, err := a.newConfig(envOption)
	if err != nil {
		return nil, "", "", err
	}
	project, err = cfg.RequireProjectSlug()
	if err != nil {
		return nil, "", "", wrapErr(err)
	}
	environment, err = cfg.RequireEnvironment()
	if err != nil {
		return nil, "", "", wrapErr(err)
	}
	aliases := cfg.Secrets()
	if len(aliases) == 0 {
		return nil, "", "", errors.New(emptyAliasesMsg)
	}
	token, err := cfg.RequireToken()
	if err != nil {
		return nil, "", "", wrapErr(err)
	}

	result, err = client.New(cfg.APIURL(), token).ResolveByAliases(aliases, project, environment)
	if err != nil {
		return nil, "", "", wrapErr(err)
	}
	if err := resolve.Check(a.err(), result, allowMissing); err != nil {
		return nil, "", "", err
	}
	return result, project, environment, nil
}

// EnvExec resolves secrets and replaces the process with the given command,
// injecting the secrets into its environment.
func (a *App) EnvExec(opts EnvOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("Usage: wdk env exec -- COMMAND [ARGS]")
	}
	result, _, _, err := a.resolveFromProject(opts.Env, opts.AllowMissing, "No secrets listed in wdk.yml. Add a `secrets:` list.")
	if err != nil {
		return err
	}

	path, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("command not found: %s", args[0])
	}
	env := append(os.Environ(), envPairs(result.Resolved)...)
	return a.execFn()(path, args, env)
}

// EnvExport prints the resolved secrets as shell export statements.
func (a *App) EnvExport(opts EnvOptions) error {
	result, _, _, err := a.resolveFromProject(opts.Env, opts.AllowMissing, "No secrets listed in wdk.yml.")
	if err != nil {
		return err
	}
	for _, s := range result.Resolved {
		fmt.Fprintf(a.out(), "export %s=%s\n", s.Name, shellEscape(s.Value))
	}
	return nil
}

func newEnvCmd(app *App) *cobra.Command {
	envCmd := &cobra.Command{
		Use:   "env",
		Short: "Manage secret injection",
	}
	envCmd.AddCommand(newEnvExecCmd(app), newEnvExportCmd(app))
	return envCmd
}

func newEnvExecCmd(app *App) *cobra.Command {
	var opts EnvOptions
	cmd := &cobra.Command{
		Use:   "exec -- COMMAND [ARGS]",
		Short: "Run a command with secrets injected into its environment",
		RunE: func(_ *cobra.Command, args []string) error {
			return app.EnvExec(opts, args)
		},
	}
	addEnvFlags(cmd, &opts)
	return cmd
}

func newEnvExportCmd(app *App) *cobra.Command {
	var opts EnvOptions
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Print secrets as shell export statements (for scripting / direnv)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return app.EnvExport(opts)
		},
	}
	addEnvFlags(cmd, &opts)
	return cmd
}

func addEnvFlags(cmd *cobra.Command, opts *EnvOptions) {
	cmd.Flags().StringVarP(&opts.Env, "env", "e", "", "Environment override")
	cmd.Flags().BoolVar(&opts.AllowMissing, "allow-missing", false, "Proceed even if some aliases fail to resolve")
}
