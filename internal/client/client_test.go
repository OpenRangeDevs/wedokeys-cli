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

	if want := []Secret{{Name: "POSTGRES_PASSWORD", Value: "secret123"}}; !reflect.DeepEqual(res.Resolved, want) {
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
	want := []Secret{{"B", "2"}, {"A", "1"}, {"C", "3"}}
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

func jsonServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
}
