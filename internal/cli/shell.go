package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// ShellOptions configures the subshell command.
type ShellOptions struct {
	Env          string
	AllowMissing bool
}

// Subshell resolves secrets and replaces the process with an interactive shell
// that has the secrets (and a labeled PS1) loaded into its environment.
func (a *App) Subshell(opts ShellOptions) error {
	result, project, environment, err := a.resolveFromProject(
		opts.Env, opts.AllowMissing,
		"No secrets listed in wdk.yml. Add a `secrets:` list to load secrets into the shell.",
	)
	if err != nil {
		return err
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	path, err := exec.LookPath(shell)
	if err != nil {
		path = shell
	}

	env := append(os.Environ(), envPairs(result.Resolved)...)
	env = append(env, fmt.Sprintf("PS1=[wdk:%s/%s] ", project, environment))
	return a.execFn()(path, []string{shell}, env)
}

func newSubshellCmd(app *App) *cobra.Command {
	var opts ShellOptions
	cmd := &cobra.Command{
		Use:     "subshell",
		Short:   "Open a sub-shell with secrets loaded into the environment",
		Example: "  wdk subshell -e production",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return app.Subshell(opts)
		},
	}
	cmd.Flags().StringVarP(&opts.Env, "env", "e", "", "Environment (overrides WDK_ENV / KAMAL_DESTINATION)")
	cmd.Flags().BoolVar(&opts.AllowMissing, "allow-missing", false, "Proceed even if some aliases fail to resolve")
	return cmd
}
