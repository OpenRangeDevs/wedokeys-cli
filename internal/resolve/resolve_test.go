package resolve

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/OpenRangeDevs/wedokeys-cli/internal/client"
)

func TestCheckNoErrorsReturnsNil(t *testing.T) {
	var buf bytes.Buffer
	res := &client.Result{Resolved: []client.Secret{{Name: "A", Value: "1"}}}
	if err := Check(&buf, res, false); err != nil {
		t.Fatalf("Check err = %v, want nil", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", buf.String())
	}
}

func TestCheckAbortsAndPrintsErrors(t *testing.T) {
	var buf bytes.Buffer
	res := &client.Result{
		Resolved: []client.Secret{{Name: "OK", Value: "1"}},
		Errors: []client.ResolveError{
			{Reference: "OLD_KEY", Message: "Reference is not active", Code: "inactive_reference"},
		},
	}

	err := Check(&buf, res, false)
	var ue *UnresolvedError
	if !errors.As(err, &ue) {
		t.Fatalf("err = %T %v, want *UnresolvedError", err, err)
	}
	if ue.Failed != 1 || ue.Total != 2 {
		t.Errorf("Failed=%d Total=%d, want 1 and 2", ue.Failed, ue.Total)
	}
	if want := "OLD_KEY: Reference is not active (inactive_reference)\n"; buf.String() != want {
		t.Errorf("stderr = %q, want %q", buf.String(), want)
	}
	if !strings.HasPrefix(ue.Error(), "Error: 1 of 2 secrets could not be resolved.") {
		t.Errorf("message = %q", ue.Error())
	}
}

func TestCheckAllowMissingReturnsNilButStillPrints(t *testing.T) {
	var buf bytes.Buffer
	res := &client.Result{
		Errors: []client.ResolveError{{Reference: "X", Message: "nope", Code: "not_found"}},
	}
	if err := Check(&buf, res, true); err != nil {
		t.Fatalf("Check err = %v, want nil with allowMissing", err)
	}
	if want := "X: nope (not_found)\n"; buf.String() != want {
		t.Errorf("stderr = %q, want %q", buf.String(), want)
	}
}

func TestAssertSingleLineAcceptsSingleLineValues(t *testing.T) {
	var buf bytes.Buffer
	secrets := []client.Secret{{Name: "A", Value: "one", IsString: true}, {Name: "B", Value: "two", IsString: true}}
	if err := AssertSingleLine(&buf, secrets); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", buf.String())
	}
}

func TestAssertSingleLineRejectsMultiline(t *testing.T) {
	var buf bytes.Buffer
	secrets := []client.Secret{
		{Name: "OK", Value: "fine", IsString: true},
		{Name: "PEM", Value: "-----BEGIN-----\nline2\n-----END-----", IsString: true},
	}
	err := AssertSingleLine(&buf, secrets)
	var me *MultilineError
	if !errors.As(err, &me) {
		t.Fatalf("err = %T %v, want *MultilineError", err, err)
	}
	if me.Count != 1 {
		t.Errorf("Count = %d, want 1", me.Count)
	}
	if !strings.HasPrefix(buf.String(), "PEM: value is not a single line") {
		t.Errorf("stderr = %q", buf.String())
	}
}

func TestAssertSingleLineRejectsNonString(t *testing.T) {
	var buf bytes.Buffer
	secrets := []client.Secret{{Name: "DB", Value: `{"user":"u"}`, IsString: false}}
	err := AssertSingleLine(&buf, secrets)
	var me *MultilineError
	if !errors.As(err, &me) {
		t.Fatalf("err = %T %v, want *MultilineError for a non-string (whole-secret) value", err, err)
	}
	if !strings.HasPrefix(buf.String(), "DB: value is not a single line") {
		t.Errorf("stderr = %q", buf.String())
	}
}
