package client_test

import (
	"context"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/netautomate/netorca-go/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// packProfilePtr takes the address of a value, for the pointer tunables on a pack profile.
// It is named for this file so it cannot collide with a helper in a sibling test.
func packProfilePtr[T any](v T) *T { return &v }

// packProfilesURL is the collection route. Note the "ai/" prefix rather than "external/".
const packProfilesURL = packTestBaseURL + "/v1/ai/serviceowner/pack/profiles/"

// packProfileJSON is a full profile as the API renders one, with every serializer field present.
const packProfileJSON = `{
	"id": 7,
	"service": 49,
	"chunk_overlap": 2,
	"max_lines": 10,
	"max_chars": 256,
	"top_k": 10,
	"return_all_documents": false,
	"cosine_similarity_threshold": 0.8,
	"query_config": {"exclude_fields": ["secret"], "exact_search": ["environment"]},
	"embedding_model": "all-MiniLM-L6-v2",
	"pack_enabled": true,
	"universal_executor_enabled": false
}`

func TestListPackProfilesToQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		request  *client.ListPackProfilesRequest
		expected string
	}{
		{
			name:     "No filters set",
			request:  &client.ListPackProfilesRequest{},
			expected: "",
		},
		{
			name:     "POV is not a query parameter",
			request:  &client.ListPackProfilesRequest{POV: client.POVConsumer},
			expected: "",
		},
		{
			name:     "Single service id",
			request:  &client.ListPackProfilesRequest{ServiceID: []int{49}},
			expected: "?service_id=49",
		},
		{
			name:     "Multiple service ids are comma joined",
			request:  &client.ListPackProfilesRequest{ServiceID: []int{49, 50}},
			expected: "?service_id=49%2C50",
		},
		{
			name: "Every filter set",
			request: &client.ListPackProfilesRequest{
				POV:       client.POVServiceOwner,
				ServiceID: []int{49},
				Limit:     20,
				Offset:    5,
				Ordering:  "-max_chars",
			},
			expected: "?limit=20&offset=5&ordering=-max_chars&service_id=49",
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

func TestClientListPackProfiles(t *testing.T) { //nolint:funlen
	const listResponse = `{"count":1,"next":null,"previous":null,"results":[` + packProfileJSON + `]}`

	t.Run("ListPackProfiles decodes the paginated envelope", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", packProfilesURL+"?service_id=49",
			httpmock.NewStringResponder(200, listResponse),
		)

		nc := newPackTestClient(t)
		res, err := nc.ListPackProfiles(context.Background(), &client.ListPackProfilesRequest{
			ServiceID: []int{49},
		})
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, 1, res.Count)
		assert.Nil(t, res.Next)
		assert.Nil(t, res.Previous)
		require.Len(t, res.Results, 1)
		assert.Equal(t, 7, res.Results[0].ID)
		assert.Equal(t, 49, res.Results[0].Service.Int())
	})

	t.Run("ListPackProfiles with nil filters defaults to serviceowner and no query", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", packProfilesURL,
			httpmock.NewStringResponder(200, `{"count":0,"next":null,"previous":null,"results":[]}`),
		)

		nc := newPackTestClient(t)
		res, err := nc.ListPackProfiles(context.Background(), nil)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Empty(t, res.Results)
		assert.Equal(t, 1, httpmock.GetTotalCallCount())
	})

	t.Run("ListPackProfiles honours the consumer POV", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", packTestBaseURL+"/v1/ai/consumer/pack/profiles/",
			httpmock.NewStringResponder(200, `{"count":0,"next":null,"previous":null,"results":[]}`),
		)

		nc := newPackTestClient(t)
		_, err := nc.ListPackProfiles(context.Background(), &client.ListPackProfilesRequest{
			POV: client.POVConsumer,
		})
		require.NoError(t, err)
	})

	t.Run("ListPackProfiles surfaces an unknown filter rejection", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", packProfilesURL,
			httpmock.NewStringResponder(400, `["Filter param not found."]`),
		)

		nc := newPackTestClient(t)
		res, err := nc.ListPackProfiles(context.Background(), nil)
		require.Error(t, err)
		assert.Nil(t, res)
		require.ErrorIs(t, err, client.ErrBadRequest)
		assert.Contains(t, err.Error(), "Filter param not found.")
	})
}

func TestClientGetPackProfile(t *testing.T) { //nolint:funlen
	const detailURL = packProfilesURL + "7/"

	t.Run("GetPackProfile decodes every tunable", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", detailURL, httpmock.NewStringResponder(200, packProfileJSON))

		nc := newPackTestClient(t)
		profile, err := nc.GetPackProfile(context.Background(), client.POVServiceOwner, 7)
		require.NoError(t, err)
		require.NotNil(t, profile)

		assert.Equal(t, 7, profile.ID)
		assert.Equal(t, 49, profile.Service.Int())
		assert.Equal(t, "all-MiniLM-L6-v2", profile.EmbeddingModel)

		require.NotNil(t, profile.ChunkOverlap)
		assert.Equal(t, 2, *profile.ChunkOverlap)
		require.NotNil(t, profile.MaxLines)
		assert.Equal(t, 10, *profile.MaxLines)
		require.NotNil(t, profile.MaxChars)
		assert.Equal(t, 256, *profile.MaxChars)
		require.NotNil(t, profile.TopK)
		assert.Equal(t, 10, *profile.TopK)
		require.NotNil(t, profile.ReturnAllDocuments)
		assert.False(t, *profile.ReturnAllDocuments)
		require.NotNil(t, profile.CosineSimilarityThreshold)
		assert.InDelta(t, 0.8, *profile.CosineSimilarityThreshold, 0.0001)
		require.NotNil(t, profile.PackEnabled)
		assert.True(t, *profile.PackEnabled)
		require.NotNil(t, profile.UniversalExecutorEnabled)
		assert.False(t, *profile.UniversalExecutorEnabled)

		require.NotNil(t, profile.QueryConfig)
		assert.Equal(t, []string{"secret"}, profile.QueryConfig.ExcludeFields)
		assert.Equal(t, []string{"environment"}, profile.QueryConfig.ExactSearch)
	})

	t.Run("GetPackProfile decodes a service rendered as an object", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", detailURL,
			httpmock.NewStringResponder(200, `{"id":7,"service":{"id":49,"name":"VIRTUAL_SERVER"}}`),
		)

		nc := newPackTestClient(t)
		profile, err := nc.GetPackProfile(context.Background(), client.POVServiceOwner, 7)
		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, 49, profile.Service.Int())
	})

	t.Run("GetPackProfile leaves omitted tunables nil", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", detailURL,
			httpmock.NewStringResponder(200, `{"id":7,"service":49,"query_config":{}}`),
		)

		nc := newPackTestClient(t)
		profile, err := nc.GetPackProfile(context.Background(), client.POVServiceOwner, 7)
		require.NoError(t, err)
		require.NotNil(t, profile)

		// Nil, not zero: the API said nothing about these, which is not the same as 0/false.
		assert.Nil(t, profile.TopK)
		assert.Nil(t, profile.PackEnabled)
		assert.Nil(t, profile.CosineSimilarityThreshold)

		// An unconfigured query_config comes back as an empty object rather than null.
		require.NotNil(t, profile.QueryConfig)
		assert.Empty(t, profile.QueryConfig.ExcludeFields)
	})

	t.Run("GetPackProfile returns ErrNotFound on 404", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", detailURL,
			httpmock.NewStringResponder(404, `{"detail":"Not found."}`),
		)

		nc := newPackTestClient(t)
		profile, err := nc.GetPackProfile(context.Background(), client.POVServiceOwner, 7)
		require.Error(t, err)
		assert.Nil(t, profile)
		require.ErrorIs(t, err, client.ErrNotFound)
	})
}

func TestClientFindPackProfile(t *testing.T) { //nolint:funlen
	const findURL = packProfilesURL + "?service_id=49"

	t.Run("FindPackProfile returns the single match", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", findURL,
			httpmock.NewStringResponder(200,
				`{"count":1,"next":null,"previous":null,"results":[`+packProfileJSON+`]}`),
		)

		nc := newPackTestClient(t)
		profile, err := nc.FindPackProfile(context.Background(), client.POVServiceOwner, 49)
		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, 7, profile.ID)
		assert.Equal(t, 49, profile.Service.Int())
	})

	t.Run("FindPackProfile returns ErrNotFound when the service has no profile", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", findURL,
			httpmock.NewStringResponder(200, `{"count":0,"next":null,"previous":null,"results":[]}`),
		)

		nc := newPackTestClient(t)
		profile, err := nc.FindPackProfile(context.Background(), client.POVServiceOwner, 49)
		require.Error(t, err)
		assert.Nil(t, profile)
		require.ErrorIs(t, err, client.ErrNotFound)
		assert.Contains(t, err.Error(), "no pack profile for service 49")
	})

	t.Run("FindPackProfile rejects an ambiguous result rather than guessing", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", findURL,
			httpmock.NewStringResponder(200,
				`{"count":2,"next":null,"previous":null,"results":[{"id":7,"service":49},{"id":8,"service":49}]}`),
		)

		nc := newPackTestClient(t)
		profile, err := nc.FindPackProfile(context.Background(), client.POVServiceOwner, 49)
		require.Error(t, err)
		assert.Nil(t, profile)
		assert.Contains(t, err.Error(), "expected at most one pack profile for service 49, got 2")
	})

	t.Run("FindPackProfile rejects a non-positive service id without calling the API", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		nc := newPackTestClient(t)
		profile, err := nc.FindPackProfile(context.Background(), client.POVServiceOwner, 0)
		require.Error(t, err)
		assert.Nil(t, profile)
		assert.Contains(t, err.Error(), "service id must be positive")
		assert.Equal(t, 0, httpmock.GetTotalCallCount())
	})

	t.Run("FindPackProfile propagates an API failure", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", findURL,
			httpmock.NewStringResponder(403, `{"detail":"You do not have permission."}`),
		)

		nc := newPackTestClient(t)
		profile, err := nc.FindPackProfile(context.Background(), client.POVServiceOwner, 49)
		require.Error(t, err)
		assert.Nil(t, profile)
		require.ErrorIs(t, err, client.ErrForbidden)
		assert.Contains(t, err.Error(), "failed to look up the pack profile for service 49")
	})
}

func TestClientCreatePackProfile(t *testing.T) { //nolint:funlen
	t.Run("CreatePackProfile omits every unset tunable from the body", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST", packProfilesURL,
			captureBodyResponder(&capturedBody, 201, packProfileJSON),
		)

		nc := newPackTestClient(t)
		profile, err := nc.CreatePackProfile(context.Background(), client.POVServiceOwner, &client.PackProfileWrite{
			Service:     49,
			PackEnabled: packProfilePtr(true),
		})
		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, 7, profile.ID)

		// The load-bearing assertion: nothing the caller left alone reaches the wire, so the
		// platform defaults survive. A plain int field would have sent "top_k":0 here.
		assert.JSONEq(t, `{"service":49,"pack_enabled":true}`, capturedBody)
	})

	t.Run("CreatePackProfile sends a deliberate zero", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST", packProfilesURL,
			captureBodyResponder(&capturedBody, 201, packProfileJSON),
		)

		nc := newPackTestClient(t)
		_, err := nc.CreatePackProfile(context.Background(), client.POVServiceOwner, &client.PackProfileWrite{
			Service:            49,
			ChunkOverlap:       packProfilePtr(0),
			ReturnAllDocuments: packProfilePtr(false),
		})
		require.NoError(t, err)

		// The other half of the pointer contract: an explicit zero is not "unset" and must
		// survive to the API, unlike the fields left nil above.
		assert.JSONEq(t, `{"service":49,"chunk_overlap":0,"return_all_documents":false}`, capturedBody)
	})

	t.Run("CreatePackProfile sends the full write shape", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST", packProfilesURL,
			captureBodyResponder(&capturedBody, 201, packProfileJSON),
		)

		nc := newPackTestClient(t)
		_, err := nc.CreatePackProfile(context.Background(), client.POVServiceOwner, &client.PackProfileWrite{
			Service:                   49,
			ChunkOverlap:              packProfilePtr(2),
			MaxLines:                  packProfilePtr(20),
			MaxChars:                  packProfilePtr(512),
			TopK:                      packProfilePtr(5),
			ReturnAllDocuments:        packProfilePtr(true),
			CosineSimilarityThreshold: packProfilePtr(0.65),
			QueryConfig: &client.VectorQueryConfig{
				ExcludeFields: []string{"secret"},
				ExactSearch:   []string{"environment"},
			},
			EmbeddingModel:           "all-MiniLM-L6-v2",
			PackEnabled:              packProfilePtr(false),
			UniversalExecutorEnabled: packProfilePtr(true),
		})
		require.NoError(t, err)

		assert.JSONEq(t, `{
			"service": 49,
			"chunk_overlap": 2,
			"max_lines": 20,
			"max_chars": 512,
			"top_k": 5,
			"return_all_documents": true,
			"cosine_similarity_threshold": 0.65,
			"query_config": {"exclude_fields": ["secret"], "exact_search": ["environment"]},
			"embedding_model": "all-MiniLM-L6-v2",
			"pack_enabled": false,
			"universal_executor_enabled": true
		}`, capturedBody)
	})

	t.Run("CreatePackProfile rejects a nil body without calling the API", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		nc := newPackTestClient(t)
		profile, err := nc.CreatePackProfile(context.Background(), client.POVServiceOwner, nil)
		require.Error(t, err)
		assert.Nil(t, profile)
		assert.Contains(t, err.Error(), "cannot be nil")
		assert.Equal(t, 0, httpmock.GetTotalCallCount())
	})

	t.Run("CreatePackProfile rejects a missing service id", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		nc := newPackTestClient(t)
		profile, err := nc.CreatePackProfile(context.Background(), client.POVServiceOwner, &client.PackProfileWrite{})
		require.Error(t, err)
		assert.Nil(t, profile)
		assert.Contains(t, err.Error(), "requires a service id")
		assert.Equal(t, 0, httpmock.GetTotalCallCount())
	})

	t.Run("CreatePackProfile surfaces the duplicate-profile rejection", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("POST", packProfilesURL,
			httpmock.NewStringResponder(400, `{"service":["pack service profile with this service already exists."]}`),
		)

		nc := newPackTestClient(t)
		profile, err := nc.CreatePackProfile(context.Background(), client.POVServiceOwner, &client.PackProfileWrite{
			Service: 49,
		})
		require.Error(t, err)
		assert.Nil(t, profile)
		require.ErrorIs(t, err, client.ErrBadRequest)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestClientUpdatePackProfile(t *testing.T) { //nolint:funlen
	const detailURL = packProfilesURL + "7/"

	t.Run("UpdatePackProfile PATCHes exactly the given keys", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("PATCH", detailURL, captureBodyResponder(&capturedBody, 200, packProfileJSON))

		nc := newPackTestClient(t)
		profile, err := nc.UpdatePackProfile(context.Background(), client.POVServiceOwner, 7, map[string]any{
			"pack_enabled": false,
			"top_k":        3,
		})
		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, 7, profile.ID)
		assert.JSONEq(t, `{"pack_enabled":false,"top_k":3}`, capturedBody)
	})

	t.Run("UpdatePackProfile can clear the query config", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("PATCH", detailURL, captureBodyResponder(&capturedBody, 200, packProfileJSON))

		nc := newPackTestClient(t)
		_, err := nc.UpdatePackProfile(context.Background(), client.POVServiceOwner, 7, map[string]any{
			"query_config": client.VectorQueryConfig{},
		})
		require.NoError(t, err)
		assert.JSONEq(t, `{"query_config":{}}`, capturedBody)
	})

	t.Run("UpdatePackProfile rejects an empty patch without calling the API", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		nc := newPackTestClient(t)
		profile, err := nc.UpdatePackProfile(context.Background(), client.POVServiceOwner, 7, nil)
		require.Error(t, err)
		assert.Nil(t, profile)
		assert.Contains(t, err.Error(), "cannot be empty")
		assert.Equal(t, 0, httpmock.GetTotalCallCount())
	})

	t.Run("UpdatePackProfile returns ErrNotFound on 404", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("PATCH", detailURL,
			httpmock.NewStringResponder(404, `{"detail":"Not found."}`),
		)

		nc := newPackTestClient(t)
		profile, err := nc.UpdatePackProfile(context.Background(), client.POVServiceOwner, 7, map[string]any{
			"pack_enabled": false,
		})
		require.Error(t, err)
		assert.Nil(t, profile)
		require.ErrorIs(t, err, client.ErrNotFound)
	})
}

func TestClientDeletePackProfile(t *testing.T) {
	const detailURL = packProfilesURL + "7/"

	t.Run("DeletePackProfile succeeds on 204", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("DELETE", detailURL, httpmock.NewStringResponder(204, ""))

		nc := newPackTestClient(t)
		err := nc.DeletePackProfile(context.Background(), client.POVServiceOwner, 7)
		require.NoError(t, err)
		assert.Equal(t, 1, httpmock.GetTotalCallCount())
	})

	t.Run("DeletePackProfile returns ErrNotFound when already gone", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("DELETE", detailURL,
			httpmock.NewStringResponder(404, `{"detail":"Not found."}`),
		)

		nc := newPackTestClient(t)
		err := nc.DeletePackProfile(context.Background(), client.POVServiceOwner, 7)
		require.Error(t, err)
		require.ErrorIs(t, err, client.ErrNotFound)
	})
}
