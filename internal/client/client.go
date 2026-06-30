package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/OpenRangeDevs/wedokeys-cli/internal/version"
)

const (
	userAgentPrefix = "wedokeys-wdk-cli/"
	resolvePath     = "/api/v1/resolve"

	connectTimeout = 10 * time.Second // establish the connection
	readTimeout    = 15 * time.Second // wait for the response headers
)

// Secret is one resolved alias→value pair. The slice in Result.Resolved keeps
// the order returned by the API so command output is deterministic.
type Secret struct {
	Name  string
	Value string
	// IsString reports whether the JSON value was a string (vs an object,
	// array, number, or bool). The Kamal output guard rejects non-strings.
	IsString bool
}

// ResolveError is a per-item resolution failure from the `errors` array.
type ResolveError struct {
	Reference string `json:"reference"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// Result is the parsed resolve response.
type Result struct {
	Resolved []Secret
	Errors   []ResolveError
}

// ResolvedMap returns the resolved secrets as a name→value map (order lost).
func (r *Result) ResolvedMap() map[string]string {
	m := make(map[string]string, len(r.Resolved))
	for _, s := range r.Resolved {
		m[s.Name] = s.Value
	}
	return m
}

// AuthError is returned on HTTP 401 — the token is invalid or expired.
type AuthError struct{ msg string }

func (e *AuthError) Error() string { return e.msg }

// APIError is returned for non-2xx responses other than 401. Status is the HTTP
// status code; the message is the server's `message` field when present.
type APIError struct {
	msg    string
	Status int
}

func (e *APIError) Error() string { return e.msg }

// NetworkError is returned when the endpoint cannot be reached at all
// (DNS, connection refused, timeout, TLS).
type NetworkError struct{ msg string }

func (e *NetworkError) Error() string { return e.msg }

// Client talks to the WeDoKeys resolve API.
type Client struct {
	apiURL string
	token  string
	hc     *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient overrides the underlying *http.Client (used in tests).
func WithHTTPClient(hc *http.Client) Option { return func(c *Client) { c.hc = hc } }

// New builds a Client for apiURL (trailing slash trimmed) authenticating with token.
func New(apiURL, token string, opts ...Option) *Client {
	c := &Client{
		apiURL: strings.TrimRight(apiURL, "/"),
		token:  token,
		hc: &http.Client{
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: connectTimeout}).DialContext,
				TLSHandshakeTimeout:   connectTimeout,
				ResponseHeaderTimeout: readTimeout,
			},
		},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

type resolveRequest struct {
	Aliases     []string `json:"aliases"`
	Project     string   `json:"project"`
	Environment string   `json:"environment"`
}

type resolveResponse struct {
	Resolved orderedSecrets `json:"resolved"`
	Errors   []ResolveError `json:"errors"`
}

// ResolveByAliases resolves the given aliases for a project+environment.
func (c *Client) ResolveByAliases(aliases []string, project, environment string) (*Result, error) {
	payload, err := json.Marshal(resolveRequest{Aliases: aliases, Project: project, Environment: environment})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.apiURL+resolvePath, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgentPrefix+version.Version)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, &NetworkError{msg: fmt.Sprintf("Could not reach %s: %s", c.apiURL, err)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		var parsed resolveResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, &APIError{msg: fmt.Sprintf("invalid API response: %s", err), Status: resp.StatusCode}
		}
		return &Result{Resolved: []Secret(parsed.Resolved), Errors: parsed.Errors}, nil

	case resp.StatusCode == http.StatusUnauthorized:
		return nil, &AuthError{msg: "Authentication failed. Run `wdk login` to refresh your token."}

	default:
		msg := fmt.Sprintf("API error %d", resp.StatusCode)
		var parsed struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &parsed) == nil && parsed.Message != "" {
			msg = parsed.Message
		}
		return nil, &APIError{msg: msg, Status: resp.StatusCode}
	}
}

// orderedSecrets decodes a JSON object into an order-preserving slice of Secret.
type orderedSecrets []Secret

func (o *orderedSecrets) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))

	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok == nil { // null
		return nil
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return fmt.Errorf("resolved: expected JSON object")
	}

	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := keyTok.(string)
		if !ok {
			return fmt.Errorf("resolved: expected string key")
		}

		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return err
		}
		value, isString := rawToString(raw)
		*o = append(*o, Secret{Name: key, Value: value, IsString: isString})
	}

	// consume closing '}'
	if _, err := dec.Token(); err != nil {
		return err
	}
	return nil
}

// rawToString renders a JSON value as the string the CLI uses and reports
// whether it was a JSON string. JSON strings are unquoted; non-string values
// (rare — e.g. a whole-secret reference) keep their raw JSON text so downstream
// guards can reject them.
func rawToString(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, true
	}
	return strings.TrimSpace(string(raw)), false
}
