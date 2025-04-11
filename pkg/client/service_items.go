package client

import (
	"encoding/json"
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

	// limit is the number of results to return per page
	Limit int `json:"limit"`
	// offset is the initial index from which to return the results
	Offset int `json:"offset"`
	// ordering is the field to use when ordering the results
	Ordering string `json:"ordering"`
}

// ToQueryParams converts the GetServiceItemsRequest to a query string - keys are sorted alphabetically
// and values are URL encoded.
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
