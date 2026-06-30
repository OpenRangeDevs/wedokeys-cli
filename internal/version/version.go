// Package version holds the CLI version string, shared by the command layer
// (for `wdk version`) and the HTTP client (for the User-Agent header).
package version

// Version is the CLI version. It defaults to "dev" and is overridden at release
// time via -ldflags "-X github.com/OpenRangeDevs/wedokeys-cli/internal/version.Version=<tag>".
var Version = "dev"
