package cli

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/OpenRangeDevs/wedokeys-cli/internal/client"
)

// LoginOptions configures Login.
type LoginOptions struct {
	Token string // non-empty skips the interactive prompt
}

// Login authenticates with WeDoKeys and stores the token. The token is verified
// with a deliberately invalid resolve probe: the server authenticating us and
// rejecting the params (HTTP 400) proves the token is valid.
func (a *App) Login(opts LoginOptions) error {
	token := opts.Token
	if token == "" {
		fmt.Fprint(a.err(), "Paste your wedokeys service account token: ")
		line, _ := bufio.NewReader(a.in()).ReadString('\n')
		token = strings.TrimSpace(line)
		if token == "" {
			return errors.New("Token cannot be blank.")
		}
	}

	cfg, err := a.newConfig("")
	if err != nil {
		return err
	}

	cl := client.New(cfg.APIURL(), token)
	if err := verifyToken(cl); err != nil {
		return err
	}

	if err := cfg.Save(token, ""); err != nil {
		return err
	}
	fmt.Fprintln(a.out(), "Logged in. Token saved to ~/.wedokeys/config.yml")
	return nil
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
	var token string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with wedokeys (paste your service account token)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return app.Login(LoginOptions{Token: token})
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "Token to store (skips interactive prompt)")
	return cmd
}
