// Command kamal-secrets-wedokeys is the Kamal secrets adapter shim. It
// forwards to `wdk kamal-fetch`, matching the Ruby shim that prepends the
// kamal-fetch subcommand to ARGV.
package main

import (
	"os"

	"github.com/OpenRangeDevs/wedokeys-cli/internal/cli"
)

func main() {
	os.Args = append([]string{os.Args[0], "kamal-fetch"}, os.Args[1:]...)
	os.Exit(cli.Execute())
}
