package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// PackActionType identifies a stage in the NetOrca AI pack pipeline.
// The pipeline runs config -> verify -> execution; RetriggerPack always restarts it from config.
type PackActionType string

const (
	// PackActionConfig is the "config" stage - the AI-generated desired configuration.
	PackActionConfig PackActionType = "config"
	// PackActionVerify is the "verify" stage - verification of the generated configuration.
	PackActionVerify PackActionType = "verify"
	// PackActionExecution is the "execution" stage - the execution/deployment data.
	PackActionExecution PackActionType = "execution"
)

// ErrPackDataNotFound is returned by the pack data getters when no data exists yet for the
// requested stage (the API responds with HTTP 404). This is a normal, expected state while a
// pack pipeline is still running; detect it with errors.Is(err, ErrPackDataNotFound).
var ErrPackDataNotFound = errors.New("netorca: pack data not found")

// PackData is the persisted output of a single pack pipeline stage.
// The Data field holds the AI-generated JSON payload that callers act on; unmarshal it into your
// own shape. Server fields that are not modelled here are ignored during decoding.
type PackData struct {
	// ID is the unique identifier for the pack data record.
	ID int `json:"id"`
	// Created is the timestamp when the pack data was created.
	Created time.Time `json:"created"`
	// Modified is the timestamp when the pack data was last modified.
	Modified time.Time `json:"modified"`
	// ActionType is the pipeline stage this data belongs to (config, verify or execution).
	ActionType string `json:"action_type"`
	// Data is the AI-generated JSON payload for this stage - the field callers read.
	Data json.RawMessage `json:"data"`
	// ObjectID is the id of the scoped object (the service item) this data applies to.
	ObjectID int `json:"object_id"`
	// Scope is the scoped-object envelope the API returns ({"scope":"service_item","data":{...}}); freeform JSON.
	Scope json.RawMessage `json:"scope"`
	// SIDeclaration is the service item declaration snapshot associated with this data.
	SIDeclaration json.RawMessage `json:"si_declaration"`
}

// retriggerPackRequest is the request body for RetriggerPack.
// An empty comment is omitted so the API receives an empty JSON object.
type retriggerPackRequest struct {
	// ServiceownerComment is optional feedback passed to the AI processor on retrigger.
	ServiceownerComment string `json:"serviceowner_comment,omitempty"`
}

// GetPackConfig returns the latest "config" stage data for the given service item.
// It returns ErrPackDataNotFound if the config stage has not produced data yet.
func (c *Client) GetPackConfig(serviceItemID int) (*PackData, error) {
	return c.getPackData(serviceItemID, PackActionConfig)
}

// GetPackVerify returns the latest "verify" stage data for the given service item.
// It returns ErrPackDataNotFound if the verify stage has not produced data yet.
func (c *Client) GetPackVerify(serviceItemID int) (*PackData, error) {
	return c.getPackData(serviceItemID, PackActionVerify)
}

// GetPackExecution returns the latest "execution" stage data for the given service item.
// It returns ErrPackDataNotFound if the execution stage has not produced data yet.
func (c *Client) GetPackExecution(serviceItemID int) (*PackData, error) {
	return c.getPackData(serviceItemID, PackActionExecution)
}

// getPackData fetches the latest pack data for a service item at the given pipeline stage.
// The pack loop is a serviceowner-only workflow, so the POV is fixed to "serviceowner".
func (c *Client) getPackData(serviceItemID int, action PackActionType) (*PackData, error) {
	// Construct the URL for the pack stage data (serviceowner POV, service_item scope).
	// The trailing slash is the canonical DRF route; without it the API replies 301 to the slashed URL.
	endpoint := fmt.Sprintf("external/serviceowner/pack/data/service_item/%d/%s/", serviceItemID, action)
	fullURL := c.BaseURL + endpoint

	// Create a context with a timeout for the HTTP request.
	ctx, cancel := context.WithTimeout(context.Background(), c.RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers.
	req.Header.Set("Authorization", "Api-Key "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	// Log the URL being called.
	log.Println("Calling API URL:", req.URL.String())

	// Execute the HTTP GET request.
	httpClient := &http.Client{Timeout: c.RequestTimeout * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// A 404 means the stage has not produced data yet - a normal state in the pack loop.
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrPackDataNotFound
	}

	// Check for successful HTTP status code.
	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		if _, err := body.ReadFrom(resp.Body); err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return nil, fmt.Errorf("failed to get pack %s data: %s, %s", action, resp.Status, body.String())
	}

	var response PackData
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// RetriggerPack re-runs the AI pack pipeline for the given service item, always restarting from the
// config stage. The optional serviceownerComment is passed to the AI processor as feedback (for
// example, why a previous verify result was rejected); pass "" to send none. On success it returns
// the confirmation message from the API (e.g. "AI Processor has been retriggered").
func (c *Client) RetriggerPack(serviceItemID int, serviceownerComment string) (string, error) {
	// Construct the URL for the pack retrigger (serviceowner POV, service_item scope).
	endpoint := fmt.Sprintf("external/serviceowner/pack/retrigger/service_item/%d/", serviceItemID)
	fullURL := c.BaseURL + endpoint

	// Create a context with a timeout for the HTTP request.
	ctx, cancel := context.WithTimeout(context.Background(), c.RequestTimeout)
	defer cancel()

	// Create the request body with the optional serviceowner comment.
	body := retriggerPackRequest{ServiceownerComment: serviceownerComment}

	// Marshal the request body into JSON.
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, io.NopCloser(bytes.NewReader(bodyJSON)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers.
	req.Header.Set("Authorization", "Api-Key "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// Log the URL being called.
	log.Println("Calling API URL:", req.URL.String())

	// Execute the HTTP POST request.
	httpClient := &http.Client{Timeout: c.RequestTimeout * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check for successful HTTP status code.
	if resp.StatusCode != http.StatusOK {
		respBody := new(bytes.Buffer)
		if _, err := respBody.ReadFrom(resp.Body); err != nil {
			return "", fmt.Errorf("failed to read response body: %w", err)
		}
		return "", fmt.Errorf("failed to retrigger pack. Details: %s, %s", resp.Status, respBody.String())
	}

	// The API returns a bare JSON string message, e.g. "AI Processor has been retriggered".
	var message string
	if err := json.NewDecoder(resp.Body).Decode(&message); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return message, nil
}
