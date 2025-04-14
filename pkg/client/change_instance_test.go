//nolint:dupl
package client_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/netautomate/netorca-go/config"
	"github.com/netautomate/netorca-go/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangeInstancesToQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		request  *client.GetChangeInstancesRequest
		expected string
	}{
		{
			name: "All fields set",
			request: &client.GetChangeInstancesRequest{
				POV:                 "pov", // will be ignored
				ChangeType:          "type",
				CommitID:            "commit-id",
				ConsumerTeamID:      "team-id",
				Declaration:         "declaration",
				DeclarationContains: "contains",
				DeclarationRegex:    "regex",
				ExcludeReferenced:   true,
				Limit:               10,
				Modified:            time.Date(2025, 4, 9, 11, 11, 4, 194909000, time.UTC),
				Offset:              0,
				ServiceID:           "service-id",
				ServiceItemID:       "item-id",
				ServiceName:         "service-name",
				ServiceOwnerTeamID:  "team-owner-id",
				State:               "state",
				SubmissionID:        "submission-id",
			},
			expected: "change_type=type&commit_id=commit-id&consumer_team_id=team-id&declaration=declaration&declaration_contains=contains&declaration_regex=regex&exclude_referenced=true&limit=10&modified=2025-04-09T11%3A11%3A04Z&service_id=service-id&service_item_id=item-id&service_name=service-name&service_owner_team_id=team-owner-id&state=state&submission_id=submission-id", //nolint
		},
		{
			name:     "No fields set",
			request:  &client.GetChangeInstancesRequest{},
			expected: "",
		},
		{
			name: "Some fields set",
			request: &client.GetChangeInstancesRequest{
				ChangeType: "type",
				CommitID:   "commit-id",
				Limit:      5,
				Offset:     10,
				Ordering:   "name",
			},
			expected: "change_type=type&commit_id=commit-id&limit=5&offset=10&ordering=name",
		},
		{
			name: "Only limit and offset set",
			request: &client.GetChangeInstancesRequest{
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

func exampleChangeInstance() *client.ChangeInstance { //nolint:funlen
	// Example response from the API: testdata/200_single_change_instance_response.json

	return &client.ChangeInstance{
		ID:         53,
		URL:        "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
		State:      "COMPLETED",
		Created:    time.Date(2025, 2, 28, 13, 18, 31, 651446000, time.UTC),
		Modified:   time.Date(2025, 2, 28, 14, 0, 28, 77379000, time.UTC),
		ChangeType: "CREATE",
		Log:        "",
		Owner: client.Team{
			ID:   4,
			Name: "AWS",
		},
		ServiceItem: client.ServiceItem{
			ID:           31,
			Name:         "django-app7",
			RuntimeState: "IN_SERVICE",
			Declaration: json.RawMessage(`{
		    "name": "django-app7",
		    "size": "small",
		    "image": "ami-02141377eee7defb9",
		    "owner": "app7@test.com",
		    "description": "Django app for alpha",
		    "environment": "dev"
			}`),
			DeployedItem: json.RawMessage(` {
				"data": "netorca terraform"
			  }`),
		},

		Submission: client.Submission{
			ID:       31,
			CommitID: "51e53e75292438c573f37152e1b831e4cd80bbc4",
		},
		NewDeclaration: client.Declaration{
			Version: 1,
			Declaration: json.RawMessage(`{
		    "name": "django-app7",
		    "size": "small",
		    "image": "ami-02141377eee7defb9",
		    "owner": "app7@test.com",
		    "description": "Django app for alpha",
		    "environment": "dev"}`),
		},
		ServiceOwnerTeam: client.Team{
			ID:       4,
			Name:     "AWS",
			Metadata: json.RawMessage(`{}`),
		},
		ConsumerTeam: client.Team{
			ID:       1,
			Name:     "alpha",
			Metadata: json.RawMessage(`{"team_name":"alpha"}`),
		},
		Service: client.ChangeInstanceService{
			ID:                    4,
			Name:                  "THREE_TIER_APPLICATION",
			AllowManualApproval:   true,
			AllowManualCompletion: true,
		},
		Application: client.Application{
			ID:       19,
			Name:     "app7",
			Metadata: json.RawMessage(`{"owner": "team@example.com","description": "My app7","environment": "DEV"}`)},
		IsDependant:    false,
		OldDeclaration: nil,
	}
}

func TestClientGetChangeInstances(t *testing.T) { //nolint:funlen
	// Test responses with mocked HTTP requests
	// Responses are mocked using httpmock
	t.Run("Test NewClient returns empty response with 200 when no filters matched", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances",
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
		filters := &client.GetChangeInstancesRequest{
			POV:       "serviceowner",
			Limit:     10,
			Offset:    0,
			ServiceID: "12321321",
		}
		changeInstances, err := nc.GetChangeInstances(filters)
		require.NoError(t, err)
		assert.NotNil(t, changeInstances)

		assert.Equal(t, 0, changeInstances.Count)
		assert.Equal(t, []client.ChangeInstance{}, changeInstances.Results)
		assert.Nil(t, changeInstances.Next)
		assert.Nil(t, changeInstances.Previous)
	})
	t.Run("Test GetChangeInstances when api responds with 200 with single record", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		// Register a mock response for the GET request with real data
		testFileContent := readTestFile(t, "200_single_change_instance_response.json")
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances",
			httpmock.NewStringResponder(200, testFileContent),
		)

		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}

		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)
		filters := &client.GetChangeInstancesRequest{
			POV:       "serviceowner",
			Limit:     10,
			Offset:    0,
			ServiceID: "4",
		}
		changeInstances, err := nc.GetChangeInstances(filters)
		require.NoError(t, err)
		assert.NotNil(t, changeInstances)
		expectedCI := exampleChangeInstance()

		assert.Equal(t, 1, changeInstances.Count)
		assert.Equal(t, expectedCI.ID, changeInstances.Results[0].ID)
		actualCIJSON, err := json.Marshal(changeInstances.Results[0])
		require.NoError(t, err)
		expectedCIJSON, err := json.Marshal(*expectedCI)
		require.NoError(t, err)
		// Compare entire json
		assert.JSONEq(t, string(expectedCIJSON), string(actualCIJSON))

		// compare all fields one by one for better error messages
		assert.Equal(t, expectedCI.ID, changeInstances.Results[0].ID)
		assert.Equal(t, expectedCI.URL, changeInstances.Results[0].URL)
		assert.Equal(t, expectedCI.State, changeInstances.Results[0].State)
		assert.Equal(t, expectedCI.Created, changeInstances.Results[0].Created)
		assert.Equal(t, expectedCI.Modified, changeInstances.Results[0].Modified)
		assert.Equal(t, expectedCI.ChangeType, changeInstances.Results[0].ChangeType)
		assert.Equal(t, expectedCI.Log, changeInstances.Results[0].Log)
		assert.Equal(t, expectedCI.Owner.ID, changeInstances.Results[0].Owner.ID)
		assert.Equal(t, expectedCI.Owner.Name, changeInstances.Results[0].Owner.Name)
		assert.Equal(t, expectedCI.Owner.Metadata, changeInstances.Results[0].Owner.Metadata)
		assert.Equal(t, expectedCI.ServiceItem.ID, changeInstances.Results[0].ServiceItem.ID)
		assert.Equal(t, expectedCI.ServiceItem.Name, changeInstances.Results[0].ServiceItem.Name)
		assert.Equal(t, expectedCI.ServiceItem.RuntimeState, changeInstances.Results[0].ServiceItem.RuntimeState)
		assert.Equal(t, expectedCI.Submission.ID, changeInstances.Results[0].Submission.ID)
		assert.Equal(t, expectedCI.Submission.CommitID, changeInstances.Results[0].Submission.CommitID)
		assert.Equal(t, expectedCI.NewDeclaration.Version, changeInstances.Results[0].NewDeclaration.Version)
		assert.Equal(t, expectedCI.ServiceOwnerTeam.ID, changeInstances.Results[0].ServiceOwnerTeam.ID)
		assert.Equal(t, expectedCI.ServiceOwnerTeam.Name, changeInstances.Results[0].ServiceOwnerTeam.Name)
		assert.Equal(t, expectedCI.ConsumerTeam.ID, changeInstances.Results[0].ConsumerTeam.ID)
		assert.Equal(t, expectedCI.ConsumerTeam.Name, changeInstances.Results[0].ConsumerTeam.Name)
		assert.Equal(t, expectedCI.Service.ID, changeInstances.Results[0].Service.ID)
		assert.Equal(t, expectedCI.Service.Name, changeInstances.Results[0].Service.Name)
		assert.Equal(t, expectedCI.Service.AllowManualApproval, changeInstances.Results[0].Service.AllowManualApproval)
		assert.Equal(t, expectedCI.Service.AllowManualCompletion, changeInstances.Results[0].Service.AllowManualCompletion)
		assert.Equal(t, expectedCI.Application.ID, changeInstances.Results[0].Application.ID)
		assert.Equal(t, expectedCI.Application.Name, changeInstances.Results[0].Application.Name)
		assert.Equal(t, expectedCI.IsDependant, changeInstances.Results[0].IsDependant)
		assert.Equal(t, expectedCI.ServiceItem.Name, changeInstances.Results[0].ServiceItem.Name)
		assert.Equal(t, expectedCI.ServiceItem.ID, changeInstances.Results[0].ServiceItem.ID)
		assert.Equal(t, expectedCI.ServiceItem.RuntimeState, changeInstances.Results[0].ServiceItem.RuntimeState)
		assert.Equal(t, expectedCI.ServiceItem.Name, changeInstances.Results[0].ServiceItem.Name)
	})
	t.Run("Test GetChangeInstances when api responds with 500", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances",
			httpmock.NewStringResponder(500, `{"error": "Internal Server Error"}`),
		)

		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}

		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)
		filters := &client.GetChangeInstancesRequest{
			POV:       "serviceowner",
			Limit:     10,
			Offset:    0,
			ServiceID: "12321321",
		}
		changeInstances, err := nc.GetChangeInstances(filters)
		require.Error(t, err)
		assert.Nil(t, changeInstances)
		assert.Equal(t, "failed to get change instances: 500 Internal Server Error", err.Error())
	})
	t.Run("Test GetChangeInstances when api responds with 400", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances",
			httpmock.NewStringResponder(400, `{"error": "Bad Request"}`),
		)

		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}

		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)
		filters := &client.GetChangeInstancesRequest{
			POV:       "serviceowner",
			Limit:     10,
			Offset:    0,
			ServiceID: "12321321",
		}
		changeInstances, err := nc.GetChangeInstances(filters)
		require.Error(t, err)
		assert.Nil(t, changeInstances)
		assert.Equal(t, "failed to get change instances: 400 Bad Request", err.Error())
	},
	)
}

func TestClientApproveChangeInstance(t *testing.T) { //nolint:funlen
	t.Run("Test ApproveChangeInstance when api responds with 500", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(500, `{"error": "Internal Server Error"}`),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)

		changeInstance, err := nc.ApproveChangeInstance(53, "test log", json.RawMessage(`{"comment": "approved"}`))

		require.Error(t, err)
		assert.Nil(t, changeInstance)
	})
	t.Run("Test ApproveChangeInstance when api responds with 400", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(400, `{"error": "Bad Request"}`),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)

		changeInstance, err := nc.ApproveChangeInstance(53, "test log", json.RawMessage(`{"comment": "approved"}`))

		require.Error(t, err)
		assert.Nil(t, changeInstance)
	})
	t.Run("Test ApproveChangeInstance when api responds with 200", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		testFileContent := readTestFile(t, "200_APPROVE_single_change_instance_response.json")

		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(200, testFileContent),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)

		changeInstance, err := nc.CompleteChangeInstance(53, "test log", json.RawMessage(`{"comment": "approved"}`))
		require.NoError(t, err)
		assert.NotNil(t, changeInstance)
		assert.NotEqual(t, client.ChangeInstance{}, *changeInstance)
		assert.Equal(t, 53, changeInstance.ID)
		assert.Equal(t, "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/", changeInstance.URL)
		assert.Equal(t, string(client.ChangeInstanceAPPROVED), changeInstance.State)
		assert.Equal(t, "test log", changeInstance.Log)
		assert.JSONEq(t, `{"comment":"approved"}`, string(changeInstance.ServiceItem.DeployedItem))
	})
}

func TestClientCompleteChangeInstance(t *testing.T) { //nolint:funlen
	t.Run("Test CompleteChangeInstance when api responds with 500", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(500, `{"error": "Internal Server Error"}`),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)

		changeInstance, err := nc.CompleteChangeInstance(53, "test log", json.RawMessage(`{"comment": "completed"}`))

		require.Error(t, err)
		assert.Nil(t, changeInstance)
	})
	t.Run("Test CompleteChangeInstance when api responds with 400", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(400, `{"error": "Bad Request"}`),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)

		changeInstance, err := nc.CompleteChangeInstance(53, "test log", json.RawMessage(`{"comment": "completed"}`))

		require.Error(t, err)
		assert.Nil(t, changeInstance)
	})
	t.Run("Test CompleteChangeInstance when api responds with 200", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		testFileContent := readTestFile(t, "200_COMPLETE_single_change_instance_response.json")

		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(200, testFileContent),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)

		changeInstance, err := nc.CompleteChangeInstance(53, "test log", json.RawMessage(`{"comment": "completed"}`))
		require.NoError(t, err)
		assert.NotNil(t, changeInstance)
		assert.NotEqual(t, client.ChangeInstance{}, *changeInstance)
		assert.Equal(t, 53, changeInstance.ID)
		assert.Equal(t, "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/", changeInstance.URL)
		assert.Equal(t, string(client.ChangeInstanceCOMPLETED), changeInstance.State)
		assert.Equal(t, "test log", changeInstance.Log)
		assert.JSONEq(t, `{"comment":"completed"}`, string(changeInstance.ServiceItem.DeployedItem))
	})
}

func TestClientCloseChangeInstance(t *testing.T) { //nolint:funlen
	t.Run("Test CloseChangeInstance when api responds with 500", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(500, `{"error": "Internal Server Error"}`),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)

		changeInstance, err := nc.CloseChangeInstance(53, "test log", json.RawMessage(`{"comment": "closed"}`))

		require.Error(t, err)
		assert.Nil(t, changeInstance)
	})
	t.Run("Test CloseChangeInstance when api responds with 400", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(400, `{"error": "Bad Request"}`),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)

		changeInstance, err := nc.CloseChangeInstance(53, "test log", json.RawMessage(`{"comment": "closed"}`))

		require.Error(t, err)
		assert.Nil(t, changeInstance)
	})
	t.Run("Test CloseChangeInstance when api responds with 200", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		testFileContent := readTestFile(t, "200_CLOSE_single_change_instance_response.json")

		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(200, testFileContent),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)

		changeInstance, err := nc.CompleteChangeInstance(53, "test log", json.RawMessage(`{"comment": "closed"}`))
		require.NoError(t, err)
		assert.NotNil(t, changeInstance)
		assert.NotEqual(t, client.ChangeInstance{}, *changeInstance)
		assert.Equal(t, 53, changeInstance.ID)
		assert.Equal(t, "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/", changeInstance.URL)
		assert.Equal(t, string(client.ChangeInstanceCLOSED), changeInstance.State)
		assert.Equal(t, "test log", changeInstance.Log)
		assert.JSONEq(t, `{"comment":"closed"}`, string(changeInstance.ServiceItem.DeployedItem))
	})
}

func TestClientRejectChangeInstance(t *testing.T) { //nolint:funlen
	t.Run("Test CompleteChangeInstance when api responds with 500", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(500, `{"error": "Internal Server Error"}`),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)

		changeInstance, err := nc.CompleteChangeInstance(53, "test log", json.RawMessage(`{"comment": "rejected"}`))

		require.Error(t, err)
		assert.Nil(t, changeInstance)
	})
	t.Run("Test CompleteChangeInstance when api responds with 400", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(400, `{"error": "Bad Request"}`),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)

		changeInstance, err := nc.CompleteChangeInstance(53, "test log", json.RawMessage(`{"comment": "rejected"}`))

		require.Error(t, err)
		assert.Nil(t, changeInstance)
	})
	t.Run("Test CompleteChangeInstance when api responds with 200", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		testFileContent := readTestFile(t, "200_REJECT_single_change_instance_response.json")

		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(200, testFileContent),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)

		changeInstance, err := nc.CompleteChangeInstance(53, "test log", json.RawMessage(`{"comment": "rejected"}`))
		require.NoError(t, err)
		assert.NotNil(t, changeInstance)
		assert.NotEqual(t, client.ChangeInstance{}, *changeInstance)
		assert.Equal(t, 53, changeInstance.ID)
		assert.Equal(t, "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/", changeInstance.URL)
		assert.Equal(t, string(client.ChangeInstanceREJECTED), changeInstance.State)
		assert.Equal(t, "test log", changeInstance.Log)
		assert.JSONEq(t, `{"comment":"rejected"}`, string(changeInstance.ServiceItem.DeployedItem))
	})
}

func TestClientSetErrorChangeInstance(t *testing.T) { //nolint:funlen
	t.Run("Test CompleteChangeInstance when api responds with 500", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(500, `{"error": "Internal Server Error"}`),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)

		changeInstance, err := nc.SetErrorChangeInstance(53, "test log", json.RawMessage(`{"comment": "error"}`))

		require.Error(t, err)
		assert.Nil(t, changeInstance)
	})
	t.Run("Test CompleteChangeInstance when api responds with 400", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(400, `{"error": "Bad Request"}`),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)
		changeInstance, err := nc.SetErrorChangeInstance(53, "test log", json.RawMessage(`{"comment": "error"}`))
		require.Error(t, err)
		assert.Nil(t, changeInstance)
		assert.Equal(
			t,
			`failed to update change instance state. Details: 400 Bad Request, {"error": "Bad Request"}`,
			err.Error(),
		)
	})
	t.Run("Test CompleteChangeInstance when api responds with 200", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		testFileContent := readTestFile(t, "200_ERROR_single_change_instance_response.json")

		httpmock.RegisterResponder("PATCH", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/",
			httpmock.NewStringResponder(200, testFileContent),
		)
		cfg := config.Config{
			BaseURL:    "http://api-aws.demo.netorca.io",
			APIKey:     "test-api-key",
			APIVersion: "v1",
		}
		nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
		require.NoError(t, err)

		changeInstance, err := nc.SetErrorChangeInstance(53, "test log", json.RawMessage(`{"comment": "error"}`))
		require.NoError(t, err)
		assert.NotNil(t, changeInstance)
		assert.NotEqual(t, client.ChangeInstance{}, *changeInstance)
		assert.Equal(t, 53, changeInstance.ID)
		assert.Equal(t, "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/", changeInstance.URL)
		assert.Equal(t, string(client.ChangeInstanceERROR), changeInstance.State)
		assert.Equal(t, "test log", changeInstance.Log)
		assert.JSONEq(t, `{"comment":"error"}`, string(changeInstance.ServiceItem.DeployedItem))
	})
}
