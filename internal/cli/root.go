// Package cli wires the wdk command-line interface (Cobra).
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is the CLI version string, injected at build time via -ldflags
// (-X github.com/OpenRangeDevs/wedokeys-cli/internal/cli.Version=<tag>).
var Version = "dev"

// newRootCmd builds the `wdk` command tree. Subcommands are added here as
// they are implemented (M5); for now it carries only `version`.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "wdk",
		Short:         "WeDoKeys CLI — resolve secrets at runtime",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the wdk version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "wdk %s\n", Version)
			return nil
		},
	})

	return root
}

// Execute runs the root command and returns a process exit code (0 on
// success, 1 on error), mirroring the Ruby CLI's exit_on_failure? behavior.
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
