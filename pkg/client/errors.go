package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors for the HTTP status codes callers routinely branch on.
// Match them with errors.Is against any error returned by this package; APIError
// unwraps to the sentinel corresponding to its status code.
//
//	if errors.Is(err, client.ErrNotFound) { /* the object is gone */ }
var (
	// ErrNotFound is the sentinel for HTTP 404. A Terraform provider should treat this
	// as drift (the object was removed outside of its control) rather than a failure.
	ErrNotFound = errors.New("netorca: not found")
	// ErrUnauthorized is the sentinel for HTTP 401 - the API key is missing, wrong or expired.
	ErrUnauthorized = errors.New("netorca: unauthorized")
	// ErrForbidden is the sentinel for HTTP 403 - the key is valid but its team lacks
	// permission on the target, often because the POV is wrong.
	ErrForbidden = errors.New("netorca: forbidden")
	// ErrBadRequest is the sentinel for HTTP 400 - a validation failure. The server's
	// explanation is carried in APIError.Body.
	ErrBadRequest = errors.New("netorca: bad request")
	// ErrServerUnavailable is the sentinel for HTTP 502/503/504 - usually transient, retry later.
	ErrServerUnavailable = errors.New("netorca: server unavailable")
)

// ErrPackDataNotFound is returned by the pack data getters when no data exists yet for the
// requested stage (the API responds with HTTP 404). This is a normal, expected state while a
// pack pipeline is still running; detect it with errors.Is(err, ErrPackDataNotFound).
//
// It unwraps to ErrNotFound, so errors.Is(err, ErrNotFound) also matches.
var ErrPackDataNotFound = fmt.Errorf("%w: pack data not found", ErrNotFound)

// APIError is returned for any non-2xx response. It carries the request that failed and the
// server's own explanation, which for a NetOrca 400 is the validation payload callers need to
// see. Use errors.As to reach the status code, or errors.Is against the sentinels above.
type APIError struct {
	// StatusCode is the HTTP status code, e.g. 404.
	StatusCode int
	// Status is the HTTP status line, e.g. "404 Not Found".
	Status string
	// Method is the HTTP method of the failed request.
	Method string
	// URL is the full URL of the failed request.
	URL string
	// Body is the raw response body, truncated to a sane length for error messages.
	Body string
	// Detail is the "detail" field of a DRF error payload, when the body parsed as one.
	Detail string
}

// Error implements the error interface. It leads with the server's own explanation when there
// is one, because that is what a practitioner needs to read.
func (e *APIError) Error() string {
	explanation := e.Detail
	if explanation == "" {
		explanation = e.Body
	}
	if explanation == "" {
		return fmt.Sprintf("netorca: %s %s: %s", e.Method, e.URL, e.Status)
	}
	return fmt.Sprintf("netorca: %s %s: %s: %s", e.Method, e.URL, e.Status, explanation)
}

// Unwrap maps the status code onto a sentinel so errors.Is works without callers
// having to reach for the status code themselves.
func (e *APIError) Unwrap() error {
	switch e.StatusCode {
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusBadRequest:
		return ErrBadRequest
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return ErrServerUnavailable
	}
	return nil
}

// maxErrorBodyLen bounds how much of a response body ends up in an error message.
// Enough to carry a DRF validation payload, short enough not to flood a Terraform diagnostic.
const maxErrorBodyLen = 4096

// newAPIError builds an APIError from a failed response, extracting the DRF "detail"
// field when the body happens to be one.
func newAPIError(method, url string, resp *http.Response, body []byte) *APIError {
	text := string(body)
	if len(text) > maxErrorBodyLen {
		text = text[:maxErrorBodyLen] + "... (truncated)"
	}

	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Method:     method,
		URL:        url,
		Body:       text,
	}

	// DRF reports most errors as {"detail": "..."}; surface that separately when present.
	var payload struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		apiErr.Detail = payload.Detail
	}

	return apiErr
}
