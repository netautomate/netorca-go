package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// GetServiceItemsRequest represents the filters for service items in the request.
type GetServiceItemsRequest struct {
	// POV is the point of view for the service item (serviceowner, consumer)
	POV string `json:"pov"`

	// name is the name of the service item
	Name string `json:"name"`

	// runtime_state is the runtime state of the service item
	RuntimeState string `json:"runtime_state"`
	// change_state is the change state of the service item
	ChangeState string `json:"change_state"`
	// declaration is the declaration of the service item
	Declaration string `json:"declaration"`

	// application_id is the ID of the application
	ApplicationID string `json:"application_id"`
	// application_name is the exact name of the application. Comma-join several to get an
	// "in" lookup (an OR), the same convention the id filters above follow.
	ApplicationName string `json:"application_name"`
	// application_name_contains matches applications whose name contains this substring,
	// case-insensitively. It is deliberately a single value rather than a list: the backend
	// declares it as a plain icontains filter, so a comma would be matched literally.
	ApplicationNameContains string `json:"application_name_contains"`
	// consumer_team_id is the ID of the consumer team
	ConsumerTeamID string `json:"consumer_team_id"`

	// declaration_contains is the declaration contains of the service item
	DeclarationContains string `json:"declaration_contains"`
	// declaration_regex is the declaration regex of the service item
	DeclarationRegex string `json:"declaration_regex"`

	// service_id is the ID of the service
	ServiceID string `json:"service_id"`
	// service_name is the name of the service
	ServiceName string `json:"service_name"`
	// service_owner_id is the ID of the service owner
	ServiceOwnerID string `json:"service_owner_id"`
	// service_owner_team_id is the ID of the service owner team
	ServiceOwnerTeamID string `json:"service_owner_team_id"`

	// start_date restricts results to items modified at or after this time. Note that the
	// backend applies both date bounds to modified rather than created, so this asks "changed
	// since", not "created since" - which is what a reconciler polling for drift wants.
	StartDate time.Time `json:"start_date"`
	// end_date restricts results to items modified at or before this time.
	EndDate time.Time `json:"end_date"`

	// limit is the number of results to return per page
	Limit int `json:"limit"`
	// offset is the initial index from which to return the results
	Offset int `json:"offset"`
	// ordering is the field to use when ordering the results
	Ordering string `json:"ordering"`
}

// ToQueryParams converts the GetServiceItemsRequest to a query string - keys are sorted alphabetically
// and values are URL encoded.
//
//nolint:funlen // one flat branch per filter; splitting it would only hide which filters exist
func (f *GetServiceItemsRequest) ToQueryParams() (string, error) {
	params := url.Values{}

	if f.Name != "" {
		params.Add("name", f.Name)
	}
	if f.RuntimeState != "" {
		params.Add("runtime_state", f.RuntimeState)
	}
	if f.ChangeState != "" {
		params.Add("change_state", f.ChangeState)
	}
	if f.Declaration != "" {
		params.Add("declaration", f.Declaration)
	}
	if f.ApplicationID != "" {
		params.Add("application_id", f.ApplicationID)
	}
	if f.ApplicationName != "" {
		params.Add("application_name", f.ApplicationName)
	}
	if f.ApplicationNameContains != "" {
		params.Add("application_name_contains", f.ApplicationNameContains)
	}
	if f.ConsumerTeamID != "" {
		params.Add("consumer_team_id", f.ConsumerTeamID)
	}
	if f.DeclarationContains != "" {
		params.Add("declaration_contains", f.DeclarationContains)
	}
	if f.DeclarationRegex != "" {
		params.Add("declaration_regex", f.DeclarationRegex)
	}
	if f.ServiceID != "" {
		params.Add("service_id", f.ServiceID)
	}
	if f.ServiceName != "" {
		params.Add("service_name", f.ServiceName)
	}
	if f.ServiceOwnerID != "" {
		params.Add("service_owner_id", f.ServiceOwnerID)
	}
	if f.ServiceOwnerTeamID != "" {
		params.Add("service_owner_team_id", f.ServiceOwnerTeamID)
	}
	if !f.StartDate.IsZero() {
		params.Add("start_date", f.StartDate.Format(time.RFC3339))
	}
	if !f.EndDate.IsZero() {
		params.Add("end_date", f.EndDate.Format(time.RFC3339))
	}
	if f.Limit > 0 {
		params.Add("limit", strconv.Itoa(f.Limit))
	}
	if f.Offset > 0 {
		params.Add("offset", strconv.Itoa(f.Offset))
	}
	if f.Ordering != "" {
		params.Add("ordering", f.Ordering)
	}

	return params.Encode(), nil
}

// GetServiceItemsResponse represents the response for service items listing
type GetServiceItemsResponse struct {
	Count    int           `json:"count"`
	Next     *string       `json:"next"`
	Previous *string       `json:"previous"`
	Results  []ServiceItem `json:"results"`
}

// ServiceItem represents a single service item in the response
type ServiceItem struct {
	ID                        int             `json:"id"`
	URL                       string          `json:"url"`
	Name                      string          `json:"name"`
	Created                   time.Time       `json:"created"`
	Modified                  time.Time       `json:"modified"`
	RuntimeState              string          `json:"runtime_state"`
	Service                   Service         `json:"service"`
	Application               Application     `json:"application"`
	Related                   *string         `json:"related"`
	ServiceOwnerTeam          Team            `json:"service_owner_team"`
	ConsumerTeam              Team            `json:"consumer_team"`
	ChangeState               string          `json:"change_state"`
	DeployedItem              json.RawMessage `json:"deployed_item"`
	Declaration               json.RawMessage `json:"declaration"`
	HealthcheckStatus         *string         `json:"healthcheck_status"`
	IsValidatedMinimumSchema  bool            `json:"is_validated_minimum_schema"`
	IsDeprecatedServiceSchema bool            `json:"is_deprecated_service_schema"`
	IsServicePrivate          bool            `json:"is_service_private"`
}

// Service represents the service information
type Service struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Owner       Owner  `json:"owner"`
	State       string `json:"state"`
	Healthcheck bool   `json:"healthcheck"`
}

// Owner represents an owner entity
type Owner struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Application represents an application entity
type Application struct {
	ID       int             `json:"id"`
	Name     string          `json:"name"`
	Metadata json.RawMessage `json:"metadata"`
	Owner    int             `json:"owner"`
}

// Team represents a team entity
type Team struct {
	ID       int             `json:"id"`
	Name     string          `json:"name"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// GetServiceItems fetches service items from the API using the provided filters.
// Requires a POV (point of view) to be set in the filters.
// The filters are used to filter the service items returned by the API.
//
// Prefer GetServiceItemsWithContext where a context is available - it lets the caller
// cancel the request, which matters for anything long-running.
func (c *Client) GetServiceItems(filters *GetServiceItemsRequest) (*GetServiceItemsResponse, error) {
	return c.GetServiceItemsWithContext(context.Background(), filters)
}

// GetServiceItemsWithContext fetches service items matching the given filters, honouring the
// caller's context for cancellation and deadlines.
func (c *Client) GetServiceItemsWithContext(
	ctx context.Context,
	filters *GetServiceItemsRequest,
) (*GetServiceItemsResponse, error) {
	return c.listServiceItems(ctx, filters, "")
}

// GetDependantServiceItems fetches the service items your team is involved in as a dependant
// team rather than as their service owner - items whose service belongs to somebody else, but
// against which your team holds change instances because that service names you in its
// dependant_teams.
//
// The plain listing cannot reach them: it is scoped to the POV's own team, so a dependant team's
// work is invisible there. This is the route to poll when your team fulfils part of somebody
// else's service.
//
// It takes the same filters and returns the same paginated shape as GetServiceItemsWithContext,
// with one caveat: the backend swaps the whole filter backend chain on this route, dropping the
// ordering backend with it, so Ordering is accepted but has no effect here.
func (c *Client) GetDependantServiceItems(
	ctx context.Context,
	filters *GetServiceItemsRequest,
) (*GetServiceItemsResponse, error) {
	return c.listServiceItems(ctx, filters, "dependant/")
}

// listServiceItems performs one service item listing, against either the plain route (an empty
// action) or one of its extra list routes. They share a filter set and a response shape, so the
// only thing that varies is the path segment.
func (c *Client) listServiceItems(
	ctx context.Context,
	filters *GetServiceItemsRequest,
	action string,
) (*GetServiceItemsResponse, error) {
	if filters == nil {
		filters = &GetServiceItemsRequest{}
	}

	params, err := filters.ToQueryParams()
	if err != nil {
		return nil, fmt.Errorf("failed to convert filters to query params: %w", err)
	}

	// The trailing slash is the canonical DRF route; without it the API replies 301 to the
	// slashed URL, doubling the round trips.
	endpoint := fmt.Sprintf("orcabase/%s/service_items/%s", POV(filters.POV).orDefault(), action)
	if params != "" {
		endpoint += "?" + params
	}

	var response GetServiceItemsResponse
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetServiceItem fetches a single service item by id.
//
// It returns an error wrapping ErrNotFound when no such item exists, which lets a caller
// reconciling state (a Terraform provider, say) tell "removed" from "request failed" without
// listing and scanning.
func (c *Client) GetServiceItem(ctx context.Context, pov POV, id int) (*ServiceItem, error) {
	endpoint := fmt.Sprintf("orcabase/%s/service_items/%d/", pov.orDefault(), id)

	var response ServiceItem
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
