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

// llmModelsRoot is the catalogue route. Note the absence of a point-of-view segment - these are
// the only endpoints in the API without one, and every path assertion below is really guarding
// that a POV never creeps back in.
const llmModelsRoot = packTestBaseURL + "/v1/ai/llm_models"

// oneLLMModel is a realistic catalogue entry: priced, active, and carrying the provider
// connection details the platform partially masks (api_key) and partially does not (base_url).
const oneLLMModel = `{
  "id": 8,
  "name": "Claude 4.6",
  "provider": "anthropic",
  "model_name": "claude-4-6",
  "prompt": "Refer to serviceowner prompt first",
  "extra_data": {"api_key": "*****", "base_url": "https://api.anthropic.com"},
  "timeout": 60,
  "input_price_per_1m_tokens": 10.0,
  "output_price_per_1m_tokens": 14.0,
  "metadata": {"tags": ["fast"]},
  "is_active": true,
  "is_deleted": false,
  "deleted_at": null
}`

// captureQueryResponder records the request's query string and replies with the given body.
func captureQueryResponder(captured *string, status int, respBody string) httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		*captured = req.URL.RawQuery
		return httpmock.NewStringResponse(status, respBody), nil
	}
}

// llmPage wraps results in the paginated envelope, with next set when more pages follow.
func llmPage(next string, results string) string {
	encodedNext := "null"
	if next != "" {
		encodedNext = `"` + next + `"`
	}
	return `{"count":2,"next":` + encodedNext + `,"previous":null,"results":[` + results + `]}`
}

func TestLLMModelRedacted(t *testing.T) {
	t.Run("replaces every value and preserves every key", func(t *testing.T) {
		model := client.LLMModel{
			ID:   3,
			Name: "Gateway",
			ExtraData: map[string]any{
				"api_key":    "sk-live-provider-credential",
				"base_url":   "https://gateway.internal.example",
				"verify_ssl": false,
			},
		}

		redacted := model.Redacted()

		// Keys stay so a caller can still assert a model's configuration shape; values go
		// wholesale, including the ones the platform never masked server-side.
		assert.Equal(t, map[string]any{
			"api_key":    "REDACTED",
			"base_url":   "REDACTED",
			"verify_ssl": "REDACTED",
		}, redacted.ExtraData)

		// Redaction must not cost the caller the rest of the record.
		assert.Equal(t, 3, redacted.ID)
		assert.Equal(t, "Gateway", redacted.Name)
	})

	t.Run("does not mutate the receiver", func(t *testing.T) {
		const secret = "sk-live-provider-credential"
		model := client.LLMModel{ExtraData: map[string]any{"api_key": secret}}

		redacted := model.Redacted()

		// The value receiver copies the map header but not the map, so an implementation
		// that wrote in place would destroy the caller's own credentials through the
		// shared backing store.
		assert.Equal(t, secret, model.ExtraData["api_key"])

		// The copy owns its map, so neither side can leak into the other afterwards.
		redacted.ExtraData["api_key"] = "changed"
		assert.Equal(t, secret, model.ExtraData["api_key"])
		model.ExtraData["api_key"] = "changed again"
		assert.Equal(t, "changed", redacted.ExtraData["api_key"])
	})

	t.Run("handles a model with no extra data", func(t *testing.T) {
		assert.Nil(t, client.LLMModel{}.Redacted().ExtraData)
		assert.Empty(t, client.LLMModel{ExtraData: map[string]any{}}.Redacted().ExtraData)
	})
}

func TestClientListLLMModels(t *testing.T) {
	t.Run("lists the catalogue from a path with no POV segment", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", llmModelsRoot+"/",
			httpmock.NewStringResponder(200, llmPage("", oneLLMModel)))

		nc := newPackTestClient(t)
		resp, err := nc.ListLLMModels(context.Background(), nil)

		require.NoError(t, err)
		require.Len(t, resp.Results, 1)
		assert.Nil(t, resp.Next)

		model := resp.Results[0]
		assert.Equal(t, 8, model.ID)
		assert.Equal(t, "Claude 4.6", model.Name)
		assert.Equal(t, client.LLMProviderAnthropic, model.Provider)
		assert.Equal(t, "claude-4-6", model.ModelName)
		assert.Equal(t, "Refer to serviceowner prompt first", model.Prompt)
		assert.Equal(t, 60, model.Timeout)
		assert.True(t, model.IsActive)
		assert.False(t, model.IsDeleted)
		assert.Nil(t, model.DeletedAt)
		assert.Equal(t, "*****", model.ExtraData["api_key"])
		assert.Equal(t, []any{"fast"}, model.Metadata["tags"])

		require.NotNil(t, model.InputPricePer1MTokens)
		assert.InDelta(t, 10.0, *model.InputPricePer1MTokens, 0.0001)
		require.NotNil(t, model.OutputPricePer1MTokens)
		assert.InDelta(t, 14.0, *model.OutputPricePer1MTokens, 0.0001)
	})

	t.Run("distinguishes an unpriced model from a free one", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", llmModelsRoot+"/",
			httpmock.NewStringResponder(200, llmPage("", `{
			  "id": 9,
			  "name": "Unpriced",
			  "input_price_per_1m_tokens": null,
			  "output_price_per_1m_tokens": 0.0
			}`)))

		nc := newPackTestClient(t)
		resp, err := nc.ListLLMModels(context.Background(), nil)

		require.NoError(t, err)
		require.Len(t, resp.Results, 1)
		// The platform leaves prices null until an administrator sets them. Decoding that
		// onto 0 would report a costed-at-nothing model as a genuinely free one.
		assert.Nil(t, resp.Results[0].InputPricePer1MTokens)
		require.NotNil(t, resp.Results[0].OutputPricePer1MTokens)
		assert.InDelta(t, 0.0, *resp.Results[0].OutputPricePer1MTokens, 0.0001)
	})

	t.Run("sends only the pagination parameters the route accepts", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET", llmModelsRoot+"/",
			captureQueryResponder(&capturedQuery, 200, llmPage("", "")))

		nc := newPackTestClient(t)
		_, err := nc.ListLLMModels(context.Background(), &client.ListLLMModelsRequest{
			Limit:    50,
			Offset:   100,
			Ordering: "-id",
		})

		require.NoError(t, err)
		assert.Contains(t, capturedQuery, "limit=50")
		assert.Contains(t, capturedQuery, "offset=100")
		assert.Contains(t, capturedQuery, "ordering=-id")
	})

	t.Run("returns the API error on failure", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", llmModelsRoot+"/",
			httpmock.NewStringResponder(403, `{"detail":"You do not have permission."}`))

		nc := newPackTestClient(t)
		resp, err := nc.ListLLMModels(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.ErrorIs(t, err, client.ErrForbidden)
	})
}

func TestClientGetLLMModel(t *testing.T) {
	t.Run("fetches one model by id", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", llmModelsRoot+"/8/",
			httpmock.NewStringResponder(200, oneLLMModel))

		nc := newPackTestClient(t)
		model, err := nc.GetLLMModel(context.Background(), 8)

		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, 8, model.ID)
		assert.Equal(t, client.LLMProviderAnthropic, model.Provider)
	})

	t.Run("decodes an unrecognised provider verbatim", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", llmModelsRoot+"/11/",
			httpmock.NewStringResponder(200, `{"id":11,"provider":"some_new_provider"}`))

		nc := newPackTestClient(t)
		model, err := nc.GetLLMModel(context.Background(), 11)

		// A platform that has gained a provider since this library was built must still
		// round-trip rather than failing to decode.
		require.NoError(t, err)
		assert.Equal(t, client.LLMProvider("some_new_provider"), model.Provider)
	})

	t.Run("returns ErrNotFound on 404", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", llmModelsRoot+"/404/",
			httpmock.NewStringResponder(404, `{"detail":"Not found."}`))

		nc := newPackTestClient(t)
		model, err := nc.GetLLMModel(context.Background(), 404)

		require.Error(t, err)
		assert.Nil(t, model)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}

func TestClientFindLLMModelByName(t *testing.T) {
	t.Run("returns the model whose name matches exactly", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", llmModelsRoot+"/",
			httpmock.NewStringResponder(200, llmPage("",
				`{"id":4,"name":"Claude 4.6 Preview"},`+oneLLMModel)))

		nc := newPackTestClient(t)
		model, err := nc.FindLLMModelByName(context.Background(), "Claude 4.6")

		require.NoError(t, err)
		require.NotNil(t, model)
		// The similarly named entry sorts first; an exact match must not settle for it.
		assert.Equal(t, 8, model.ID)
	})

	t.Run("orders the scan by id so paging is stable", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET", llmModelsRoot+"/",
			captureQueryResponder(&capturedQuery, 200, llmPage("", oneLLMModel)))

		nc := newPackTestClient(t)
		_, err := nc.FindLLMModelByName(context.Background(), "Claude 4.6")

		require.NoError(t, err)
		assert.Contains(t, capturedQuery, "ordering=id")
		assert.Contains(t, capturedQuery, "limit=100")
	})

	t.Run("scans past the first page", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var secondPageQuery string
		httpmock.RegisterResponder("GET", llmModelsRoot+"/",
			func(req *http.Request) (*http.Response, error) {
				if req.URL.Query().Get("offset") == "" {
					body := llmPage(llmModelsRoot+"/?limit=100&offset=1",
						`{"id":4,"name":"Something Else"}`)
					return httpmock.NewStringResponse(200, body), nil
				}
				secondPageQuery = req.URL.RawQuery
				return httpmock.NewStringResponse(200, llmPage("", oneLLMModel)), nil
			})

		nc := newPackTestClient(t)
		model, err := nc.FindLLMModelByName(context.Background(), "Claude 4.6")

		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, 8, model.ID)
		// The offset advances by what actually arrived, not by the requested limit, so a
		// short page cannot make the next request skip records.
		assert.Contains(t, secondPageQuery, "offset=1")
	})

	t.Run("returns ErrNotFound when no model carries the name", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", llmModelsRoot+"/",
			httpmock.NewStringResponder(200, llmPage("", oneLLMModel)))

		nc := newPackTestClient(t)
		model, err := nc.FindLLMModelByName(context.Background(), "GPT-5")

		require.Error(t, err)
		assert.Nil(t, model)
		assert.ErrorIs(t, err, client.ErrNotFound)
		assert.Contains(t, err.Error(), `"GPT-5"`)
	})

	t.Run("propagates a listing failure rather than reporting a miss", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", llmModelsRoot+"/",
			httpmock.NewStringResponder(401, `{"detail":"Invalid API key."}`))

		nc := newPackTestClient(t)
		model, err := nc.FindLLMModelByName(context.Background(), "Claude 4.6")

		require.Error(t, err)
		assert.Nil(t, model)
		// A broken key must not look like an absent model to a caller reconciling state.
		assert.ErrorIs(t, err, client.ErrUnauthorized)
		assert.NotErrorIs(t, err, client.ErrNotFound)
	})
}

func TestClientFirstActiveLLMModel(t *testing.T) {
	t.Run("skips disabled models", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", llmModelsRoot+"/",
			httpmock.NewStringResponder(200, llmPage("",
				`{"id":2,"name":"Retired","is_active":false},`+oneLLMModel)))

		nc := newPackTestClient(t)
		model, err := nc.FirstActiveLLMModel(context.Background())

		require.NoError(t, err)
		require.NotNil(t, model)
		// A disabled model stays in the catalogue and stays referenceable, so the fallback
		// has to look past it rather than take the lowest id outright.
		assert.Equal(t, 8, model.ID)
		assert.True(t, model.IsActive)
	})

	t.Run("returns ErrNotFound when the catalogue has no active model", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", llmModelsRoot+"/",
			httpmock.NewStringResponder(200, llmPage("", `{"id":2,"is_active":false}`)))

		nc := newPackTestClient(t)
		model, err := nc.FirstActiveLLMModel(context.Background())

		require.Error(t, err)
		assert.Nil(t, model)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})

	t.Run("returns ErrNotFound when the catalogue is empty", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", llmModelsRoot+"/",
			httpmock.NewStringResponder(200, `{"count":0,"next":null,"previous":null,"results":[]}`))

		nc := newPackTestClient(t)
		model, err := nc.FirstActiveLLMModel(context.Background())

		require.Error(t, err)
		assert.Nil(t, model)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}
