// Package client is the HTTP client for the WeDoKeys resolve API
// (POST /api/v1/resolve). It maps responses to a Result and the failure modes
// to typed errors: AuthError (401), APIError (other non-2xx, carrying the HTTP
// status), and NetworkError (the endpoint could not be reached).
package client
