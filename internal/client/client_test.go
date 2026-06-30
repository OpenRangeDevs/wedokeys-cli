package client

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestResolveByAliasesReturnsResolved(t *testing.T) {
	var gotAuth, gotCT, gotAccept, gotUA, gotPath string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"resolved":{"POSTGRES_PASSWORD":"secret123"},"errors":[],"ttl_seconds":300,"request_id":"req_abc"}`)
	}))
	defer srv.Close()

	res, err := New(srv.URL, "wdk_sat_testtoken").ResolveByAliases([]string{"POSTGRES_PASSWORD"}, "my-app", "production")
	if err != nil {
		t.Fatal(err)
	}

	if want := []Secret{{Name: "POSTGRES_PASSWORD", Value: "secret123", IsString: true}}; !reflect.DeepEqual(res.Resolved, want) {
		t.Fatalf("Resolved = %v, want %v", res.Resolved, want)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("Errors = %v, want none", res.Errors)
	}

	if gotPath != "/api/v1/resolve" {
		t.Errorf("path = %q, want /api/v1/resolve", gotPath)
	}
	if gotAuth != "Bearer wdk_sat_testtoken" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotCT != "application/json" || gotAccept != "application/json" {
		t.Errorf("Content-Type = %q, Accept = %q", gotCT, gotAccept)
	}
	if !strings.HasPrefix(gotUA, "wedokeys-wdk-cli/") {
		t.Errorf("User-Agent = %q, want wedokeys-wdk-cli/ prefix", gotUA)
	}
	wantBody := map[string]any{"aliases": []any{"POSTGRES_PASSWORD"}, "project": "my-app", "environment": "production"}
	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Errorf("request body = %v, want %v", gotBody, wantBody)
	}
}

func TestResolvePreservesResolvedOrder(t *testing.T) {
	srv := jsonServer(t, 200, `{"resolved":{"B":"2","A":"1","C":"3"},"errors":[]}`)
	defer srv.Close()

	res, err := New(srv.URL, "t").ResolveByAliases([]string{"B", "A", "C"}, "app", "production")
	if err != nil {
		t.Fatal(err)
	}
	want := []Secret{{"B", "2", true}, {"A", "1", true}, {"C", "3", true}}
	if !reflect.DeepEqual(res.Resolved, want) {
		t.Fatalf("Resolved order = %v, want %v (response order preserved)", res.Resolved, want)
	}
}

func TestResolveParsesErrors(t *testing.T) {
	srv := jsonServer(t, 200, `{"resolved":{},"errors":[{"reference":"OLD_KEY","code":"inactive_reference","message":"Reference is not active"}]}`)
	defer srv.Close()

	res, err := New(srv.URL, "t").ResolveByAliases([]string{"OLD_KEY"}, "app", "production")
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolveError{{Reference: "OLD_KEY", Code: "inactive_reference", Message: "Reference is not active"}}
	if !reflect.DeepEqual(res.Errors, want) {
		t.Fatalf("Errors = %v, want %v", res.Errors, want)
	}
}

func TestResolveMarksNonStringValue(t *testing.T) {
	srv := jsonServer(t, 200, `{"resolved":{"DB":{"user":"u","password":"p"}},"errors":[]}`)
	defer srv.Close()

	res, err := New(srv.URL, "t").ResolveByAliases([]string{"DB"}, "app", "production")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Resolved) != 1 || res.Resolved[0].IsString {
		t.Fatalf("Resolved = %+v, want one non-string secret", res.Resolved)
	}
}

func TestResolveRaisesOn401(t *testing.T) {
	srv := jsonServer(t, 401, `{"error":"unauthorized"}`)
	defer srv.Close()

	_, err := New(srv.URL, "t").ResolveByAliases([]string{"KEY"}, "app", "production")
	var ae *AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("err = %T %v, want *AuthError", err, err)
	}
}

func TestResolveRaisesOn400WithStatusAndMessage(t *testing.T) {
	srv := jsonServer(t, 400, `{"error":"missing_project","message":"project is required"}`)
	defer srv.Close()

	_, err := New(srv.URL, "t").ResolveByAliases([]string{"KEY"}, "", "production")
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("err = %T %v, want *APIError", err, err)
	}
	if ae.Status != 400 {
		t.Errorf("Status = %d, want 400", ae.Status)
	}
	if ae.Error() != "project is required" {
		t.Errorf("message = %q, want 'project is required'", ae.Error())
	}
}

func TestResolveServerErrorCarriesStatus(t *testing.T) {
	srv := jsonServer(t, 500, "")
	defer srv.Close()

	_, err := New(srv.URL, "t").ResolveByAliases([]string{"KEY"}, "app", "production")
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("err = %T %v, want *APIError", err, err)
	}
	if ae.Status != 500 {
		t.Errorf("Status = %d, want 500", ae.Status)
	}
}

func TestNetworkErrorOnConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing listening now → connection refused

	_, err := New(url, "t").ResolveByAliases([]string{"KEY"}, "app", "production")
	var ne *NetworkError
	if !errors.As(err, &ne) {
		t.Fatalf("err = %T %v, want *NetworkError", err, err)
	}
}

func TestNetworkErrorOnTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		io.WriteString(w, `{"resolved":{},"errors":[]}`)
	}))
	defer srv.Close()

	hc := &http.Client{Timeout: 50 * time.Millisecond}
	_, err := New(srv.URL, "t", WithHTTPClient(hc)).ResolveByAliases([]string{"KEY"}, "app", "production")
	var ne *NetworkError
	if !errors.As(err, &ne) {
		t.Fatalf("err = %T %v, want *NetworkError", err, err)
	}
}

func TestListProjects(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"projects":[{"slug":"demo-store","name":"Demo Store"},{"slug":"demo-analytics","name":"Demo Analytics"}],"request_id":"req_x"}`)
	}))
	defer srv.Close()

	projects, err := New(srv.URL, "wdk_sat_t").ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	want := []Project{{Slug: "demo-store", Name: "Demo Store"}, {Slug: "demo-analytics", Name: "Demo Analytics"}}
	if !reflect.DeepEqual(projects, want) {
		t.Fatalf("projects = %v, want %v", projects, want)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/v1/projects" {
		t.Errorf("request = %s %s, want GET /api/v1/projects", gotMethod, gotPath)
	}
	if gotAuth != "Bearer wdk_sat_t" {
		t.Errorf("Authorization = %q", gotAuth)
	}
}

func TestListAliases(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"aliases":[{"name":"STRIPE_KEY","environment":"development"}],"request_id":"req_x"}`)
	}))
	defer srv.Close()

	aliases, err := New(srv.URL, "t").ListAliases("demo-store")
	if err != nil {
		t.Fatal(err)
	}
	if want := []Alias{{Name: "STRIPE_KEY", Environment: "development"}}; !reflect.DeepEqual(aliases, want) {
		t.Fatalf("aliases = %v, want %v", aliases, want)
	}
	if gotPath != "/api/v1/projects/demo-store/aliases" {
		t.Errorf("path = %q, want /api/v1/projects/demo-store/aliases", gotPath)
	}
}

func TestListProjectsAuthError(t *testing.T) {
	srv := jsonServer(t, 401, `{"error":"unauthorized"}`)
	defer srv.Close()
	_, err := New(srv.URL, "t").ListProjects()
	var ae *AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("err = %T %v, want *AuthError", err, err)
	}
}

func TestListAliasesAPIError(t *testing.T) {
	srv := jsonServer(t, 400, `{"error":"project_scope_denied","message":"Service account is not scoped to this project"}`)
	defer srv.Close()
	_, err := New(srv.URL, "t").ListAliases("other")
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("err = %T %v, want *APIError", err, err)
	}
	if ae.Status != 400 || ae.Error() != "Service account is not scoped to this project" {
		t.Errorf("APIError = %d %q", ae.Status, ae.Error())
	}
}

func jsonServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
}
