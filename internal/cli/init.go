package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/OpenRangeDevs/wedokeys-cli/internal/client"
)

// InitOptions configures Init. Empty fields fall back to interactive pickers.
type InitOptions struct {
	Project string
	Secrets []string
	All     bool
}

// Init scaffolds a wdk.yml in the current directory by letting the user pick a
// project and its aliases from what the logged-in token can see. Flags skip the
// pickers; it refuses to overwrite an existing wdk.yml.
func (a *App) Init(opts InitOptions) error {
	dir := a.StartDir
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		dir = wd
	}
	path := filepath.Join(dir, "wdk.yml")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("wdk.yml already exists in %s", dir)
	}

	cfg, err := a.newConfig("")
	if err != nil {
		return err
	}
	token, err := cfg.RequireToken()
	if err != nil {
		return err // "No token found. Run `wdk login` first."
	}
	cl := client.New(cfg.APIURL(), token)
	reader := bufio.NewReader(a.in())
	interactive := a.interactive()

	project, err := a.chooseProject(cl, opts, reader, interactive)
	if err != nil {
		return err
	}
	secrets, err := a.chooseSecrets(cl, project, opts, reader, interactive)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(buildWdkYML(project, secrets)), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(a.out(), "Created %s\n", path)
	return nil
}

func (a *App) chooseProject(cl *client.Client, opts InitOptions, reader *bufio.Reader, interactive bool) (string, error) {
	if opts.Project != "" {
		return opts.Project, nil
	}
	projects, err := cl.ListProjects()
	if err != nil {
		return "", wrapErr(err)
	}
	switch {
	case len(projects) == 0:
		return "", errors.New("No projects are available for this token.")
	case len(projects) == 1:
		fmt.Fprintf(a.err(), "Using project: %s\n", projects[0].Slug)
		return projects[0].Slug, nil
	}
	if !interactive {
		return "", errors.New("Multiple projects available; pass --project <slug>.")
	}

	fmt.Fprintln(a.err(), "Projects:")
	for i, p := range projects {
		label := p.Slug
		if p.Name != "" && p.Name != p.Slug {
			label = fmt.Sprintf("%s (%s)", p.Slug, p.Name)
		}
		fmt.Fprintf(a.err(), "  %d) %s\n", i+1, label)
	}
	fmt.Fprint(a.err(), "Select a project [1]: ")
	choice := readLine(reader)
	idx := 1
	if choice != "" {
		n, err := strconv.Atoi(choice)
		if err != nil || n < 1 || n > len(projects) {
			return "", fmt.Errorf("invalid selection: %q", choice)
		}
		idx = n
	}
	return projects[idx-1].Slug, nil
}

func (a *App) chooseSecrets(cl *client.Client, project string, opts InitOptions, reader *bufio.Reader, interactive bool) ([]string, error) {
	if len(opts.Secrets) > 0 {
		return opts.Secrets, nil
	}
	if !opts.All && !interactive {
		return nil, nil // non-interactive, no flags → write an empty skeleton
	}

	aliases, err := cl.ListAliases(project)
	if err != nil {
		return nil, wrapErr(err)
	}
	names := make([]string, len(aliases))
	for i, al := range aliases {
		names[i] = al.Name
	}
	if opts.All {
		return names, nil
	}
	if len(names) == 0 {
		fmt.Fprintf(a.err(), "No aliases are defined for %s yet.\n", project)
		return nil, nil
	}

	fmt.Fprintf(a.err(), "Secrets in %s:\n", project)
	for i, n := range names {
		fmt.Fprintf(a.err(), "  %d) %s\n", i+1, n)
	}
	fmt.Fprint(a.err(), "Select (space-separated numbers, 'all', or empty for none): ")
	return parseSelection(readLine(reader), names)
}

// parseSelection turns "1 3", "all", or "" into the chosen alias names.
func parseSelection(input string, names []string) ([]string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}
	if strings.EqualFold(input, "all") {
		return names, nil
	}
	var selected []string
	for _, tok := range strings.Fields(input) {
		n, err := strconv.Atoi(tok)
		if err != nil || n < 1 || n > len(names) {
			return nil, fmt.Errorf("invalid selection: %q", tok)
		}
		selected = append(selected, names[n-1])
	}
	return selected, nil
}

func buildWdkYML(project string, secrets []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "project: %s\n", project)
	if len(secrets) == 0 {
		b.WriteString("secrets: []\n")
		return b.String()
	}
	b.WriteString("secrets:\n")
	for _, s := range secrets {
		fmt.Fprintf(&b, "  - %s\n", s)
	}
	return b.String()
}

func newInitCmd(app *App) *cobra.Command {
	var opts InitOptions
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a wdk.yml by picking a project and its secrets",
		Long: "Create a wdk.yml in the current directory. With no flags it lists the\n" +
			"projects and aliases your token can see and lets you pick interactively.",
		Example: "  wdk init\n" +
			"  wdk init --project my-app --all\n" +
			"  wdk init --project my-app --secret STRIPE_KEY --secret POSTGRES_PASSWORD",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return app.Init(opts)
		},
	}
	cmd.Flags().StringVar(&opts.Project, "project", "", "Project slug (skips the project picker)")
	cmd.Flags().StringArrayVar(&opts.Secrets, "secret", nil, "Secret alias to include (repeatable; skips the picker)")
	cmd.Flags().BoolVar(&opts.All, "all", false, "Include all of the project's aliases")
	return cmd
}
