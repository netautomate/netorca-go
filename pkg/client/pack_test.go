package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/netautomate/netorca-go/config"
	"github.com/netautomate/netorca-go/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const packTestBaseURL = "http://api-aws.demo.netorca.io"

// newPackTestClient builds a client pointed at the mocked base URL used across the pack tests.
func newPackTestClient(t *testing.T) *client.Client {
	t.Helper()
	cfg := config.Config{
		BaseURL:    packTestBaseURL,
		APIKey:     "test-api-key",
		APIVersion: "v1",
	}
	nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
	require.NoError(t, err)
	return nc
}

// captureBodyResponder records the request body into captured and replies with the given response.
func captureBodyResponder(captured *string, status int, respBody string) httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		*captured = string(b)
		return httpmock.NewStringResponse(status, respBody), nil
	}
}

func TestClientGetPackData(t *testing.T) { //nolint:funlen
	t.Run("GetPackConfig returns config PackData on success", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET",
			"http://api-aws.demo.netorca.io/v1/external/serviceowner/pack/data/service_item/42/config/",
			httpmock.NewStringResponder(200, readTestFile(t, "200_pack_config_response.json")),
		)

		nc := newPackTestClient(t)
		pd, err := nc.GetPackConfig(42)
		require.NoError(t, err)
		require.NotNil(t, pd)
		assert.Equal(t, "config", pd.ActionType)
		assert.Equal(t, 501, pd.ID)
		assert.JSONEq(t, `{"policy_name":"app42-waf","rules":[{"name":"block-sqli","action":"block"}]}`, string(pd.Data))
	})

	t.Run("GetPackVerify returns verify PackData with usable Data", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET",
			"http://api-aws.demo.netorca.io/v1/external/serviceowner/pack/data/service_item/42/verify/",
			httpmock.NewStringResponder(200, readTestFile(t, "200_pack_verify_response.json")),
		)

		nc := newPackTestClient(t)
		pd, err := nc.GetPackVerify(42)
		require.NoError(t, err)
		require.NotNil(t, pd)
		assert.Equal(t, "verify", pd.ActionType)

		// Mirror the real loop: decode Data and act on the verify result.
		var verify struct {
			Approved bool   `json:"approved"`
			Summary  string `json:"summary"`
		}
		require.NoError(t, json.Unmarshal(pd.Data, &verify))
		assert.True(t, verify.Approved)
	})

	t.Run("GetPackExecution returns execution PackData on success", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET",
			"http://api-aws.demo.netorca.io/v1/external/serviceowner/pack/data/service_item/42/execution/",
			httpmock.NewStringResponder(200, readTestFile(t, "200_pack_execution_response.json")),
		)

		nc := newPackTestClient(t)
		pd, err := nc.GetPackExecution(42)
		require.NoError(t, err)
		require.NotNil(t, pd)
		assert.Equal(t, "execution", pd.ActionType)
		assert.JSONEq(t, `{"status":"deployed","target":"f5-bigip-01"}`, string(pd.Data))
	})

	t.Run("GetPackConfig returns ErrPackDataNotFound on 404", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET",
			"http://api-aws.demo.netorca.io/v1/external/serviceowner/pack/data/service_item/42/config/",
			httpmock.NewStringResponder(404, `{"detail":"scope not found"}`),
		)

		nc := newPackTestClient(t)
		pd, err := nc.GetPackConfig(42)
		require.Error(t, err)
		assert.Nil(t, pd)
		require.ErrorIs(t, err, client.ErrPackDataNotFound)
	})

	t.Run("GetPackConfig returns error on 500", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET",
			"http://api-aws.demo.netorca.io/v1/external/serviceowner/pack/data/service_item/42/config/",
			httpmock.NewStringResponder(500, `{"error":"Internal Server Error"}`),
		)

		nc := newPackTestClient(t)
		pd, err := nc.GetPackConfig(42)
		require.Error(t, err)
		assert.Nil(t, pd)
		assert.Contains(t, err.Error(), "500 Internal Server Error")
	})
}

func TestClientRetriggerPack(t *testing.T) {
	const retriggerURL = "http://api-aws.demo.netorca.io/v1/external/serviceowner/pack/retrigger/service_item/42/"
	const okResponse = `"AI Processor has been retriggered"`

	t.Run("RetriggerPack sends comment and returns message on success", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST", retriggerURL, captureBodyResponder(&capturedBody, 200, okResponse))

		nc := newPackTestClient(t)
		msg, err := nc.RetriggerPack(42, "fix the sqli rule")
		require.NoError(t, err)
		assert.Equal(t, "AI Processor has been retriggered", msg)
		assert.JSONEq(t, `{"serviceowner_comment":"fix the sqli rule"}`, capturedBody)
	})

	t.Run("RetriggerPack with empty comment sends empty object", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST", retriggerURL, captureBodyResponder(&capturedBody, 200, okResponse))

		nc := newPackTestClient(t)
		msg, err := nc.RetriggerPack(42, "")
		require.NoError(t, err)
		assert.Equal(t, "AI Processor has been retriggered", msg)
		assert.JSONEq(t, `{}`, capturedBody)
	})

	t.Run("RetriggerPack returns error on non-200", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("POST", retriggerURL,
			httpmock.NewStringResponder(400, `{"error":"Bad Request"}`),
		)

		nc := newPackTestClient(t)
		msg, err := nc.RetriggerPack(42, "whatever")
		require.Error(t, err)
		assert.Empty(t, msg)
		assert.Contains(t, err.Error(), "400 Bad Request")
	})
}

const packDataRoot = packTestBaseURL + "/v1/external/serviceowner/pack/data"

// onePackDataRecord is a config stage payload as the API returns it, scope envelope and all.
const onePackDataRecord = `{
  "id": 6600,
  "created": "2026-07-19T10:11:12Z",
  "modified": "2026-07-19T10:11:12Z",
  "action_type": "config",
  "object_id": 389,
  "data": {"as3_json": {"id": "vs_demo"}},
  "scope": {"scope": "service_item", "data": {"id": 389, "name": "demo"}}
}`

func TestGetPackDataByID(t *testing.T) {
	t.Run("returns one record by its own id", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", packDataRoot+"/6600/",
			httpmock.NewStringResponder(200, onePackDataRecord))

		nc := newPackTestClient(t)
		data, err := nc.GetPackDataByID(context.Background(), client.POVServiceOwner, 6600)

		require.NoError(t, err)
		assert.Equal(t, 6600, data.ID)
		assert.Equal(t, 389, data.ObjectID)
		assert.Equal(t, client.PackScopeServiceItem, data.ScopeKind())
	})

	t.Run("surfaces an unknown id as ErrNotFound rather than ErrPackDataNotFound", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", packDataRoot+"/999999/",
			httpmock.NewStringResponder(404, `{"detail":"Not found."}`))

		nc := newPackTestClient(t)
		data, err := nc.GetPackDataByID(context.Background(), client.POVServiceOwner, 999999)

		require.Error(t, err)
		assert.Nil(t, data)
		// A bad id is a bad reference. Only the scoped getter treats a 404 as "the stage
		// has not run yet", which is a normal phase rather than a mistake.
		require.ErrorIs(t, err, client.ErrNotFound)
		require.NotErrorIs(t, err, client.ErrPackDataNotFound)
	})
}

func TestListPackData(t *testing.T) {
	t.Run("sends only the pagination parameters the route honours", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET", packDataRoot+"/",
			func(req *http.Request) (*http.Response, error) {
				capturedQuery = req.URL.RawQuery
				body := `{"count":1,"next":null,"previous":null,"results":[` + onePackDataRecord + `]}`
				return httpmock.NewStringResponse(200, body), nil
			})

		nc := newPackTestClient(t)
		resp, err := nc.ListPackData(context.Background(), &client.ListPackDataRequest{
			Limit:    5,
			Offset:   10,
			Ordering: "-created",
		})

		require.NoError(t, err)
		assert.Equal(t, "limit=5&offset=10&ordering=-created", capturedQuery)
		assert.Equal(t, 1, resp.Count)
		require.Len(t, resp.Results, 1)
		assert.Equal(t, "config", resp.Results[0].ActionType)
	})

	t.Run("sends no query string at all for the zero request", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET", packDataRoot+"/",
			func(req *http.Request) (*http.Response, error) {
				capturedQuery = req.URL.RawQuery
				return httpmock.NewStringResponse(200, emptyPage), nil
			})

		nc := newPackTestClient(t)
		_, err := nc.ListPackData(context.Background(), nil)

		require.NoError(t, err)
		assert.Empty(t, capturedQuery)
	})

	t.Run("honours a consumer POV", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", packTestBaseURL+"/v1/external/consumer/pack/data/",
			httpmock.NewStringResponder(200, emptyPage))

		nc := newPackTestClient(t)
		_, err := nc.ListPackData(context.Background(), &client.ListPackDataRequest{POV: client.POVConsumer})

		require.NoError(t, err)
	})
}
