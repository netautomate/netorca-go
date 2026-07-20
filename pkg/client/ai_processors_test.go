package client_test

import (
	"context"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/netautomate/netorca-go/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	aiProcessorBaseURL    = "http://api-aws.demo.netorca.io"
	aiProcessorsURL       = "http://api-aws.demo.netorca.io/v1/external/serviceowner/ai_processors/"
	aiProcessorDetailURL  = aiProcessorsURL + "12/"
	aiProcessorHistoryURL = aiProcessorDetailURL + "history/"
)

// aiProcessorJSON is one processor in the read shape: nested relation objects for service and
// llm_model, a server-filled extra_data, and a schema.
const aiProcessorJSON = `{
	"id": 12,
	"name": "Render F5 config",
	"service": {"id": 49, "name": "VIRTUAL_SERVER"},
	"llm_model": {"id": 3, "name": "gpt-4o", "prompt": "You are a network engineer."},
	"action_type": "config",
	"prompt": "Render the declaration into an AS3 payload.",
	"extra_data": {"include_change_instance": true, "enable_pack_context": false},
	"response_schema": {"type": "object", "properties": {"as3": {"type": "object"}}},
	"active": true
}`

const aiProcessorListJSON = `{"count":1,"next":null,"previous":null,"results":[` + aiProcessorJSON + `]}`

// newAIProcessorTestClient builds a client pointed at the mocked base URL used across these tests.
func newAIProcessorTestClient(t *testing.T) *client.Client {
	t.Helper()
	nc, err := client.NewClient(aiProcessorBaseURL, "test-api-key", "v1", 5*time.Second)
	require.NoError(t, err)
	return nc
}

func TestListAIProcessorsRequestToQueryParams(t *testing.T) {
	t.Run("zero value produces no query string", func(t *testing.T) {
		query, err := (&client.ListAIProcessorsRequest{}).ToQueryParams()
		require.NoError(t, err)
		assert.Empty(t, query)
	})

	t.Run("every filter is rendered under the name the API expects", func(t *testing.T) {
		inactive := false
		filters := client.ListAIProcessorsRequest{
			ServiceID:  []int{49, 50},
			LLMModelID: []int{7},
			ActionType: []client.PackActionType{client.PackActionConfig, client.PackActionVerify},
			Active:     &inactive,
			Ordering:   "-id",
			Limit:      5,
			Offset:     10,
		}

		query, err := filters.ToQueryParams()
		require.NoError(t, err)
		assert.Equal(t,
			"?action_type=config%2Cverify&active=false&limit=5&llm_model_id=7"+
				"&offset=10&ordering=-id&service_id=49%2C50",
			query,
		)
	})

	t.Run("active is tri-state so false survives", func(t *testing.T) {
		active := true
		query, err := (&client.ListAIProcessorsRequest{Active: &active}).ToQueryParams()
		require.NoError(t, err)
		assert.Equal(t, "?active=true", query)
	})
}

func TestClientListAIProcessors(t *testing.T) {
	t.Run("returns processors with relations normalised to ids", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", aiProcessorsURL,
			httpmock.NewStringResponder(200, aiProcessorListJSON),
		)

		nc := newAIProcessorTestClient(t)
		resp, err := nc.ListAIProcessors(context.Background(), nil)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, 1, resp.Count)
		assert.Nil(t, resp.Next)
		require.Len(t, resp.Results, 1)

		processor := resp.Results[0]
		assert.Equal(t, 12, processor.ID)
		assert.Equal(t, "Render F5 config", processor.Name)
		// The read shape sends objects; RefID must reduce both to their ids.
		assert.Equal(t, 49, processor.Service.Int())
		assert.Equal(t, 3, processor.LLMModel.Int())
		assert.Equal(t, client.PackActionConfig, processor.ActionType)
		assert.True(t, processor.Active)
		assert.Equal(t, true, processor.ExtraData["include_change_instance"])
		assert.JSONEq(t,
			`{"type":"object","properties":{"as3":{"type":"object"}}}`,
			string(processor.ResponseSchema),
		)
	})

	t.Run("filters are sent as query parameters", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		// Registering the URL complete with its query means a client that builds a
		// different one finds no responder and fails, rather than passing silently.
		httpmock.RegisterResponder("GET", aiProcessorsURL+"?action_type=config&active=true&service_id=49",
			httpmock.NewStringResponder(200, aiProcessorListJSON),
		)

		active := true
		nc := newAIProcessorTestClient(t)
		resp, err := nc.ListAIProcessors(context.Background(), &client.ListAIProcessorsRequest{
			POV:        client.POVServiceOwner,
			ServiceID:  []int{49},
			ActionType: []client.PackActionType{client.PackActionConfig},
			Active:     &active,
		})
		require.NoError(t, err)
		require.Len(t, resp.Results, 1)
	})

	t.Run("consumer POV is honoured", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET",
			"http://api-aws.demo.netorca.io/v1/external/consumer/ai_processors/",
			httpmock.NewStringResponder(200, aiProcessorListJSON),
		)

		nc := newAIProcessorTestClient(t)
		resp, err := nc.ListAIProcessors(context.Background(), &client.ListAIProcessorsRequest{
			POV: client.POVConsumer,
		})
		require.NoError(t, err)
		require.Len(t, resp.Results, 1)
	})

	t.Run("returns error on 500", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", aiProcessorsURL,
			httpmock.NewStringResponder(500, `{"error":"Internal Server Error"}`),
		)

		nc := newAIProcessorTestClient(t)
		resp, err := nc.ListAIProcessors(context.Background(), nil)
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "500 Internal Server Error")
	})
}

func TestClientGetAIProcessor(t *testing.T) {
	t.Run("returns the processor on success", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", aiProcessorDetailURL,
			httpmock.NewStringResponder(200, aiProcessorJSON),
		)

		nc := newAIProcessorTestClient(t)
		processor, err := nc.GetAIProcessor(context.Background(), client.POVServiceOwner, 12)
		require.NoError(t, err)
		require.NotNil(t, processor)
		assert.Equal(t, 12, processor.ID)
		assert.Equal(t, 49, processor.Service.Int())
		assert.Equal(t, "Render the declaration into an AS3 payload.", processor.Prompt)
	})

	t.Run("a null response_schema decodes to the JSON null literal", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", aiProcessorDetailURL,
			httpmock.NewStringResponder(200, `{"id":12,"service":49,"llm_model":3,"response_schema":null}`),
		)

		nc := newAIProcessorTestClient(t)
		processor, err := nc.GetAIProcessor(context.Background(), client.POVServiceOwner, 12)
		require.NoError(t, err)
		assert.Equal(t, "null", string(processor.ResponseSchema))
		// Some routes answer with bare ids rather than relation objects; RefID takes both.
		assert.Equal(t, 49, processor.Service.Int())
	})

	t.Run("returns ErrNotFound on 404", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", aiProcessorDetailURL,
			httpmock.NewStringResponder(404, `{"detail":"Not found."}`),
		)

		nc := newAIProcessorTestClient(t)
		processor, err := nc.GetAIProcessor(context.Background(), client.POVServiceOwner, 12)
		require.Error(t, err)
		assert.Nil(t, processor)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}

func TestClientCreateAIProcessor(t *testing.T) { //nolint:funlen
	t.Run("sends relations as bare ids and omits undeclared fields", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST", aiProcessorsURL,
			captureBodyResponder(&capturedBody, 201, aiProcessorJSON),
		)

		nc := newAIProcessorTestClient(t)
		processor, err := nc.CreateAIProcessor(context.Background(), client.POVServiceOwner,
			&client.AIProcessorWrite{
				Name:       "Render F5 config",
				Service:    49,
				LLMModel:   3,
				ActionType: client.PackActionConfig,
				Prompt:     "Render the declaration into an AS3 payload.",
			})
		require.NoError(t, err)
		require.NotNil(t, processor)
		assert.Equal(t, 12, processor.ID)

		// Bare ints, not objects - and nothing the caller left undeclared, so the
		// platform's own defaults stand.
		assert.JSONEq(t, `{
			"name": "Render F5 config",
			"service": 49,
			"llm_model": 3,
			"action_type": "config",
			"prompt": "Render the declaration into an AS3 payload."
		}`, capturedBody)
	})

	t.Run("an explicit inactive flag is sent, unlike an omitted one", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST", aiProcessorsURL,
			captureBodyResponder(&capturedBody, 201, aiProcessorJSON),
		)

		inactive := false
		nc := newAIProcessorTestClient(t)
		_, err := nc.CreateAIProcessor(context.Background(), client.POVServiceOwner,
			&client.AIProcessorWrite{
				Name:       "Nightly optimiser",
				Service:    49,
				LLMModel:   3,
				ActionType: client.PackActionOptimiser,
				Prompt:     "Reduce the retrigger count.",
				ExtraData: map[string]any{
					"schedule_enabled": true,
					"schedule_crontab": "0 */6 * * *",
				},
				ResponseSchema: []byte(`{"type":"object"}`),
				Active:         &inactive,
			})
		require.NoError(t, err)

		assert.JSONEq(t, `{
			"name": "Nightly optimiser",
			"service": 49,
			"llm_model": 3,
			"action_type": "optimiser",
			"prompt": "Reduce the retrigger count.",
			"extra_data": {"schedule_enabled": true, "schedule_crontab": "0 */6 * * *"},
			"response_schema": {"type": "object"},
			"active": false
		}`, capturedBody)
	})

	t.Run("returns the platform's validation payload on a duplicate pair", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("POST", aiProcessorsURL,
			httpmock.NewStringResponder(400,
				`{"non_field_errors":["AI Processor with this service and processor type already exists."]}`),
		)

		nc := newAIProcessorTestClient(t)
		processor, err := nc.CreateAIProcessor(context.Background(), client.POVServiceOwner,
			&client.AIProcessorWrite{Name: "Duplicate", Service: 49, ActionType: client.PackActionConfig})
		require.Error(t, err)
		assert.Nil(t, processor)
		assert.ErrorIs(t, err, client.ErrBadRequest)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("rejects a nil body without calling the API", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		// No responder is registered: a request would fail with a different message.

		nc := newAIProcessorTestClient(t)
		processor, err := nc.CreateAIProcessor(context.Background(), client.POVServiceOwner, nil)
		require.Error(t, err)
		assert.Nil(t, processor)
		assert.Equal(t, "AI processor body cannot be nil", err.Error())
	})
}

func TestClientUpdateAIProcessor(t *testing.T) {
	t.Run("PATCHes exactly the keys given, and nothing else", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("PATCH", aiProcessorDetailURL,
			captureBodyResponder(&capturedBody, 200, aiProcessorJSON),
		)

		nc := newAIProcessorTestClient(t)
		processor, err := nc.UpdateAIProcessor(context.Background(), client.POVServiceOwner, 12,
			map[string]any{"prompt": "Render an AS3 payload, minimally."})
		require.NoError(t, err)
		require.NotNil(t, processor)
		assert.Equal(t, 12, processor.ID)

		// Anything else in the body would reset server-side defaults the caller never
		// declared, extra_data above all.
		assert.JSONEq(t, `{"prompt":"Render an AS3 payload, minimally."}`, capturedBody)
	})

	t.Run("rejects an empty patch without calling the API", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		// No responder is registered: a request would fail with a different message.

		nc := newAIProcessorTestClient(t)
		processor, err := nc.UpdateAIProcessor(context.Background(), client.POVServiceOwner, 12, nil)
		require.Error(t, err)
		assert.Nil(t, processor)
		assert.Equal(t, "AI processor patch cannot be empty", err.Error())
	})

	t.Run("returns ErrNotFound on 404", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("PATCH", aiProcessorDetailURL,
			httpmock.NewStringResponder(404, `{"detail":"Not found."}`),
		)

		nc := newAIProcessorTestClient(t)
		_, err := nc.UpdateAIProcessor(context.Background(), client.POVServiceOwner, 12,
			map[string]any{"active": false})
		require.Error(t, err)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}

func TestClientDeleteAIProcessor(t *testing.T) {
	t.Run("succeeds on a 204 with no body", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("DELETE", aiProcessorDetailURL,
			httpmock.NewStringResponder(204, ""),
		)

		nc := newAIProcessorTestClient(t)
		require.NoError(t, nc.DeleteAIProcessor(context.Background(), client.POVServiceOwner, 12))
	})

	t.Run("returns ErrNotFound when the processor has already gone", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("DELETE", aiProcessorDetailURL,
			httpmock.NewStringResponder(404, `{"detail":"Not found."}`),
		)

		nc := newAIProcessorTestClient(t)
		err := nc.DeleteAIProcessor(context.Background(), client.POVServiceOwner, 12)
		require.Error(t, err)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}

func TestClientListAIProcessorHistory(t *testing.T) {
	const historyJSON = `{
		"count": 2,
		"next": null,
		"previous": null,
		"results": [
			{
				"history_id": 88,
				"history_date": "2026-05-01T12:00:00.123456Z",
				"history_user": "alice",
				"history_type": "~",
				"version": 2,
				"id": 12,
				"name": "Render F5 config",
				"service": 49,
				"llm_model": 3,
				"action_type": "config",
				"prompt": "Render an AS3 payload, minimally.",
				"extra_data": {"include_change_instance": true},
				"response_schema": null
			},
			{
				"history_id": 87,
				"history_date": "2026-04-01T09:30:00Z",
				"history_user": null,
				"history_type": "+",
				"version": 1,
				"id": 12,
				"name": "Render F5 config",
				"service": 49,
				"llm_model": 3,
				"action_type": "config",
				"prompt": "Render the declaration into an AS3 payload.",
				"extra_data": {},
				"response_schema": null
			}
		]
	}`

	t.Run("unwraps the paginated envelope, newest first", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", aiProcessorHistoryURL,
			httpmock.NewStringResponder(200, historyJSON),
		)

		nc := newAIProcessorTestClient(t)
		entries, err := nc.ListAIProcessorHistory(context.Background(), client.POVServiceOwner, 12)
		require.NoError(t, err)
		require.Len(t, entries, 2)

		assert.Equal(t, 88, entries[0].HistoryID)
		assert.Equal(t, 2, entries[0].Version)
		assert.Equal(t, "~", entries[0].HistoryType)
		require.NotNil(t, entries[0].HistoryUser)
		assert.Equal(t, "alice", *entries[0].HistoryUser)
		assert.Equal(t, 49, entries[0].Service.Int())
		assert.Equal(t, client.PackActionConfig, entries[0].ActionType)
		assert.Equal(t,
			time.Date(2026, 5, 1, 12, 0, 0, 123456000, time.UTC),
			entries[0].HistoryDate.UTC(),
		)

		// A platform-made revision carries no user, which must stay distinguishable
		// from an empty username.
		assert.Equal(t, "+", entries[1].HistoryType)
		assert.Nil(t, entries[1].HistoryUser)
	})

	t.Run("accepts a bare array from an instance with pagination disabled", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", aiProcessorHistoryURL,
			httpmock.NewStringResponder(200, `[{"history_id":88,"history_type":"+","version":1}]`),
		)

		nc := newAIProcessorTestClient(t)
		entries, err := nc.ListAIProcessorHistory(context.Background(), client.POVServiceOwner, 12)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, 88, entries[0].HistoryID)
	})

	t.Run("returns ErrNotFound on 404", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", aiProcessorHistoryURL,
			httpmock.NewStringResponder(404, `{"detail":"Not found."}`),
		)

		nc := newAIProcessorTestClient(t)
		entries, err := nc.ListAIProcessorHistory(context.Background(), client.POVServiceOwner, 12)
		require.Error(t, err)
		assert.Nil(t, entries)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}

func TestClientFindAIProcessor(t *testing.T) {
	const findURL = aiProcessorsURL + "?action_type=config&service_id=49"

	t.Run("looks the processor up by its identity pair", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", findURL,
			httpmock.NewStringResponder(200, aiProcessorListJSON),
		)

		nc := newAIProcessorTestClient(t)
		processor, err := nc.FindAIProcessor(
			context.Background(), client.POVServiceOwner, 49, client.PackActionConfig,
		)
		require.NoError(t, err)
		require.NotNil(t, processor)
		assert.Equal(t, 12, processor.ID)
		assert.Equal(t, 49, processor.Service.Int())
	})

	t.Run("returns ErrNotFound when the pair has no processor", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", findURL,
			httpmock.NewStringResponder(200, `{"count":0,"next":null,"previous":null,"results":[]}`),
		)

		nc := newAIProcessorTestClient(t)
		processor, err := nc.FindAIProcessor(
			context.Background(), client.POVServiceOwner, 49, client.PackActionConfig,
		)
		require.Error(t, err)
		assert.Nil(t, processor)
		assert.ErrorIs(t, err, client.ErrNotFound)
		assert.Contains(t, err.Error(), `service 49 with action type "config"`)
	})

	t.Run("refuses to guess when the pair somehow matches more than one", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", findURL,
			httpmock.NewStringResponder(200,
				`{"count":2,"next":null,"previous":null,"results":[`+aiProcessorJSON+`,`+aiProcessorJSON+`]}`),
		)

		nc := newAIProcessorTestClient(t)
		processor, err := nc.FindAIProcessor(
			context.Background(), client.POVServiceOwner, 49, client.PackActionConfig,
		)
		require.Error(t, err)
		assert.Nil(t, processor)
		assert.Contains(t, err.Error(), "expected one AI processor")
	})

	t.Run("propagates a transport failure", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", findURL,
			httpmock.NewStringResponder(403, `{"detail":"You do not have permission."}`),
		)

		nc := newAIProcessorTestClient(t)
		processor, err := nc.FindAIProcessor(
			context.Background(), client.POVServiceOwner, 49, client.PackActionConfig,
		)
		require.Error(t, err)
		assert.Nil(t, processor)
		assert.ErrorIs(t, err, client.ErrForbidden)
	})
}
