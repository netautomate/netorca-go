package client_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/netautomate/netorca-go/config"
	"github.com/netautomate/netorca-go/pkg/client"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readTestFile(t *testing.T, filename string) string {
	testFilePath := filepath.Join("testdata", filename)
	content, err := os.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to read test file %s: %v", testFilePath, err)
	}
	return string(content)
}

func exampleServiceItem() *client.ServiceItem {
	// Example response from the API: testdata/single_service_item_response.json
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

func TestClient(t *testing.T) {
	t.Run("Test NewClient returns client on success", func(t *testing.T) {
		client, err := client.NewClient("http://example.com", "test-api-key", "v1", 5*time.Second)
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, "http://example.com/v1/", client.BaseURL)
		assert.Equal(t, "test-api-key", client.APIKey)
	})

	t.Run("Test NewClient returns error on empty base URL", func(t *testing.T) {
		_, err := client.NewClient("", "test-api-key", "v1", 5*time.Second)
		require.Error(t, err)
		assert.Equal(t, "base URL cannot be empty", err.Error())
	})
	t.Run("Test NewClient returns error invalid base URL", func(t *testing.T) {
		_, err := client.NewClient("invalid-url", "test-api-key", "v1", 5*time.Second)
		require.Error(t, err)
		assert.Equal(t, "base URL must start with http:// or https://", err.Error())
	})
}

func TestClientServiceItems(t *testing.T) { //nolint:funlen
	// Test responses with mocked HTTP requests
	// Responses are mocked using httpmock

	// skip this test if the environment variable is not set
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}
	t.Run("Test NewClient returns empty response with 200 when no filters matched", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/service_items",
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
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/service_items",
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
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/service_items",
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
		assert.Equal(t, "failed to get service items: 500 Internal Server Error", err.Error())
	})
	t.Run("Test GetServiceItems returns error on 400", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/service_items",
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
		assert.Equal(t, "failed to get service items: 400 Bad Request", err.Error())
	})
}
