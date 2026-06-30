// Package cli wires the wdk command-line interface (Cobra).
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/OpenRangeDevs/wedokeys-cli/internal/version"
)

// newRootCmd builds the `wdk` command tree against the given App.
func newRootCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:           "wdk",
		Short:         "WeDoKeys CLI — resolve secrets at runtime",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(app.out())
	root.SetErr(app.err())

	var showVersion bool
	root.Flags().BoolVarP(&showVersion, "version", "v", false, "Print the wdk version")
	root.RunE = func(cmd *cobra.Command, _ []string) error {
		if showVersion {
			fmt.Fprintf(app.out(), "wdk %s\n", version.Version)
			return nil
		}
		return cmd.Help()
	}

	root.AddCommand(
		newLoginCmd(app),
		newInitCmd(app),
		newSubshellCmd(app),
		newEnvCmd(app),
		newKamalFetchCmd(app),
		newVersionCmd(app),
	)
	return root
}

func newVersionCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the wdk version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Fprintf(app.out(), "wdk %s\n", version.Version)
			return nil
		},
	}
}

// Execute runs the CLI against the process streams and returns an exit code
// (0 on success, 1 on error), mirroring the Ruby CLI's exit_on_failure?.
func Execute() int {
	app := &App{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}
	if err := newRootCmd(app).Execute(); err != nil {
		fmt.Fprintln(app.err(), err)
		return 1
	}
	return 0
}
