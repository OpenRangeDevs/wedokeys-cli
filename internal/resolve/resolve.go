package resolve

import (
	"fmt"
	"io"
	"strings"

	"github.com/OpenRangeDevs/wedokeys-cli/internal/client"
)

// UnresolvedError is returned by Check when one or more aliases failed to
// resolve and --allow-missing was not set.
type UnresolvedError struct {
	Failed int
	Total  int
}

func (e *UnresolvedError) Error() string {
	return fmt.Sprintf(
		"Error: %d of %d secrets could not be resolved. Fix the aliases above, or pass --allow-missing to proceed without them.",
		e.Failed, e.Total,
	)
}

// Check writes one stderr line per resolution error and, unless allowMissing,
// returns an *UnresolvedError. Resolution is all-or-nothing by default:
// proceeding with partial results would boot an app with missing env vars.
// Mirrors Ruby ResolveGuard.check!.
func Check(stderr io.Writer, result *client.Result, allowMissing bool) error {
	if len(result.Errors) == 0 {
		return nil
	}
	for _, e := range result.Errors {
		fmt.Fprintf(stderr, "%s: %s (%s)\n", e.Reference, e.Message, e.Code)
	}
	if allowMissing {
		return nil
	}
	return &UnresolvedError{Failed: len(result.Errors), Total: len(result.Resolved) + len(result.Errors)}
}

// MultilineError is returned by AssertSingleLine when a value cannot be emitted
// as a single NAME=value line.
type MultilineError struct {
	Count int
}

func (e *MultilineError) Error() string {
	return fmt.Sprintf("Error: %d secret(s) cannot be emitted as NAME=value lines for Kamal.", e.Count)
}

// AssertSingleLine writes a stderr line per offending secret and returns a
// *MultilineError if any value spans multiple lines (a PEM key, a multi-line
// string, or a whole-secret reference). Kamal parses one NAME=value per line,
// so escaping a newline would corrupt the secret — fail hard instead. Mirrors
// Ruby KamalFetch#assert_single_line_values!.
func AssertSingleLine(stderr io.Writer, secrets []client.Secret) error {
	offenders := 0
	for _, s := range secrets {
		if !s.IsString || strings.ContainsAny(s.Value, "\r\n") {
			offenders++
			fmt.Fprintf(stderr, "%s: value is not a single line (multiline secret or whole-secret \"*\" reference); reference a specific field instead.\n", s.Name)
		}
	}
	if offenders == 0 {
		return nil
	}
	return &MultilineError{Count: offenders}
}
