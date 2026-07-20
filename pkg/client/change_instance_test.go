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

const changeInstancesRoot = packTestBaseURL + "/v1/orcabase/serviceowner/change_instances"

// changeInstanceHistory is the trail of a change raised by a consumer, approved by a service
// owner's API key with a note, then completed by the platform itself. It shows the three things
// about the shape a caller has to know: id repeats (it is the change's id, not the entry's),
// changed_by reads "SYSTEM" for a platform transition, and changed_by_team can be null.
const changeInstanceHistory = `[
  {
    "id": 7,
    "state": "COMPLETED",
    "log": "Deployed to lon-dc1.",
    "modified": "2026-07-19T14:03:00Z",
    "reason": "state",
    "changed_by": "SYSTEM",
    "changed_by_team": null
  },
  {
    "id": 7,
    "state": "APPROVED",
    "log": "Capacity confirmed.",
    "modified": "2026-07-19T11:20:00Z",
    "reason": "state",
    "changed_by": "terraform-key (Api Key)",
    "changed_by_team": "AWS"
  }
]`

//nolint:funlen // a filter table: every case has to spell out the fields it is pinning
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
				ApplicationID:       "app-id",
				ChangeType:          "type",
				CommitID:            "commit-id",
				ConsumerTeamID:      "team-id",
				Declaration:         "declaration",
				DeclarationContains: "contains",
				DeclarationRegex:    "regex",
				EndDate:             time.Date(2025, 4, 30, 0, 0, 0, 0, time.UTC),
				ExcludeReferenced:   true,
				Limit:               10,
				Modified:            time.Date(2025, 4, 9, 11, 11, 4, 194909000, time.UTC),
				Offset:              0,
				ServiceID:           "service-id",
				ServiceItemID:       "item-id",
				ServiceName:         "service-name",
				ServiceOwnerTeamID:  "team-owner-id",
				StartDate:           time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
				State:               "state",
				SubmissionID:        "submission-id",
			},
			expected: "application_id=app-id&change_type=type&commit_id=commit-id&consumer_team_id=team-id&declaration=declaration&declaration_contains=contains&declaration_regex=regex&end_date=2025-04-30T00%3A00%3A00Z&exclude_referenced=true&limit=10&modified=2025-04-09T11%3A11%3A04Z&service_id=service-id&service_item_id=item-id&service_name=service-name&service_owner_team_id=team-owner-id&start_date=2025-04-01T00%3A00%3A00Z&state=state&submission_id=submission-id", //nolint
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
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/",
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
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/",
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
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/",
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
		require.ErrorContains(t, err, "500 Internal Server Error")
	})
	t.Run("Test GetChangeInstances when api responds with 400", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/",
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
		require.ErrorIs(t, err, client.ErrBadRequest)
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
	t.Run("Test RejectChangeInstance when api responds with 500", func(t *testing.T) {
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

		changeInstance, err := nc.RejectChangeInstance(53, "test log", json.RawMessage(`{"comment": "rejected"}`))

		require.Error(t, err)
		assert.Nil(t, changeInstance)
	})
	t.Run("Test RejectChangeInstance when api responds with 400", func(t *testing.T) {
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

		changeInstance, err := nc.RejectChangeInstance(53, "test log", json.RawMessage(`{"comment": "rejected"}`))

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

		changeInstance, err := nc.RejectChangeInstance(53, "test log", json.RawMessage(`{"comment": "rejected"}`))
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
		// The failure surfaces as an APIError carrying the server's own explanation, so a
		// caller can both branch on the status and show the practitioner why it was rejected.
		require.ErrorIs(t, err, client.ErrBadRequest)
		require.ErrorContains(t, err, `{"error": "Bad Request"}`)
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

// TestUpdateChangeInstanceOptionalFields pins the omitempty behaviour on the transition body.
//
// It exists because a live run caught the opposite: json.RawMessage is a []byte, and a nil one
// marshals to the literal null rather than being omitted, so the API rejected every transition
// that did not also assert a deployed item. Mocked tests never noticed because they all passed
// one.
func TestUpdateChangeInstanceOptionalFields(t *testing.T) {
	const patchURL = "http://api-aws.demo.netorca.io/v1/orcabase/serviceowner/change_instances/53/"

	t.Run("a nil deployed item is omitted, not sent as null", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("PATCH", patchURL,
			captureBodyResponder(&capturedBody, 200, `{"id":53,"state":"APPROVED"}`))

		nc := newPackTestClient(t)
		_, err := nc.ApproveChangeInstance(53, "looks good", nil)

		require.NoError(t, err)
		assert.JSONEq(t, `{"state":"APPROVED","log":"looks good"}`, capturedBody)
		assert.NotContains(t, capturedBody, "deployed_item")
	})

	t.Run("an empty log is omitted so it does not blank an existing one", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("PATCH", patchURL,
			captureBodyResponder(&capturedBody, 200, `{"id":53,"state":"APPROVED"}`))

		nc := newPackTestClient(t)
		_, err := nc.ApproveChangeInstance(53, "", nil)

		require.NoError(t, err)
		assert.JSONEq(t, `{"state":"APPROVED"}`, capturedBody)
	})

	t.Run("a supplied deployed item still goes on the wire", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("PATCH", patchURL,
			captureBodyResponder(&capturedBody, 200, `{"id":53,"state":"COMPLETED"}`))

		nc := newPackTestClient(t)
		_, err := nc.CompleteChangeInstance(53, "deployed", json.RawMessage(`{"vip":"10.1.1.1"}`))

		require.NoError(t, err)
		assert.JSONEq(t, `{"state":"COMPLETED","log":"deployed","deployed_item":{"vip":"10.1.1.1"}}`, capturedBody)
	})
}

// TestGetChangeInstancesFilterWire pins the new filters to the wire. The backend's filterset
// rejects any parameter it does not declare with a 400, so the names matter as much as the
// values - and application_id in particular had no way of being sent at all before.
func TestGetChangeInstancesFilterWire(t *testing.T) {
	t.Run("sends application_id and the modified window", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET", changeInstancesRoot+"/",
			func(req *http.Request) (*http.Response, error) {
				capturedQuery = req.URL.RawQuery
				return httpmock.NewStringResponse(200, emptyPage), nil
			})

		nc := newPackTestClient(t)
		_, err := nc.GetChangeInstancesWithContext(context.Background(), &client.GetChangeInstancesRequest{
			ApplicationID: "23,24",
			StartDate:     time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
			EndDate:       time.Date(2025, 4, 30, 0, 0, 0, 0, time.UTC),
		})

		require.NoError(t, err)
		// A comma-joined value is the platform's "in" lookup, so this asks for either.
		assert.Contains(t, capturedQuery, "application_id=23%2C24")
		assert.Contains(t, capturedQuery, "start_date=2025-04-01T00%3A00%3A00Z")
		assert.Contains(t, capturedQuery, "end_date=2025-04-30T00%3A00%3A00Z")
	})

	t.Run("omits the date filters when unset", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET", changeInstancesRoot+"/",
			func(req *http.Request) (*http.Response, error) {
				capturedQuery = req.URL.RawQuery
				return httpmock.NewStringResponse(200, emptyPage), nil
			})

		nc := newPackTestClient(t)
		_, err := nc.GetChangeInstancesWithContext(context.Background(), &client.GetChangeInstancesRequest{
			State: "PENDING",
		})

		require.NoError(t, err)
		// A zero time is "unset", not the epoch - sending it would bound the window at 1970
		// and quietly drop everything.
		assert.NotContains(t, capturedQuery, "start_date")
		assert.NotContains(t, capturedQuery, "end_date")
	})
}

func TestGetDependantChangeInstances(t *testing.T) {
	t.Run("queries the dependant route carrying the filters", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET", changeInstancesRoot+"/dependant/",
			func(req *http.Request) (*http.Response, error) {
				capturedQuery = req.URL.RawQuery
				return httpmock.NewStringResponse(200, emptyPage), nil
			})

		nc := newPackTestClient(t)
		resp, err := nc.GetDependantChangeInstances(context.Background(), &client.GetChangeInstancesRequest{
			ApplicationID: "23",
			State:         "PENDING",
		})

		require.NoError(t, err)
		assert.Equal(t, 0, resp.Count)
		// The extra route shares the plain listing's filterset, so the filters must survive
		// the detour rather than being dropped on the way.
		assert.Contains(t, capturedQuery, "application_id=23")
		assert.Contains(t, capturedQuery, "state=PENDING")
	})

	t.Run("defaults an empty POV to serviceowner", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", changeInstancesRoot+"/dependant/",
			httpmock.NewStringResponder(200, emptyPage))

		nc := newPackTestClient(t)
		_, err := nc.GetDependantChangeInstances(context.Background(), nil)

		require.NoError(t, err)
	})
}

func TestGetReferencedChangeInstances(t *testing.T) {
	t.Run("queries the referenced route carrying the filters", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET", changeInstancesRoot+"/referenced/",
			func(req *http.Request) (*http.Response, error) {
				capturedQuery = req.URL.RawQuery
				return httpmock.NewStringResponse(200, emptyPage), nil
			})

		nc := newPackTestClient(t)
		_, err := nc.GetReferencedChangeInstances(context.Background(), &client.GetChangeInstancesRequest{
			ServiceItemID: "389",
			Limit:         25,
		})

		require.NoError(t, err)
		assert.Contains(t, capturedQuery, "service_item_id=389")
		assert.Contains(t, capturedQuery, "limit=25")
	})

	t.Run("honours a consumer POV", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", packTestBaseURL+"/v1/orcabase/consumer/change_instances/referenced/",
			httpmock.NewStringResponder(200, emptyPage))

		nc := newPackTestClient(t)
		_, err := nc.GetReferencedChangeInstances(context.Background(), &client.GetChangeInstancesRequest{
			POV: string(client.POVConsumer),
		})

		require.NoError(t, err)
	})
}

//nolint:funlen // four subtests over one fixture; splitting them would scatter the shape assertions
func TestListChangeInstanceHistory(t *testing.T) {
	t.Run("decodes the paginated envelope the route normally returns", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedPath string
		httpmock.RegisterResponder("GET", changeInstancesRoot+"/7/history/",
			func(req *http.Request) (*http.Response, error) {
				capturedPath = req.URL.Path
				body := `{"count":2,"next":null,"previous":null,"results":` + changeInstanceHistory + `}`
				return httpmock.NewStringResponse(200, body), nil
			})

		nc := newPackTestClient(t)
		entries, err := nc.ListChangeInstanceHistory(context.Background(), client.POVServiceOwner, 7)

		require.NoError(t, err)
		// The id belongs once in the path. Repeating it, as the Python SDK does, gives a
		// route that does not exist.
		assert.Equal(t, "/v1/orcabase/serviceowner/change_instances/7/history/", capturedPath)

		require.Len(t, entries, 2)
		// Entries arrive newest first, and every one repeats the change's id rather than
		// carrying an identifier of its own.
		assert.Equal(t, "COMPLETED", entries[0].State)
		assert.Equal(t, 7, entries[0].ID)
		assert.Equal(t, 7, entries[1].ID)
		assert.Equal(t, "Capacity confirmed.", entries[1].Log)

		// A platform transition is attributed to nobody's team, which is why the field is
		// a pointer rather than a string.
		require.NotNil(t, entries[0].ChangedBy)
		assert.Equal(t, "SYSTEM", *entries[0].ChangedBy)
		assert.Nil(t, entries[0].ChangedByTeam)
		require.NotNil(t, entries[1].ChangedByTeam)
		assert.Equal(t, "AWS", *entries[1].ChangedByTeam)
	})

	t.Run("decodes a bare array when pagination is switched off", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// An on-prem deployment can turn the paginator off instance-wide, at which point
		// the view stops wrapping its results.
		httpmock.RegisterResponder("GET", changeInstancesRoot+"/7/history/",
			httpmock.NewStringResponder(200, changeInstanceHistory))

		nc := newPackTestClient(t)
		entries, err := nc.ListChangeInstanceHistory(context.Background(), client.POVServiceOwner, 7)

		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Equal(t, "APPROVED", entries[1].State)
		assert.Equal(t, "state", entries[1].Reason)
	})

	t.Run("defaults an empty POV to serviceowner", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", changeInstancesRoot+"/7/history/",
			httpmock.NewStringResponder(200, `[]`))

		nc := newPackTestClient(t)
		entries, err := nc.ListChangeInstanceHistory(context.Background(), "", 7)

		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("surfaces a missing change as ErrNotFound", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", changeInstancesRoot+"/999999/history/",
			httpmock.NewStringResponder(404, `{"detail":"Not found."}`))

		nc := newPackTestClient(t)
		entries, err := nc.ListChangeInstanceHistory(context.Background(), client.POVServiceOwner, 999999)

		require.Error(t, err)
		assert.Nil(t, entries)
		require.ErrorIs(t, err, client.ErrNotFound)
	})
}
