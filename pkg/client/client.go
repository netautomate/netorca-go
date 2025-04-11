package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

// GetServiceItems fetches service items from the API using the provided filters.
// Requires a POV (point of view) to be set in the filters.
// The filters are used to filter the service items returned by the API.
func (c *Client) GetServiceItems(filters *GetServiceItemsRequest) (*GetServiceItemsResponse, error) {
	pov := filters.POV

	params, err := filters.ToQueryParams()
	if err != nil {
		return nil, fmt.Errorf("failed to convert filters to query params: %w", err)
	}

	url := c.BaseURL + fmt.Sprintf("orcabase/%s/service_items?%s", pov, params)

	// set timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Api-Key "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	log.Println("Calling API url:", req.URL.String())

	client := &http.Client{Timeout: c.RequestTimeout * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get service items: %s", resp.Status)
	}

	var response GetServiceItemsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}
