package client_test

import (
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
		assert.ErrorIs(t, err, client.ErrPackDataNotFound)
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
