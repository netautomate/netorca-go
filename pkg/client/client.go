package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// POV is the point of view a request is made from. NetOrca scopes almost every endpoint by it:
// the same team is a service owner for the services it owns and a consumer for the declarations
// it submits, and the POV segment decides which of the two the request speaks as.
type POV string

const (
	// POVServiceOwner is the service owner point of view - fulfilling requests for services you own.
	POVServiceOwner POV = "serviceowner"
	// POVConsumer is the consumer point of view - requesting services from other teams.
	POVConsumer POV = "consumer"
)

// Validate reports whether the POV is one the API recognises. A typo would otherwise be
// interpolated straight into the path and come back as an unexplained 404.
func (p POV) Validate() error {
	switch p {
	case POVServiceOwner, POVConsumer:
		return nil
	case "":
		return fmt.Errorf("POV cannot be empty (expected %q or %q)", POVServiceOwner, POVConsumer)
	}
	return fmt.Errorf("invalid POV %q (expected %q or %q)", p, POVServiceOwner, POVConsumer)
}

// orDefault returns the POV, falling back to serviceowner when unset. Pack is a
// serviceowner-only workflow, so an empty POV there is a caller omission, not an error.
func (p POV) orDefault() POV {
	if p == "" {
		return POVServiceOwner
	}
	return p
}

// Client is a NetOrca API client. Construct it with NewClient; the exported fields may be
// adjusted afterwards (in particular HTTPClient, to supply a custom CA, proxy or test transport).
type Client struct {
	// BaseURL is the fully-qualified, version-suffixed API root, ending in a slash.
	BaseURL string
	// APIKey authenticates every request via the "Authorization: Api-Key <key>" header.
	APIKey string
	// RequestTimeout bounds each request. It is applied as a context deadline when the
	// caller's context does not already carry an earlier one.
	RequestTimeout time.Duration
	// HTTPClient performs the requests. Leave nil to use a shared default. Set it to supply
	// a custom TLS config, proxy or instrumented transport - common for on-prem instances.
	HTTPClient *http.Client
	// Logger, when set, receives one line per request. Leave nil for silence; a library
	// should not write to its consumer's log stream uninvited.
	Logger Logger
}

// Logger is the minimal logging surface the client needs. It is satisfied by *log.Logger
// and by thin adapters over structured loggers such as tflog.
type Logger interface {
	Printf(format string, v ...any)
}

// NewClient initialises a Client for the given base URL, API key and API version.
//
// The base URL may be given with or without the version suffix - both
// "https://api.netorca.io" and "https://api.netorca.io/v1" produce the same client.
// requestsTimeout bounds each request; 30 seconds is a reasonable default.
func NewClient(baseURL string, apiKey string, apiVer string, requestsTimeout time.Duration) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("base URL cannot be empty")
	}
	// HasPrefix rather than slicing: a base URL shorter than the prefix must be a plain
	// error, not a panic, in a function whose contract is returning one.
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		return nil, fmt.Errorf("base URL must start with http:// or https://")
	}
	if apiVer == "" {
		return nil, fmt.Errorf("API version cannot be empty")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key cannot be empty")
	}

	// Appending the version is idempotent, so a caller who already includes it (as the
	// Ansible collection and the platform docs both do) does not end up with ".../v1/v1/".
	normalised := strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(normalised, "/"+apiVer) {
		normalised += "/" + apiVer
	}
	normalised += "/"

	return &Client{
		BaseURL:        normalised,
		APIKey:         apiKey,
		RequestTimeout: requestsTimeout,
	}, nil
}

// RefID is an object reference that the API reads back as an object but accepts as a bare id.
// Nested relations such as a processor's service and llm_model come back as
// {"id": 49, "name": "VIRTUAL_SERVER"} and must be sent as 49; RefID absorbs both forms so
// callers only ever deal with the id.
type RefID int

// Int returns the referenced id.
func (r RefID) Int() int { return int(r) }

// UnmarshalJSON accepts either a bare id or an object carrying one, so the same struct
// can model both the read and the write shape of a relation.
func (r *RefID) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == "" {
		*r = 0
		return nil
	}

	// The write shape, and what the API returns for some endpoints: a bare integer.
	if trimmed[0] != '{' {
		var id int
		if err := json.Unmarshal(data, &id); err != nil {
			return fmt.Errorf("failed to decode reference as an id: %w", err)
		}
		*r = RefID(id)
		return nil
	}

	// The read shape: an object whose id is the part we care about.
	var object struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("failed to decode reference object: %w", err)
	}
	*r = RefID(object.ID)
	return nil
}
