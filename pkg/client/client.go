package client

import (
	"fmt"
	"time"
)

type Client struct {
	BaseURL        string
	APIKey         string
	RequestTimeout time.Duration
}

// NewClient initializes a new Client instance with the provided base URL and API key.
// It returns an error if the base URL or API key is empty.
// The base URL should be the endpoint of the API you are trying to access.
// The API key is used for authentication and should be kept secret.
// The API version is  used in this implementation - `v1`.
func NewClient(baseURL string, apiKey string, apiVer string, requestsTimeout time.Duration) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("base URL cannot be empty")
	}
	// Validate the base URL format should start with http:// or https:// and end with `/v{version}` - use regexp
	if !(baseURL[:7] == "http://" || baseURL[:8] == "https://") {
		return nil, fmt.Errorf("base URL must start with http:// or https://")
	}
	if baseURL[len(baseURL)-1] != '/' {
		baseURL += "/"
	}
	baseURL += apiVer + "/"
	if apiVer == "" {
		return nil, fmt.Errorf("API version cannot be empty")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API key cannot be empty")
	}

	return &Client{
		BaseURL:        baseURL,
		APIKey:         apiKey,
		RequestTimeout: requestsTimeout,
	}, nil
}
