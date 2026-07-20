package client_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/netautomate/netorca-go/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pipelinesRoot = packTestBaseURL + "/v1/external/serviceowner/pack/pipelines"

// onePipeline is a minimal but realistic run: successful, unapplied, with config stage data.
// That combination - state OK and applied false - is the executor work queue.
const onePipeline = `{
  "id": 2935,
  "version": 4,
  "state": "OK",
  "current_stage": "execution",
  "applied": false,
  "cost": "0.0431",
  "created": "2026-07-19T10:11:12Z",
  "config": {
    "id": 6600,
    "action_type": "config",
    "object_id": 389,
    "data": {"as3_json": {"id": "vs_demo"}},
    "scope": {"scope": "service_item", "data": {"id": 389, "name": "demo"}}
  },
  "verify": null,
  "execution": null
}`

func TestListPackPipelines(t *testing.T) {
	t.Run("returns the executor work queue for state OK and applied false", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", pipelinesRoot+"/",
			httpmock.NewStringResponder(200, `{"count":1,"next":null,"previous":null,"results":[`+onePipeline+`]}`))

		nc := newPackTestClient(t)
		applied := false
		resp, err := nc.ListPackPipelines(context.Background(), &client.ListPackPipelinesRequest{
			State:   []string{string(client.PackPipelineOK)},
			Applied: &applied,
		})

		require.NoError(t, err)
		assert.Equal(t, 1, resp.Count)
		require.Len(t, resp.Results, 1)
		assert.Equal(t, 2935, resp.Results[0].ID)
		assert.False(t, resp.Results[0].Applied)
		assert.Equal(t, string(client.PackPipelineOK), resp.Results[0].State)

		// The embedded stage data is what the executor deploys, and its scope is read back
		// off the record rather than assumed.
		require.NotNil(t, resp.Results[0].Config)
		assert.Equal(t, client.PackScopeServiceItem, resp.Results[0].Config.ScopeKind())
	})

	t.Run("sends applied=false rather than omitting it", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET", pipelinesRoot+"/",
			func(req *http.Request) (*http.Response, error) {
				capturedQuery = req.URL.RawQuery
				return httpmock.NewStringResponse(200, `{"count":0,"results":[]}`), nil
			})

		nc := newPackTestClient(t)
		applied := false
		_, err := nc.ListPackPipelines(context.Background(), &client.ListPackPipelinesRequest{
			State:         []string{"OK"},
			Applied:       &applied,
			ServiceItemID: []int{389, 412},
		})

		require.NoError(t, err)
		// applied=false is a meaningful query, not an absent one - the whole executor
		// queue depends on it surviving the "drop empty values" pass.
		assert.Contains(t, capturedQuery, "applied=false")
		assert.Contains(t, capturedQuery, "state=OK")
		// Lists become comma-joined "in" lookups.
		assert.Contains(t, capturedQuery, "service_item_id=389%2C412")
	})

	t.Run("omits a nil applied filter entirely", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET", pipelinesRoot+"/",
			func(req *http.Request) (*http.Response, error) {
				capturedQuery = req.URL.RawQuery
				return httpmock.NewStringResponse(200, `{"count":0,"results":[]}`), nil
			})

		nc := newPackTestClient(t)
		_, err := nc.ListPackPipelines(context.Background(), &client.ListPackPipelinesRequest{})

		require.NoError(t, err)
		assert.NotContains(t, capturedQuery, "applied")
	})

	t.Run("surfaces a 403 as ErrForbidden", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", pipelinesRoot+"/",
			httpmock.NewStringResponder(403, `{"detail":"You do not have permission to perform this action."}`))

		nc := newPackTestClient(t)
		resp, err := nc.ListPackPipelines(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.ErrorIs(t, err, client.ErrForbidden)
		assert.ErrorContains(t, err, "You do not have permission")
	})
}

func TestGetPackPipeline(t *testing.T) {
	t.Run("returns one run by id", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", pipelinesRoot+"/2935/",
			httpmock.NewStringResponder(200, onePipeline))

		nc := newPackTestClient(t)
		pipeline, err := nc.GetPackPipeline(context.Background(), client.POVServiceOwner, 2935)

		require.NoError(t, err)
		assert.Equal(t, 4, pipeline.Version)

		cost, err := pipeline.ParseCost()
		require.NoError(t, err)
		assert.InDelta(t, 0.0431, cost, 1e-9)
	})

	t.Run("surfaces a missing run as ErrNotFound", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", pipelinesRoot+"/999999/",
			httpmock.NewStringResponder(404, `{"detail":"Not found."}`))

		nc := newPackTestClient(t)
		pipeline, err := nc.GetPackPipeline(context.Background(), client.POVServiceOwner, 999999)

		require.Error(t, err)
		assert.Nil(t, pipeline)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}

func TestGetLatestPackPipeline(t *testing.T) {
	t.Run("returns the newest run for a scoped object", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", pipelinesRoot+"/latest/service_item/389/",
			httpmock.NewStringResponder(200, onePipeline))

		nc := newPackTestClient(t)
		pipeline, err := nc.GetLatestPackPipeline(
			context.Background(), client.POVServiceOwner, client.PackScopeServiceItem, 389,
		)

		require.NoError(t, err)
		assert.Equal(t, 2935, pipeline.ID)
	})

	t.Run("defaults an empty scope to service_item", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", pipelinesRoot+"/latest/service_item/389/",
			httpmock.NewStringResponder(200, onePipeline))

		nc := newPackTestClient(t)
		_, err := nc.GetLatestPackPipeline(context.Background(), "", "", 389)

		require.NoError(t, err)
	})
}

func TestListPackPipelineVersions(t *testing.T) {
	t.Run("decodes the bare array this route returns", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// Unlike its sibling list route, versions answers with a bare array rather than
		// a paginated envelope.
		httpmock.RegisterResponder("GET", pipelinesRoot+"/versions/service_item/389/",
			httpmock.NewStringResponder(200, `[{"id":2935,"version":4},{"id":2801,"version":3}]`))

		nc := newPackTestClient(t)
		versions, err := nc.ListPackPipelineVersions(
			context.Background(), client.POVServiceOwner, client.PackScopeServiceItem, 389,
		)

		require.NoError(t, err)
		require.Len(t, versions, 2)
		assert.Equal(t, 4, versions[0].Version)
		assert.Equal(t, 2801, versions[1].ID)
	})
}

func TestSetPackPipelineApplied(t *testing.T) {
	t.Run("patches only the applied flag", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("PATCH", pipelinesRoot+"/2935/",
			captureBodyResponder(&capturedBody, 200, `{"id":2935,"version":4,"state":"OK","applied":true}`))

		nc := newPackTestClient(t)
		pipeline, err := nc.SetPackPipelineApplied(context.Background(), client.POVServiceOwner, 2935, true)

		require.NoError(t, err)
		assert.True(t, pipeline.Applied)
		// Applied is the only field the platform accepts from clients, so the body must
		// carry nothing else - anything more would be rejected.
		assert.JSONEq(t, `{"applied":true}`, capturedBody)
	})

	t.Run("can un-apply a run", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("PATCH", pipelinesRoot+"/2935/",
			captureBodyResponder(&capturedBody, 200, `{"id":2935,"applied":false}`))

		nc := newPackTestClient(t)
		_, err := nc.SetPackPipelineApplied(context.Background(), client.POVServiceOwner, 2935, false)

		require.NoError(t, err)
		assert.JSONEq(t, `{"applied":false}`, capturedBody)
	})
}
