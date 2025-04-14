package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ChangeInstanceState represents the state of a change instance.
type ChangeInstanceState string

const (
	ChangeInstanceERROR     ChangeInstanceState = "ERROR"
	ChangeInstancePENDING   ChangeInstanceState = "PENDING"
	ChangeInstanceAPPROVED  ChangeInstanceState = "APPROVED"
	ChangeInstanceCOMPLETED ChangeInstanceState = "COMPLETED"
	ChangeInstanceCLOSED    ChangeInstanceState = "CLOSED"
	ChangeInstanceREJECTED  ChangeInstanceState = "REJECTED"
)

// GetChangeInstancesRequest represents the filters for change instances.
// It includes fields for filtering by change instance state, type, and the consumer team,
// as well as pagination and ordering options.
// The POV (point of view) field is used to determine the API path.
type GetChangeInstancesRequest struct {
	// The POV (point of view) is used to determine the API path(serviceowner or consumer).
	POV string `json:"pov"`
	// ChangeType is the type of change instance (e.g., "CREATE", "UPDATE", "DELETE").
	ChangeType string `json:"change_type"`
	// CommitID is the ID of the commit associated with the submission.
	CommitID string `json:"commit_id"`
	// ConsumerTeamID is the ID of the consumer team associated with the change instance.
	ConsumerTeamID string `json:"consumer_team_id"`
	// Declaration is the declaration associated with the change instance.
	Declaration string `json:"declaration"`
	// DeclarationContains is a substring to search for in the declaration.
	DeclarationContains string `json:"declaration_contains"`
	// DeclarationRegex is a regex pattern to match against the declaration.
	DeclarationRegex string `json:"declaration_regex"`
	// ExcludeReferenced indicates whether to exclude referenced change instances.
	ExcludeReferenced bool `json:"exclude_referenced"`
	// Limit is the maximum number of results to return per page.
	Limit int `json:"limit"`
	// Modified is the timestamp of the last modification.
	Modified time.Time `json:"modified"`
	// Offset is the initial index from which to return the results.
	Offset int `json:"offset"`
	// Ordering is the field to use when ordering the results.
	Ordering string `json:"ordering"`
	// ServiceID is the ID of the service associated with the change instance.
	ServiceID string `json:"service_id"`
	// ServiceItemID is the ID of the service item associated with the change instance.
	ServiceItemID string `json:"service_item_id"`
	// ServiceName is the name of the service associated with the change instance.
	ServiceName string `json:"service_name"`
	// ServiceOwnerTeamID is the ID of the service owner team associated with the change instance.
	ServiceOwnerTeamID string `json:"service_owner_team_id"`
	// State is the state of the change instance (e.g., "PENDING", "APPROVED", "REJECTED").
	State string `json:"state"`
	// SubmissionID is the ID of the submission associated with the change instance.
	SubmissionID string `json:"submission_id"`
}

// ToQueryParams converts the GetChangeInstancesRequest fields into a URL-encoded query string.
func (r *GetChangeInstancesRequest) ToQueryParams() (string, error) {
	params := url.Values{}

	if r.ChangeType != "" {
		params.Add("change_type", r.ChangeType)
	}
	if r.CommitID != "" {
		params.Add("commit_id", r.CommitID)
	}
	if r.ConsumerTeamID != "" {
		params.Add("consumer_team_id", r.ConsumerTeamID)
	}
	if r.Declaration != "" {
		params.Add("declaration", r.Declaration)
	}
	if r.DeclarationContains != "" {
		params.Add("declaration_contains", r.DeclarationContains)
	}
	if r.DeclarationRegex != "" {
		params.Add("declaration_regex", r.DeclarationRegex)
	}
	if r.ExcludeReferenced {
		params.Add("exclude_referenced", strconv.FormatBool(r.ExcludeReferenced))
	}
	if r.Limit > 0 {
		params.Add("limit", strconv.Itoa(r.Limit))
	}
	if !r.Modified.IsZero() {
		params.Add("modified", r.Modified.Format(time.RFC3339))
	}
	if r.Offset > 0 {
		params.Add("offset", strconv.Itoa(r.Offset))
	}
	if r.Ordering != "" {
		params.Add("ordering", r.Ordering)
	}
	if r.ServiceID != "" {
		params.Add("service_id", r.ServiceID)
	}
	if r.ServiceItemID != "" {
		params.Add("service_item_id", r.ServiceItemID)
	}
	if r.ServiceName != "" {
		params.Add("service_name", r.ServiceName)
	}
	if r.ServiceOwnerTeamID != "" {
		params.Add("service_owner_team_id", r.ServiceOwnerTeamID)
	}
	if r.State != "" {
		params.Add("state", r.State)
	}
	if r.SubmissionID != "" {
		params.Add("submission_id", r.SubmissionID)
	}

	return params.Encode(), nil
}

// GetChangeInstancesResponse represents the paginated response returned by the API.
// It contains the result count, paging links and a slice of ChangeInstance objects.
type GetChangeInstancesResponse struct {
	Count    int              `json:"count"`
	Next     *string          `json:"next"`
	Previous *string          `json:"previous"`
	Results  []ChangeInstance `json:"results"`
}

// ChangeInstance represents a single change instance returned by the API.
// It includes identifying information, timestamps, and properties such as state and type.
type ChangeInstance struct {
	// ID is the unique identifier for the change instance.
	ID int `json:"id"`
	// URL is the API endpoint for the change instance.
	URL string `json:"url"`
	// State is the current state of the change instance (e.g., "PENDING", "APPROVED").
	State string `json:"state"`
	// Created is the timestamp when the change instance was created.
	Created time.Time `json:"created"`
	// Modified is the timestamp when the change instance was last modified.
	Modified time.Time `json:"modified"`
	// ChangeType is the type of change (e.g., "CREATE", "UPDATE", "DELETE").
	ChangeType string `json:"change_type"`
	// Log is a string containing the log or message associated with the change instance.
	Log string `json:"log"`
	// Owner is the team responsible for the Service.
	Owner Team `json:"owner"`
	// ServiceItem is the service item associated with the change instance.
	ServiceItem ServiceItem `json:"service_item"`
	// Submission is the submission associated with the change instance.
	Submission Submission `json:"submission"`
	// NewDeclaration is the new declaration associated with the change instance.
	NewDeclaration Declaration `json:"new_declaration"`
	// ServiceOwnerTeam is the team responsible for the service.
	ServiceOwnerTeam Team `json:"service_owner_team"`
	// ConsumerTeam is the team consuming the service.
	ConsumerTeam Team `json:"consumer_team"`
	// Service is the service associated with the ServiceItem.
	Service ChangeInstanceService `json:"service"`
	// Application is the application associated with the ServiceItem.
	Application Application `json:"application"`
	// IsDependant indicates whether the change instance is dependent on another.
	IsDependant bool `json:"is_dependant"`
	// OldDeclaration is the old declaration associated with the change instance.
	OldDeclaration *Declaration `json:"old_declaration"`
}

// Submission represents the submission associated with the change instance.
type Submission struct {
	// ID is the unique identifier for the submission.
	ID int `json:"id"`
	// CommitID is the ID of the commit associated with the submission of the change instance.
	CommitID string `json:"commit_id"`
}

// Declaration represents the JSON declaration associated with the change instance.
type Declaration struct {
	// Version is the unique identifier for the declaration (autoincremented).
	Version int `json:"version"`
	// Declaration is the JSON declaration associated with the change instance.
	Declaration json.RawMessage `json:"declaration"`
}
type ChangeInstanceService struct {
	ID                    int    `json:"id"`
	Name                  string `json:"name"`
	AllowManualApproval   bool   `json:"allow_manual_approval"`
	AllowManualCompletion bool   `json:"allow_manual_completion"`
}

type UpdateChangeInstanceRequest struct {

	// State is the new state of the change instance (e.g., "APPROVED", "REJECTED").
	State ChangeInstanceState `json:"state"`
	// Log is a string containing the log or message associated with the change instance.
	Log string `json:"log"`
	// DeployedItem is the deployed item associated with the change instance.
	DeployedItem json.RawMessage `json:"deployed_item"`
}

// GetChangeInstances is a method on Client that fetches change instances from the API using
// the provided filters. It builds the endpoint URL based on the POV, converts the filters into
// a query parameter string, sets up the HTTP GET request with necessary headers and a timeout,
// and decodes the JSON response into a GetChangeInstancesResponse object.
func (c *Client) GetChangeInstances(filters *GetChangeInstancesRequest) (*GetChangeInstancesResponse, error) {
	pov := filters.POV

	// Convert the filters to a URL query string.
	params, err := filters.ToQueryParams()
	if err != nil {
		return nil, fmt.Errorf("failed to convert filters to query params: %w", err)
	}

	// Construct the URL using the base URL, POV, and query parameters.
	endpoint := fmt.Sprintf("orcabase/%s/change_instances?%s", pov, params)
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

	// Check for successful HTTP status code.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get change instances: %s", resp.Status)
	}

	// Decode the JSON response into the GetChangeInstancesResponse structure.
	var response GetChangeInstancesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// ApproveChangeInstance approves a change instance by updating its state to "APPROVED".
func (c *Client) ApproveChangeInstance(id int, logStr string, deployedItem json.RawMessage) (*ChangeInstance, error) {
	return c.updateChangeInstanceState(id, ChangeInstanceAPPROVED, logStr, deployedItem)
}

// RejectChangeInstance rejects a change instance by updating its state to "REJECTED".
func (c *Client) RejectChangeInstance(id int, logStr string, deployedItem json.RawMessage) (*ChangeInstance, error) {
	return c.updateChangeInstanceState(id, ChangeInstanceREJECTED, logStr, deployedItem)
}

// CompleteChangeInstance completes a change instance by updating its state to "COMPLETED".
func (c *Client) CompleteChangeInstance(id int, logStr string, deployedItem json.RawMessage) (*ChangeInstance, error) {
	return c.updateChangeInstanceState(id, ChangeInstanceCOMPLETED, logStr, deployedItem)
}

// CloseChangeInstance closes a change instance by updating its state to "CLOSED".
func (c *Client) CloseChangeInstance(id int, logStr string, deployedItem json.RawMessage) (*ChangeInstance, error) {
	return c.updateChangeInstanceState(id, ChangeInstanceCLOSED, logStr, deployedItem)
}

// SetErrorChangeInstance sets the error state for a change instance by updating its state to "ERROR".
func (c *Client) SetErrorChangeInstance(id int, logStr string, deployedItem json.RawMessage) (*ChangeInstance, error) {
	return c.updateChangeInstanceState(id, ChangeInstanceERROR, logStr, deployedItem)
}

// PendingChangeInstance sets the pending state for a change instance by updating its state to "PENDING".
func (c *Client) PendingChangeInstance(id int, logStr string, deployedItem json.RawMessage) (*ChangeInstance, error) {
	return c.updateChangeInstanceState(id, ChangeInstancePENDING, logStr, deployedItem)
}

func (c *Client) updateChangeInstanceState(
	id int,
	state ChangeInstanceState,
	logStr string,
	deployedItem json.RawMessage,
) (*ChangeInstance, error) {
	// Construct the URL for the change instance.
	endpoint := fmt.Sprintf("orcabase/serviceowner/change_instances/%d/", id)
	fullURL := c.BaseURL + endpoint

	// Create a context with a timeout for the HTTP request.
	ctx, cancel := context.WithTimeout(context.Background(), c.RequestTimeout)
	defer cancel()

	// Create the request body with the new state and log message.
	body := UpdateChangeInstanceRequest{
		State:        state,
		Log:          logStr,
		DeployedItem: deployedItem,
	}

	// Marshal the request body into JSON.
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", fullURL, io.NopCloser(bytes.NewReader(bodyJSON)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers.
	req.Header.Set("Authorization", "Api-Key "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// Log the URL being called.
	log.Println("Calling API URL:", req.URL.String())

	// Execute the HTTP PATCH request.
	httpClient := &http.Client{Timeout: c.RequestTimeout * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check for successful HTTP status code.
	if resp.StatusCode != http.StatusOK {
		bodyJSON := new(bytes.Buffer)
		if _, err := bodyJSON.ReadFrom(resp.Body); err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return nil, fmt.Errorf("failed to update change instance state. Details: %s, %s", resp.Status, bodyJSON.String())
	}

	var response ChangeInstance
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}
