package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// defaultHTTPClient is shared by every Client that does not supply its own. It relies on
// http.DefaultTransport for connection pooling; per-request deadlines come from the context.
var defaultHTTPClient = &http.Client{}

// httpClient returns the client's transport, falling back to the shared default.
func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return defaultHTTPClient
}

// logf emits a request line when a Logger is configured, and is silent otherwise.
func (c *Client) logf(format string, v ...any) {
	if c.Logger != nil {
		c.Logger.Printf(format, v...)
	}
}

// doRequest performs one API call: it builds the URL from the client's base and the given
// relative path, sends body as JSON when non-nil, and decodes a successful response into out.
//
// Pass a nil body for requests without one, and a nil out to discard the response (a DELETE,
// for instance). Any non-2xx response becomes an *APIError carrying the server's explanation.
func (c *Client) doRequest(ctx context.Context, method, path string, body any, out any) error {
	fullURL := c.BaseURL + strings.TrimPrefix(path, "/")

	// Apply the client's timeout unless the caller's context already bounds the call more
	// tightly - a Terraform provider passes a context that may already be cancelled.
	if c.RequestTimeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.RequestTimeout)
			defer cancel()
		}
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Api-Key "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.logf("netorca: %s %s", method, fullURL)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Read the body before giving up on it: for a 400 it carries the validation
		// payload, which is the only part of the failure a practitioner can act on.
		raw, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			raw = nil
		}
		return newAPIError(method, fullURL, resp, raw)
	}

	// 204 carries no body, and callers who pass a nil out do not want one decoded.
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

// queryParams accumulates list-endpoint filters in the form the NetOrca API expects, and
// renders them percent-encoded. The zero value is ready to use.
//
// The platform's conventions, which the setters encode so callers need not remember them:
// omitted values are dropped entirely, lists become comma-joined "in" lookups (an OR),
// booleans are lowercase, and structured filters are compact JSON.
type queryParams struct {
	values url.Values
}

// newQueryParams returns an empty parameter set.
func newQueryParams() *queryParams {
	return &queryParams{values: url.Values{}}
}

// SetString adds a string filter, dropping it when empty.
func (q *queryParams) SetString(name, value string) {
	if value == "" {
		return
	}
	q.values.Set(name, value)
}

// SetInt adds a numeric filter, dropping it when zero. Zero is the server default for
// every numeric filter the API exposes, so it never needs sending explicitly.
func (q *queryParams) SetInt(name string, value int) {
	if value == 0 {
		return
	}
	q.values.Set(name, strconv.Itoa(value))
}

// SetBool adds a tri-state boolean filter, dropping it when nil. The pointer matters:
// "applied=false" is the executor work queue, and must be distinguishable from "unset".
func (q *queryParams) SetBool(name string, value *bool) {
	if value == nil {
		return
	}
	q.values.Set(name, strconv.FormatBool(*value))
}

// SetStrings adds a list filter as a comma-joined "in" lookup, dropping it when empty.
func (q *queryParams) SetStrings(name string, values []string) {
	if len(values) == 0 {
		return
	}
	q.values.Set(name, strings.Join(values, ","))
}

// SetInts adds a numeric list filter as a comma-joined "in" lookup, dropping it when empty.
func (q *queryParams) SetInts(name string, values []int) {
	if len(values) == 0 {
		return
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	q.values.Set(name, strings.Join(parts, ","))
}

// SetTime adds an ISO 8601 timestamp filter, dropping it when zero.
func (q *queryParams) SetTime(name string, value time.Time) {
	if value.IsZero() {
		return
	}
	q.values.Set(name, value.Format(time.RFC3339))
}

// SetJSON adds a structured filter as compact JSON - the form the declaration search
// filters (declaration, declaration_regex, declaration_contains) take.
func (q *queryParams) SetJSON(name string, value any) error {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to encode %s filter: %w", name, err)
	}
	if string(encoded) == "null" || string(encoded) == "{}" {
		return nil
	}
	q.values.Set(name, string(encoded))
	return nil
}

// Encode renders the parameters as a query string prefixed with "?", or "" when empty,
// so it can be concatenated onto a path unconditionally.
func (q *queryParams) Encode() string {
	if len(q.values) == 0 {
		return ""
	}
	return "?" + q.values.Encode()
}
