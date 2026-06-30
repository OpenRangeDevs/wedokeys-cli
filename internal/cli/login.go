package cli

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/OpenRangeDevs/wedokeys-cli/internal/client"
)

// LoginOptions configures Login. Empty fields fall back to an interactive
// prompt (when stdin is a TTY) or a default.
type LoginOptions struct {
	APIURL string // server URL; "" prompts (default: current/production)
	Token  string // service-account token; "" prompts
}

// Login authenticates with WeDoKeys and stores the server + token in
// ~/.wedokeys/config.yml. With no flags it prompts interactively. The token is
// verified with a deliberately invalid resolve probe: the server authenticating
// us and rejecting the params (HTTP 400) proves the token is valid.
func (a *App) Login(opts LoginOptions) error {
	cfg, err := a.newConfig("")
	if err != nil {
		return err
	}
	reader := bufio.NewReader(a.in())
	interactive := a.interactive()

	apiURL := opts.APIURL
	if apiURL == "" {
		apiURL = cfg.APIURL() // current config or production default
		if interactive {
			fmt.Fprintf(a.err(), "WeDoKeys server [%s]: ", apiURL)
			if entered := readLine(reader); entered != "" {
				apiURL = entered
			}
		}
	}

	token := opts.Token
	if token == "" {
		if interactive {
			tokenURL := strings.TrimRight(apiURL, "/") + "/service_accounts/new"
			fmt.Fprintf(a.err(), "Create a service account at: %s\n", tokenURL)
			fmt.Fprint(a.err(), "Open in browser? [y/N]: ")
			if isYes(readLine(reader)) {
				if err := a.openURL(tokenURL); err != nil {
					fmt.Fprintf(a.err(), "(couldn't open browser: %s)\n", err)
				}
			}
		}
		fmt.Fprint(a.err(), "Paste your wedokeys service account token: ")
		token = readLine(reader)
		if token == "" {
			return errors.New("Token cannot be blank.")
		}
	}

	cl := client.New(apiURL, token)
	if err := verifyToken(cl); err != nil {
		return err
	}

	if err := cfg.Save(token, apiURL); err != nil {
		return err
	}
	fmt.Fprintln(a.out(), "Logged in. Token saved to ~/.wedokeys/config.yml")
	return nil
}

func readLine(r *bufio.Reader) string {
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}

func isYes(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

func verifyToken(cl *client.Client) error {
	_, err := cl.ResolveByAliases([]string{}, "__verify__", "development")
	if err == nil {
		return nil // 2xx: authenticated
	}

	var authErr *client.AuthError
	var netErr *client.NetworkError
	var apiErr *client.APIError
	switch {
	case errors.As(err, &authErr):
		return errors.New("Invalid token — authentication failed.")
	case errors.As(err, &netErr):
		return fmt.Errorf("Network error: %s", netErr)
	case errors.As(err, &apiErr):
		// The probe is deliberately invalid, so a 400 means the server
		// authenticated us and rejected the params — the token is valid.
		if apiErr.Status == 400 {
			return nil
		}
		return fmt.Errorf("Could not verify token (server error %d). Please try again.", apiErr.Status)
	default:
		return err
	}
}

func newLoginCmd(app *App) *cobra.Command {
	var opts LoginOptions
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with wedokeys (prompts for server + token)",
		Long: "Authenticate this machine with WeDoKeys and store the server + token in\n" +
			"~/.wedokeys/config.yml. Run with no flags to be prompted interactively.",
		Example: "  wdk login\n" +
			"  wdk login --api-url http://localhost:3000 --token wdk_sat_...",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return app.Login(opts)
		},
	}
	cmd.Flags().StringVar(&opts.APIURL, "api-url", "", "Server URL (skips the prompt; default https://app.wedokeys.com)")
	cmd.Flags().StringVar(&opts.Token, "token", "", "Token to store (skips the prompt)")
	return cmd
}
