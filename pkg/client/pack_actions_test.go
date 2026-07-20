package client_test

import (
	"context"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/netautomate/netorca-go/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const packRoot = packTestBaseURL + "/v1/external/serviceowner/pack"

func TestPushPackData(t *testing.T) {
	t.Run("stores the payload verbatim with no envelope", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST", packRoot+"/data/service_item/389/execution/",
			captureBodyResponder(&capturedBody, 200, `{"id":6601,"action_type":"execution","object_id":389}`))

		nc := newPackTestClient(t)
		result, err := nc.PushPackData(
			context.Background(),
			client.POVServiceOwner, client.PackScopeServiceItem, 389, client.PackActionExecution,
			map[string]any{"success": true, "deployed_at": "2026-07-20T09:00:00Z"},
		)

		require.NoError(t, err)
		assert.Equal(t, 6601, result.ID)
		// The platform stores the whole request body as the stage's data field, so the
		// payload must go on the wire exactly as given - wrapping it would corrupt the stage.
		assert.JSONEq(t, `{"success":true,"deployed_at":"2026-07-20T09:00:00Z"}`, capturedBody)
	})

	t.Run("reports failures honestly, which is what feeds the retrigger loop", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST", packRoot+"/data/service_item/389/execution/",
			captureBodyResponder(&capturedBody, 200, `{"id":6602}`))

		nc := newPackTestClient(t)
		_, err := nc.PushPackData(
			context.Background(),
			client.POVServiceOwner, client.PackScopeServiceItem, 389, client.PackActionExecution,
			map[string]any{"success": false, "error": "monitor type rejected by device"},
		)

		require.NoError(t, err)
		assert.JSONEq(t, `{"success":false,"error":"monitor type rejected by device"}`, capturedBody)
	})

	t.Run("rejects a non-pipeline action before making a request", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		nc := newPackTestClient(t)
		result, err := nc.PushPackData(
			context.Background(),
			client.POVServiceOwner, client.PackScopeServiceItem, 389,
			client.PackActionChangeInstanceValidator,
			map[string]any{"success": true},
		)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorContains(t, err, "config, verify and execution")
		// Caught client-side: no call should have been attempted.
		assert.Equal(t, 0, httpmock.GetTotalCallCount())
	})

	t.Run("rejects a nil payload", func(t *testing.T) {
		nc := newPackTestClient(t)
		_, err := nc.PushPackData(
			context.Background(),
			client.POVServiceOwner, client.PackScopeServiceItem, 389, client.PackActionExecution, nil,
		)
		require.Error(t, err)
	})
}

func TestTriggerPack(t *testing.T) {
	t.Run("triggers one stage and returns the confirmation message", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("POST", packRoot+"/trigger/service_item/389/config/",
			httpmock.NewStringResponder(200, `"AI Processor has been triggered"`))

		nc := newPackTestClient(t)
		msg, err := nc.TriggerPack(
			context.Background(),
			client.POVServiceOwner, client.PackScopeServiceItem, 389, client.PackActionConfig,
		)

		require.NoError(t, err)
		assert.Equal(t, "AI Processor has been triggered", msg)
	})

	t.Run("supports the standalone processor action types", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// Trigger accepts all five action types, unlike pack data which takes only the
		// three pipeline stages.
		httpmock.RegisterResponder("POST", packRoot+"/trigger/service/49/change_instance_validator/",
			httpmock.NewStringResponder(200, `"AI Processor has been triggered"`))

		nc := newPackTestClient(t)
		msg, err := nc.TriggerPack(
			context.Background(),
			client.POVServiceOwner, client.PackScopeService, 49,
			client.PackActionChangeInstanceValidator,
		)

		require.NoError(t, err)
		assert.Equal(t, "AI Processor has been triggered", msg)
	})
}

func TestRetriggerPackScoped(t *testing.T) {
	t.Run("sends the serviceowner comment as feedback for the AI", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST", packRoot+"/retrigger/service_item/389/",
			captureBodyResponder(&capturedBody, 200, `"AI Processor has been retriggered"`))

		nc := newPackTestClient(t)
		msg, err := nc.RetriggerPackScoped(
			context.Background(),
			client.POVServiceOwner, client.PackScopeServiceItem, 389,
			"use a tcp monitor, the health endpoint is TCP-only",
		)

		require.NoError(t, err)
		assert.Equal(t, "AI Processor has been retriggered", msg)
		assert.JSONEq(
			t,
			`{"serviceowner_comment":"use a tcp monitor, the health endpoint is TCP-only"}`,
			capturedBody,
		)
	})

	t.Run("omits an empty comment", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST", packRoot+"/retrigger/service_item/389/",
			captureBodyResponder(&capturedBody, 200, `"AI Processor has been retriggered"`))

		nc := newPackTestClient(t)
		_, err := nc.RetriggerPackScoped(
			context.Background(), client.POVServiceOwner, client.PackScopeServiceItem, 389, "",
		)

		require.NoError(t, err)
		assert.JSONEq(t, `{}`, capturedBody)
	})
}

func TestGetPackDataScoped(t *testing.T) {
	t.Run("reads a stage for an explicitly scoped object", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", packRoot+"/data/service/49/config/",
			httpmock.NewStringResponder(200, `{"id":6600,"action_type":"config","object_id":49,
			  "data":{"rendered":true},"scope":{"scope":"service","data":{"id":49}}}`))

		nc := newPackTestClient(t)
		data, err := nc.GetPackData(
			context.Background(),
			client.POVServiceOwner, client.PackScopeService, 49, client.PackActionConfig,
		)

		require.NoError(t, err)
		assert.Equal(t, 6600, data.ID)
		assert.Equal(t, client.PackScopeService, data.ScopeKind())
	})

	t.Run("a stage with no data yet is ErrPackDataNotFound, and also ErrNotFound", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", packRoot+"/data/service_item/389/verify/",
			httpmock.NewStringResponder(404, `{"detail":"scope not found"}`))

		nc := newPackTestClient(t)
		data, err := nc.GetPackData(
			context.Background(),
			client.POVServiceOwner, client.PackScopeServiceItem, 389, client.PackActionVerify,
		)

		require.Error(t, err)
		assert.Nil(t, data)
		// A stage that has not produced data yet is a normal state in a running pipeline,
		// so callers branch on the specific sentinel rather than treating it as a failure.
		assert.ErrorIs(t, err, client.ErrPackDataNotFound)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}
