package client

import (
	"context"
	"encoding/json"
	"fmt"
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
	// ApplicationID is the ID of the application owning the change instance's service item.
	// Comma-join several to get an "in" lookup (an OR), as with the other id filters here.
	//
	// This is the filter a per-application view needs: a change instance carries no direct
	// link to an application, so without it a caller has to list every change instance and
	// sift them client-side.
	ApplicationID string `json:"application_id"`
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
	// EndDate restricts results to change instances modified at or before this time.
	EndDate time.Time `json:"end_date"`
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
	// StartDate restricts results to change instances modified at or after this time. It is
	// the same lower bound Modified already applies - the backend declares both against
	// modified with a gte lookup - and exists so a window can be expressed as one pair of
	// fields alongside EndDate. Set one or the other, not both.
	StartDate time.Time `json:"start_date"`
	// State is the state of the change instance (e.g., "PENDING", "APPROVED", "REJECTED").
	State string `json:"state"`
	// SubmissionID is the ID of the submission associated with the change instance.
	SubmissionID string `json:"submission_id"`
}

// ToQueryParams converts the GetChangeInstancesRequest fields into a URL-encoded query string.
//
//nolint:funlen // one flat branch per filter; splitting it would only hide which filters exist
func (r *GetChangeInstancesRequest) ToQueryParams() (string, error) {
	params := url.Values{}

	if r.ApplicationID != "" {
		params.Add("application_id", r.ApplicationID)
	}
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
	if !r.EndDate.IsZero() {
		params.Add("end_date", r.EndDate.Format(time.RFC3339))
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
	if !r.StartDate.IsZero() {
		params.Add("start_date", r.StartDate.Format(time.RFC3339))
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
	// Log is the reason for the transition, recorded against the change. Omitted when empty so
	// a transition that says nothing leaves any existing log intact rather than blanking it.
	Log string `json:"log,omitempty"`
	// DeployedItem records what was built to serve the request.
	//
	// The omitempty is load-bearing, not cosmetic. json.RawMessage is a []byte, and a nil one
	// marshals to the literal null rather than disappearing - so without this, passing nil
	// sends {"deployed_item": null} and the API rejects the whole transition with
	// {"deployed_item":["This field may not be null."]}. That made the most natural call in
	// the package - approving a change without asserting anything about deployment - fail
	// every time.
	DeployedItem json.RawMessage `json:"deployed_item,omitempty"`
}

// GetChangeInstances is a method on Client that fetches change instances from the API using
// the provided filters. It builds the endpoint URL based on the POV, converts the filters into
// a query parameter string, sets up the HTTP GET request with necessary headers and a timeout,
// and decodes the JSON response into a GetChangeInstancesResponse object.
// Prefer GetChangeInstancesWithContext where a context is available - it lets the caller
// cancel the request, which matters for anything long-running.
func (c *Client) GetChangeInstances(filters *GetChangeInstancesRequest) (*GetChangeInstancesResponse, error) {
	return c.GetChangeInstancesWithContext(context.Background(), filters)
}

// GetChangeInstancesWithContext fetches change instances matching the given filters, honouring
// the caller's context for cancellation and deadlines.
func (c *Client) GetChangeInstancesWithContext(
	ctx context.Context,
	filters *GetChangeInstancesRequest,
) (*GetChangeInstancesResponse, error) {
	return c.listChangeInstances(ctx, filters, "")
}

// GetDependantChangeInstances fetches the change instances raised against services your team
// owns but handed to a different team to fulfil - the copies your dependant teams are working
// on, which the plain listing hides because it only shows changes your own team owns.
//
// This is the main team's view of its dependants' progress, and so is the mirror image of
// GetDependantServiceItems, which shows the items where your team is itself the dependant.
//
// It takes the same filters and returns the same paginated shape as
// GetChangeInstancesWithContext, except that the backend swaps out the whole filter backend
// chain on this route, dropping the ordering backend with it - so Ordering is accepted but has
// no effect here.
func (c *Client) GetDependantChangeInstances(
	ctx context.Context,
	filters *GetChangeInstancesRequest,
) (*GetChangeInstancesResponse, error) {
	return c.listChangeInstances(ctx, filters, "dependant/")
}

// GetReferencedChangeInstances fetches the referenced change instances your team can see: those
// raised on somebody else's service item because it names one of your items in its related list.
//
// When a consumer's load balancer declares your virtual machine as related, a change to that
// machine raises a MODIFY change on the load balancer. That change belongs to the load
// balancer's service owner, not to you, so it never appears in your plain listing - but you are
// the reason it exists, and this route is how you see it.
//
// It is the complement of the ExcludeReferenced filter, which removes exactly this class of
// change from the plain listing. The same ordering caveat as GetDependantChangeInstances
// applies.
func (c *Client) GetReferencedChangeInstances(
	ctx context.Context,
	filters *GetChangeInstancesRequest,
) (*GetChangeInstancesResponse, error) {
	return c.listChangeInstances(ctx, filters, "referenced/")
}

// listChangeInstances performs one change instance listing, against either the plain route (an
// empty action) or one of its extra list routes. They share a filter set and a response shape,
// so the only thing that varies is the path segment.
func (c *Client) listChangeInstances(
	ctx context.Context,
	filters *GetChangeInstancesRequest,
	action string,
) (*GetChangeInstancesResponse, error) {
	if filters == nil {
		filters = &GetChangeInstancesRequest{}
	}

	// Convert the filters to a URL query string.
	params, err := filters.ToQueryParams()
	if err != nil {
		return nil, fmt.Errorf("failed to convert filters to query params: %w", err)
	}

	// The trailing slash is the canonical DRF route; without it the API replies 301 to the
	// slashed URL, doubling the round trips.
	endpoint := fmt.Sprintf("orcabase/%s/change_instances/%s", POV(filters.POV).orDefault(), action)
	if params != "" {
		endpoint += "?" + params
	}

	var response GetChangeInstancesResponse
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetChangeInstance fetches a single change instance by id.
//
// It returns an error wrapping ErrNotFound when no such change exists, which lets a caller
// reconciling state (a Terraform provider, say) tell "removed" from "request failed" without
// listing and scanning.
func (c *Client) GetChangeInstance(ctx context.Context, pov POV, id int) (*ChangeInstance, error) {
	endpoint := fmt.Sprintf("orcabase/%s/change_instances/%d/", pov.orDefault(), id)

	var response ChangeInstance
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// ChangeInstanceHistoryEntry is one recorded step in a change instance's life.
//
// The platform tracks only state and log changes, so the trail is a record of decisions - who
// approved or rejected the change and why - rather than of every field it touched along the way.
type ChangeInstanceHistoryEntry struct {
	// ID is the change instance the entry belongs to, not the entry's own identifier: the
	// platform serialises the tracked object's id, so every entry in one trail repeats it.
	// Entries are told apart by Modified and Reason.
	ID int `json:"id"`
	// State is the state the change held at this point, rendered as its name ("PENDING",
	// "APPROVED") rather than the integer the database stores.
	State string `json:"state"`
	// Log is the message recorded with the transition - the explanation a consumer reads when
	// their request is rejected. Empty when the transition carried none.
	Log string `json:"log"`
	// Modified is when the entry was recorded. Entries arrive newest first.
	Modified time.Time `json:"modified"`
	// Reason is which of the two tracked fields moved: "state" or "log".
	Reason string `json:"reason"`
	// ChangedBy is the username behind the change, or an API key's name suffixed " (Api Key)"
	// when a key made it. It reads "SYSTEM" for a transition the platform made itself.
	ChangedBy *string `json:"changed_by"`
	// ChangedByTeam is the team the actor was acting for, nil when the platform cannot
	// attribute the change to one - a user who has since left the team, for instance.
	ChangedByTeam *string `json:"changed_by_team"`
}

// ListChangeInstanceHistory returns a change instance's state and log history, newest first.
//
// This is the audit trail behind a change: who approved or rejected it, when, and with what
// explanation. It is the only way to recover a superseded log message, because a later
// transition overwrites the change's own Log field.
//
// It returns an error wrapping ErrNotFound when no such change exists.
func (c *Client) ListChangeInstanceHistory(
	ctx context.Context,
	pov POV,
	id int,
) ([]ChangeInstanceHistoryEntry, error) {
	endpoint := fmt.Sprintf("orcabase/%s/change_instances/%d/history/", pov.orDefault(), id)

	var raw json.RawMessage
	if err := c.doRequest(ctx, "GET", endpoint, nil, &raw); err != nil {
		return nil, err
	}
	return decodeHistoryList[ChangeInstanceHistoryEntry](raw, "change instance")
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

// UpdateChangeInstanceState transitions a change instance to the given state, honouring the
// caller's context.
//
// The log string is recorded against the change as the reason for the transition, and is what a
// consumer reads when their request is rejected - write it for them, not for a machine. Pass a
// nil deployedItem to leave the linked deployed item untouched.
//
// The platform enforces which transitions are legal (a COMPLETED change must have been APPROVED,
// for instance); an illegal one comes back as an error wrapping ErrBadRequest.
func (c *Client) UpdateChangeInstanceState(
	ctx context.Context,
	pov POV,
	id int,
	state ChangeInstanceState,
	logStr string,
	deployedItem json.RawMessage,
) (*ChangeInstance, error) {
	endpoint := fmt.Sprintf("orcabase/%s/change_instances/%d/", pov.orDefault(), id)

	body := UpdateChangeInstanceRequest{
		State:        state,
		Log:          logStr,
		DeployedItem: deployedItem,
	}

	var response ChangeInstance
	if err := c.doRequest(ctx, "PATCH", endpoint, body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// updateChangeInstanceState is the serviceowner-POV shorthand the state-specific helpers use.
func (c *Client) updateChangeInstanceState(
	id int,
	state ChangeInstanceState,
	logStr string,
	deployedItem json.RawMessage,
) (*ChangeInstance, error) {
	return c.UpdateChangeInstanceState(context.Background(), POVServiceOwner, id, state, logStr, deployedItem)
}
