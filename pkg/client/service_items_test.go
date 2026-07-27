//nolint:dupl
package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/netautomate/netorca-go/config"
	"github.com/netautomate/netorca-go/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const serviceItemsRoot = packTestBaseURL + "/v1/orcabase/serviceowner/service_items"

// emptyPage is the paginated envelope for a listing that matched nothing. The wire tests care
// about the request they sent, not the answer, so they all reply with this.
const emptyPage = `{"count":0,"next":null,"previous":null,"results":[]}`

//nolint:funlen // a filter table: every case has to spell out the fields it is pinning
func TestGetServiceItemsToQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		request  *client.GetServiceItemsRequest
		expected string
	}{
		{
			name: "All fields set",
			request: &client.GetServiceItemsRequest{
				Name:                    "test",
				RuntimeState:            "running",
				ChangeState:             "changed",
				Declaration:             "declaration",
				ApplicationID:           "app-id",
				ApplicationName:         "app-name",
				ApplicationNameContains: "app-nam",
				ConsumerTeamID:          "team-id",
				DeclarationContains:     "contains",
				DeclarationRegex:        "regex",
				ServiceID:               "service-id",
				ServiceName:             "service-name",
				ServiceOwnerID:          "owner-id",
				ServiceOwnerTeamID:      "team-owner-id",
				StartDate:               time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
				EndDate:                 time.Date(2025, 4, 30, 0, 0, 0, 0, time.UTC),
				Limit:                   10,
				Offset:                  0,
				Ordering:                "-created_at",
			},
			expected: "application_id=app-id&application_name=app-name&application_name_contains=app-nam&change_state=changed&consumer_team_id=team-id&declaration=declaration&declaration_contains=contains&declaration_regex=regex&end_date=2025-04-30T00%3A00%3A00Z&limit=10&name=test&ordering=-created_at&runtime_state=running&service_id=service-id&service_name=service-name&service_owner_id=owner-id&service_owner_team_id=team-owner-id&start_date=2025-04-01T00%3A00%3A00Z", //nolint
		},
		{
			name:     "No fields set",
			request:  &client.GetServiceItemsRequest{},
			expected: "",
		},
		{
			name: "Some fields set",
			request: &client.GetServiceItemsRequest{
				Name:         "test",
				RuntimeState: "running",
				Limit:        5,
				Offset:       10,
				Ordering:     "name",
			},
			expected: "limit=5&name=test&offset=10&ordering=name&runtime_state=running",
		},
		{
			name: "Only limit and offset set",
			request: &client.GetServiceItemsRequest{
				Limit:  20,
				Offset: 5,
			},
			expected: "limit=20&offset=5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := tt.request.ToQueryParams()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, query)
		})
	}
}
func exampleServiceItem() *client.ServiceItem {
	// Example response from the API: testdata/200_single_service_item_response.json
	// created by hand to match the API response and validate marshaling and unmarshaling
	return &client.ServiceItem{
		ID:   35,
		Name: "fastapi-app17",
		URL:  "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/service_items/35/",
		Service: client.Service{
			ID:   4,
			Name: "THREE_TIER_APPLICATION",
			Owner: client.Owner{
				ID:   4,
				Name: "AWS",
			},
			State:       "IN_SERVICE",
			Healthcheck: false,
		},
		Application: client.Application{
			ID:   23,
			Name: "app17",
			Metadata: json.RawMessage(`
			{
				"owner": "team5@example.com",
				"description": "My fastApi application17",
				"environment": "DEV"
			}
			`),
			Owner: 2,
		},
		ServiceOwnerTeam: client.Team{
			ID:   4,
			Name: "AWS",
		},
		ConsumerTeam: client.Team{
			ID:       2,
			Name:     "beta",
			Metadata: json.RawMessage(`{"team_name":"beta"}`),
		},

		ChangeState:  "CHANGES_APPROVED",
		DeployedItem: json.RawMessage(`{}`),

		Declaration: json.RawMessage(`
		{
			"name": "fastapi-app17",
			"size": "small",
			"image": "ami-02141377eee7defb91",
			"owner": "beta11111@test.com",
			"description": "fastapi app for beta",
			"environment": "dev"
		}`),
		Related:      nil,
		Created:      time.Date(2025, 4, 9, 11, 11, 4, 194909000, time.UTC),
		Modified:     time.Date(2025, 4, 9, 11, 18, 46, 902227000, time.UTC),
		RuntimeState: "IN_SERVICE",

		HealthcheckStatus:         nil,
		IsValidatedMinimumSchema:  false,
		IsDeprecatedServiceSchema: false,
		IsServicePrivate:          false,
	}
}

func TestClientServiceItems(t *testing.T) { //nolint:funlen
	// Test responses with mocked HTTP requests
	// Responses are mocked using httpmock
	t.Run("Test NewClient returns empty response with 200 when no filters matched", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/service_items/",
			httpmock.NewStringResponder(200, `{
			"count": 0,
			"next": null,
			"previous": null,
			"results": []
		}`),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}

		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)
		filters := &client.GetServiceItemsRequest{
			POV:           "serviceowner",
			Limit:         10,
			Offset:        0,
			ApplicationID: "23",
		}
		serviceItems, err := nc.GetServiceItems(filters)
		require.NoError(t, err)
		assert.NotNil(t, serviceItems)

		assert.Equal(t, 0, serviceItems.Count)
		assert.Equal(t, []client.ServiceItem{}, serviceItems.Results)
		assert.Nil(t, serviceItems.Next)
		assert.Nil(t, serviceItems.Previous)
	})

	t.Run("Test GetServiceItems returns services list on success", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		// Register a mock response for the GET request with real data
		testFileContent := readTestFile(t, "200_single_service_item_response.json")
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/service_items/",
			httpmock.NewStringResponder(200, testFileContent),
		)

		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}

		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)
		filters := &client.GetServiceItemsRequest{
			POV:           "serviceowner",
			Limit:         10,
			Offset:        0,
			ApplicationID: "23",
		}
		serviceItems, err := nc.GetServiceItems(filters)
		require.NoError(t, err)
		assert.NotNil(t, serviceItems)
		expectedSvc := exampleServiceItem()

		assert.Equal(t, 1, serviceItems.Count)
		assert.Equal(t, expectedSvc.Name, serviceItems.Results[0].Name)
		// turn two interfaces into json and compare them to avoid issues with RawMessage jsons
		actualSvcItem, err := json.Marshal(serviceItems.Results[0])
		require.NoError(t, err)
		expectedSvcItem, err := json.Marshal(*expectedSvc)
		require.NoError(t, err)
		assert.JSONEq(t, string(expectedSvcItem), string(actualSvcItem))
	})

	t.Run("Test GetServiceItems returns error on 500", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/service_items/",
			httpmock.NewStringResponder(500, `{"error": "Internal Server Error"}`),
		)

		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}

		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)
		filters := &client.GetServiceItemsRequest{
			POV:           "serviceowner",
			Limit:         10,
			Offset:        0,
			ApplicationID: "23",
		}
		serviceItems, err := nc.GetServiceItems(filters)
		require.Error(t, err)
		assert.Nil(t, serviceItems)
		require.ErrorContains(t, err, "500 Internal Server Error")
	})
	t.Run("Test GetServiceItems returns error on 400", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/service_items/",
			httpmock.NewStringResponder(400, `{"error": "Bad Request"}`),
		)

		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}

		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)
		filters := &client.GetServiceItemsRequest{
			POV:           "serviceowner",
			Limit:         10,
			Offset:        0,
			ApplicationID: "23",
		}
		serviceItems, err := nc.GetServiceItems(filters)
		require.Error(t, err)
		assert.Nil(t, serviceItems)
		require.ErrorIs(t, err, client.ErrBadRequest)
	})
}

// TestGetServiceItemsFilterWire pins the new filters to the wire. The backend's filterset rejects
// any parameter it does not declare with a 400, so the names matter as much as the values.
func TestGetServiceItemsFilterWire(t *testing.T) {
	t.Run("sends the application name and modified-window filters", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET", serviceItemsRoot+"/",
			func(req *http.Request) (*http.Response, error) {
				capturedQuery = req.URL.RawQuery
				return httpmock.NewStringResponse(200, emptyPage), nil
			})

		nc := newPackTestClient(t)
		_, err := nc.GetServiceItemsWithContext(context.Background(), &client.GetServiceItemsRequest{
			ApplicationName:         "app17,app18",
			ApplicationNameContains: "app1",
			StartDate:               time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
			EndDate:                 time.Date(2025, 4, 30, 0, 0, 0, 0, time.UTC),
		})

		require.NoError(t, err)
		// A comma-joined value is the platform's "in" lookup, so this asks for either.
		assert.Contains(t, capturedQuery, "application_name=app17%2Capp18")
		assert.Contains(t, capturedQuery, "application_name_contains=app1")
		assert.Contains(t, capturedQuery, "start_date=2025-04-01T00%3A00%3A00Z")
		assert.Contains(t, capturedQuery, "end_date=2025-04-30T00%3A00%3A00Z")
	})

	t.Run("omits the date filters when unset", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET", serviceItemsRoot+"/",
			func(req *http.Request) (*http.Response, error) {
				capturedQuery = req.URL.RawQuery
				return httpmock.NewStringResponse(200, emptyPage), nil
			})

		nc := newPackTestClient(t)
		_, err := nc.GetServiceItemsWithContext(context.Background(), &client.GetServiceItemsRequest{
			Name: "fastapi",
		})

		require.NoError(t, err)
		// A zero time is "unset", not the epoch - sending it would silently exclude
		// everything modified before 1970, which is to say everything.
		assert.NotContains(t, capturedQuery, "start_date")
		assert.NotContains(t, capturedQuery, "end_date")
	})
}

func TestGetDependantServiceItems(t *testing.T) {
	t.Run("queries the dependant route carrying the filters", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET", serviceItemsRoot+"/dependant/",
			func(req *http.Request) (*http.Response, error) {
				capturedQuery = req.URL.RawQuery
				return httpmock.NewStringResponse(200, emptyPage), nil
			})

		nc := newPackTestClient(t)
		resp, err := nc.GetDependantServiceItems(context.Background(), &client.GetServiceItemsRequest{
			ApplicationName: "app17",
			Limit:           50,
		})

		require.NoError(t, err)
		assert.Equal(t, 0, resp.Count)
		// The extra route shares the plain listing's filterset, so the filters must survive
		// the detour rather than being dropped on the way.
		assert.Contains(t, capturedQuery, "application_name=app17")
		assert.Contains(t, capturedQuery, "limit=50")
	})

	t.Run("honours a consumer POV", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", packTestBaseURL+"/v1/orcabase/consumer/service_items/dependant/",
			httpmock.NewStringResponder(200, emptyPage))

		nc := newPackTestClient(t)
		_, err := nc.GetDependantServiceItems(context.Background(), &client.GetServiceItemsRequest{
			POV: string(client.POVConsumer),
		})

		require.NoError(t, err)
	})

	t.Run("defaults an empty POV to serviceowner", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", serviceItemsRoot+"/dependant/",
			httpmock.NewStringResponder(200, emptyPage))

		nc := newPackTestClient(t)
		_, err := nc.GetDependantServiceItems(context.Background(), nil)

		require.NoError(t, err)
	})
}
