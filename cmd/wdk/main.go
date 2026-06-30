// Command wdk is the WeDoKeys CLI entrypoint.
package main

import (
	"os"

	"github.com/OpenRangeDevs/wedokeys-cli/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
