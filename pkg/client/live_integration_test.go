//go:build integration

// Live integration tests for the endpoints added in v0.1.2: pack pipelines, the pack write path,
// AI processors, pack profiles, LLM models and deployed items. They talk to a real NetOrca
// instance and are excluded from a normal `go test ./...` run by the `integration` build tag.
//
//	go test -tags=integration ./pkg/client/ -run TestLive -v
//
// or, loading credentials from .env:
//
//	./scripts/smoke.sh
//
// Required env: NETORCA_API_URL, NETORCA_API_KEY.
// Optional env:
//
//	NETORCA_SMOKE_SERVICE_ID  - pin a service instead of auto-discovering one with a pack profile
//	NETORCA_SMOKE_WRITE=1     - opt in to the create/update/delete tests, which write for real
//
// Everything except the write tests is read-only and safe against a shared instance. Nothing here
// triggers an AI processor, so no test in this file bills an LLM run - see pack_integration_test.go
// for the (separately gated) retrigger test.
package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/netautomate/netorca-go/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// missingID is an id chosen to be far beyond anything a real instance has allocated, so the
// not-found paths can be exercised without depending on a particular object being absent.
const missingID = 999999999

// liveCtx returns a context for a live call. Each request is bounded by the client's own timeout.
func liveCtx() context.Context { return context.Background() }

// writesEnabled reports whether the mutating tests were opted into.
func writesEnabled() bool { return os.Getenv("NETORCA_SMOKE_WRITE") == "1" }

// requireWrites skips a test unless writes were explicitly opted into.
func requireWrites(t *testing.T) {
	t.Helper()
	if !writesEnabled() {
		t.Skip("set NETORCA_SMOKE_WRITE=1 to run the tests that write to the instance")
	}
}

// discoverPackService finds a service that has a pack profile, so the processor and profile tests
// act on something real. NETORCA_SMOKE_SERVICE_ID overrides the search.
func discoverPackService(t *testing.T, nc *client.Client) int {
	t.Helper()

	if pinned := os.Getenv("NETORCA_SMOKE_SERVICE_ID"); pinned != "" {
		id, err := strconv.Atoi(pinned)
		require.NoError(t, err, "NETORCA_SMOKE_SERVICE_ID must be an integer")
		return id
	}

	profiles, err := nc.ListPackProfiles(liveCtx(), &client.ListPackProfilesRequest{Limit: 50})
	require.NoError(t, err)
	if len(profiles.Results) == 0 {
		t.Skip("no pack profiles on this instance; set NETORCA_SMOKE_SERVICE_ID to pin a service")
	}

	// Prefer a service whose pack is actually switched on - that is the realistic case.
	for _, profile := range profiles.Results {
		if profile.PackEnabled != nil && *profile.PackEnabled {
			return profile.Service.Int()
		}
	}
	return profiles.Results[0].Service.Int()
}

func TestLiveLLMModels(t *testing.T) {
	nc := liveClient(t)

	catalogue, err := nc.ListLLMModels(liveCtx(), nil)
	require.NoError(t, err)
	require.NotEmpty(t, catalogue.Results, "the instance has no LLM models configured")
	t.Logf("catalogue: %d model(s)", catalogue.Count)

	first := catalogue.Results[0]
	assert.NotZero(t, first.ID)
	assert.NotEmpty(t, first.Name)

	t.Run("get by id round-trips", func(t *testing.T) {
		model, err := nc.GetLLMModel(liveCtx(), first.ID)
		require.NoError(t, err)
		assert.Equal(t, first.ID, model.ID)
		assert.Equal(t, first.Name, model.Name)
	})

	t.Run("find by name", func(t *testing.T) {
		model, err := nc.FindLLMModelByName(liveCtx(), first.Name)
		require.NoError(t, err)
		assert.Equal(t, first.ID, model.ID)
	})

	t.Run("redaction masks values but keeps keys", func(t *testing.T) {
		// Find a model that actually carries credentials, or there is nothing to prove.
		var withSecrets *client.LLMModel
		for i := range catalogue.Results {
			if len(catalogue.Results[i].ExtraData) > 0 {
				withSecrets = &catalogue.Results[i]
				break
			}
		}
		if withSecrets == nil {
			t.Skip("no model on this instance exposes extra_data")
		}

		// Snapshot the real values first, so mutation of the receiver is detectable.
		before := make(map[string]any, len(withSecrets.ExtraData))
		for key, value := range withSecrets.ExtraData {
			before[key] = value
		}

		redacted := withSecrets.Redacted()
		assert.Len(t, redacted.ExtraData, len(before), "keys must survive redaction")
		for key, value := range redacted.ExtraData {
			assert.Equal(t, "REDACTED", value, "extra_data[%s] was not redacted", key)
		}

		// Redaction returns a copy: mutating the receiver would destroy the caller's
		// credentials as a side effect of asking for a safe view of them.
		assert.Equal(t, before, withSecrets.ExtraData, "Redacted() mutated its receiver")
	})

	t.Run("a missing model is ErrNotFound", func(t *testing.T) {
		model, err := nc.GetLLMModel(liveCtx(), missingID)
		require.Error(t, err)
		assert.Nil(t, model)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}

func TestLiveAIProcessors(t *testing.T) {
	nc := liveClient(t)

	processors, err := nc.ListAIProcessors(liveCtx(), &client.ListAIProcessorsRequest{Limit: 50})
	require.NoError(t, err)
	t.Logf("processors: %d", processors.Count)
	if len(processors.Results) == 0 {
		t.Skip("no AI processors configured on this instance")
	}

	first := processors.Results[0]
	t.Logf("first: id=%d name=%q action=%s service=%d model=%d",
		first.ID, first.Name, first.ActionType, first.Service.Int(), first.LLMModel.Int())

	t.Run("nested relations flatten to ids", func(t *testing.T) {
		// The API returns service and llm_model as objects; RefID must have absorbed that.
		assert.NotZero(t, first.Service.Int(), "service did not decode to an id")
		assert.NotZero(t, first.LLMModel.Int(), "llm_model did not decode to an id")
	})

	t.Run("get by id round-trips", func(t *testing.T) {
		processor, err := nc.GetAIProcessor(liveCtx(), client.POVServiceOwner, first.ID)
		require.NoError(t, err)
		assert.Equal(t, first.ID, processor.ID)
		assert.Equal(t, first.Name, processor.Name)
	})

	t.Run("find by the (service, action_type) identity pair", func(t *testing.T) {
		processor, err := nc.FindAIProcessor(
			liveCtx(), client.POVServiceOwner, first.Service.Int(), first.ActionType,
		)
		require.NoError(t, err)
		assert.Equal(t, first.ID, processor.ID)
	})

	t.Run("history", func(t *testing.T) {
		history, err := nc.ListAIProcessorHistory(liveCtx(), client.POVServiceOwner, first.ID)
		require.NoError(t, err)
		t.Logf("history: %d revision(s)", len(history))
	})

	t.Run("a missing processor is ErrNotFound", func(t *testing.T) {
		processor, err := nc.GetAIProcessor(liveCtx(), client.POVServiceOwner, missingID)
		require.Error(t, err)
		assert.Nil(t, processor)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}

func TestLivePackProfiles(t *testing.T) {
	nc := liveClient(t)

	profiles, err := nc.ListPackProfiles(liveCtx(), &client.ListPackProfilesRequest{Limit: 50})
	require.NoError(t, err)
	t.Logf("pack profiles: %d", profiles.Count)
	if len(profiles.Results) == 0 {
		t.Skip("no pack profiles on this instance")
	}

	first := profiles.Results[0]
	enabled := first.PackEnabled != nil && *first.PackEnabled
	t.Logf("first: id=%d service=%d pack_enabled=%t", first.ID, first.Service.Int(), enabled)

	t.Run("get by id round-trips", func(t *testing.T) {
		profile, err := nc.GetPackProfile(liveCtx(), client.POVServiceOwner, first.ID)
		require.NoError(t, err)
		assert.Equal(t, first.ID, profile.ID)
		assert.Equal(t, first.Service.Int(), profile.Service.Int())
	})

	t.Run("find by service", func(t *testing.T) {
		profile, err := nc.FindPackProfile(liveCtx(), client.POVServiceOwner, first.Service.Int())
		require.NoError(t, err)
		assert.Equal(t, first.ID, profile.ID)
	})

	t.Run("unset tunables decode as nil, not zero", func(t *testing.T) {
		// This is the whole reason the tunables are pointers: a nil TopK means "platform
		// default", and collapsing it onto 0 would look like a deliberate setting.
		profile, err := nc.GetPackProfile(liveCtx(), client.POVServiceOwner, first.ID)
		require.NoError(t, err)
		if profile.TopK != nil {
			assert.NotEqual(t, 0, *profile.TopK, "a set top_k of 0 would be indistinguishable from unset")
		}
	})
}

//nolint:funlen // one coherent walk through the pipeline surface; splitting it would duplicate discovery
func TestLivePackPipelines(t *testing.T) {
	nc := liveClient(t)

	pipelines, err := nc.ListPackPipelines(liveCtx(), &client.ListPackPipelinesRequest{Limit: 20})
	require.NoError(t, err)
	t.Logf("pipelines: %d", pipelines.Count)
	if len(pipelines.Results) == 0 {
		t.Skip("no pack pipelines on this instance")
	}

	first := pipelines.Results[0]
	t.Logf("first: id=%d v%d state=%s applied=%t cost=%s",
		first.ID, first.Version, first.State, first.Applied, first.Cost)

	t.Run("cost parses", func(t *testing.T) {
		cost, err := first.ParseCost()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, cost, 0.0)
	})

	t.Run("the executor work queue is state OK plus applied false", func(t *testing.T) {
		applied := false
		queue, err := nc.ListPackPipelines(liveCtx(), &client.ListPackPipelinesRequest{
			State:   []string{string(client.PackPipelineOK)},
			Applied: &applied,
			Limit:   20,
		})
		require.NoError(t, err)
		t.Logf("work queue: %d run(s) waiting", queue.Count)

		// The filter must actually be applied - an unfiltered response would silently look
		// like a full queue, which is exactly the bug the *bool exists to prevent.
		for _, run := range queue.Results {
			assert.False(t, run.Applied, "run %d is applied but came back in the unapplied queue", run.ID)
			assert.Equal(t, string(client.PackPipelineOK), run.State)
		}
	})

	t.Run("get by id round-trips", func(t *testing.T) {
		pipeline, err := nc.GetPackPipeline(liveCtx(), client.POVServiceOwner, first.ID)
		require.NoError(t, err)
		assert.Equal(t, first.ID, pipeline.ID)
	})

	// Find a run with config stage data so the scoped lookups have a real object to use.
	var scoped *client.PackPipeline
	for i := range pipelines.Results {
		if pipelines.Results[i].Config != nil && pipelines.Results[i].Config.ObjectID != 0 {
			scoped = &pipelines.Results[i]
			break
		}
	}
	if scoped == nil {
		t.Skip("no pipeline on this instance carries config stage data")
	}

	scope := scoped.Config.ScopeKind()
	objectID := scoped.Config.ObjectID
	t.Logf("scoped lookups against %s %d", scope, objectID)

	t.Run("scope is read back off the record, not assumed", func(t *testing.T) {
		assert.NotEmpty(t, string(scope), "scope envelope did not decode")
	})

	t.Run("latest", func(t *testing.T) {
		latest, err := nc.GetLatestPackPipeline(liveCtx(), client.POVServiceOwner, scope, objectID)
		require.NoError(t, err)
		t.Logf("latest run for %s %d: id=%d v%d state=%s", scope, objectID, latest.ID, latest.Version, latest.State)
		assert.NotZero(t, latest.ID)
	})

	t.Run("versions returns a bare array", func(t *testing.T) {
		versions, err := nc.ListPackPipelineVersions(liveCtx(), client.POVServiceOwner, scope, objectID)
		require.NoError(t, err)
		require.NotEmpty(t, versions, "an object with a run must have at least one version")
		t.Logf("versions: %d", len(versions))
		for _, version := range versions {
			assert.NotZero(t, version.ID)
		}
	})

	t.Run("pack data for each stage", func(t *testing.T) {
		for _, stage := range []client.PackActionType{
			client.PackActionConfig, client.PackActionVerify, client.PackActionExecution,
		} {
			data, err := nc.GetPackData(liveCtx(), client.POVServiceOwner, scope, objectID, stage)
			switch {
			case errors.Is(err, client.ErrPackDataNotFound):
				// Normal: the stage has not produced data. Must also satisfy ErrNotFound.
				require.ErrorIs(t, err, client.ErrNotFound)
				t.Logf("stage %s: no data yet", stage)
			case err != nil:
				t.Errorf("stage %s: unexpected error: %v", stage, err)
			default:
				t.Logf("stage %s: id=%d, %d bytes of data", stage, data.ID, len(data.Data))
				assert.Equal(t, string(stage), data.ActionType)
			}
		}
	})

	t.Run("pack data rejects a non-pipeline stage client-side", func(t *testing.T) {
		data, err := nc.GetPackData(
			liveCtx(), client.POVServiceOwner, scope, objectID, client.PackActionOptimiser,
		)
		require.Error(t, err)
		assert.Nil(t, data)
		assert.ErrorContains(t, err, "config, verify and execution")
	})

	t.Run("a missing pipeline is ErrNotFound", func(t *testing.T) {
		pipeline, err := nc.GetPackPipeline(liveCtx(), client.POVServiceOwner, missingID)
		require.Error(t, err)
		assert.Nil(t, pipeline)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}

func TestLiveDeployedItems(t *testing.T) {
	nc := liveClient(t)

	items, err := nc.ListDeployedItems(liveCtx(), &client.ListDeployedItemsRequest{Limit: 20})
	require.NoError(t, err)
	t.Logf("deployed items: %d", items.Count)
	if len(items.Results) == 0 {
		t.Skip("no deployed items on this instance")
	}

	first := items.Results[0]
	serviceItemID, err := first.ServiceItemID()
	require.NoError(t, err, "the service item hyperlink did not parse back to an id")
	t.Logf("first: id=%d v%d service_item=%d", first.ID, first.Version, serviceItemID)

	t.Run("get by id round-trips", func(t *testing.T) {
		item, err := nc.GetDeployedItem(liveCtx(), client.POVServiceOwner, first.ID)
		require.NoError(t, err)
		assert.Equal(t, first.ID, item.ID)
	})

	t.Run("find for a service item returns the highest version", func(t *testing.T) {
		item, err := nc.FindDeployedItemForServiceItem(liveCtx(), client.POVServiceOwner, serviceItemID)
		require.NoError(t, err)

		// Every other record for the same service item must be an older version.
		all, err := nc.ListDeployedItems(liveCtx(), &client.ListDeployedItemsRequest{
			ServiceItemID: []int{serviceItemID},
			Limit:         50,
		})
		require.NoError(t, err)
		for _, candidate := range all.Results {
			assert.LessOrEqual(t, candidate.Version, item.Version,
				"find returned v%d but v%d exists", item.Version, candidate.Version)
		}
	})

	t.Run("a missing deployed item is ErrNotFound", func(t *testing.T) {
		item, err := nc.GetDeployedItem(liveCtx(), client.POVServiceOwner, missingID)
		require.Error(t, err)
		assert.Nil(t, item)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}

func TestLiveSingleObjectReads(t *testing.T) {
	nc := liveClient(t)

	t.Run("service item", func(t *testing.T) {
		list, err := nc.GetServiceItemsWithContext(liveCtx(), &client.GetServiceItemsRequest{Limit: 1})
		require.NoError(t, err)
		if len(list.Results) == 0 {
			t.Skip("no service items on this instance")
		}

		want := list.Results[0]
		got, err := nc.GetServiceItem(liveCtx(), client.POVServiceOwner, want.ID)
		require.NoError(t, err)
		assert.Equal(t, want.ID, got.ID)
		assert.Equal(t, want.Name, got.Name)

		missing, err := nc.GetServiceItem(liveCtx(), client.POVServiceOwner, missingID)
		require.Error(t, err)
		assert.Nil(t, missing)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})

	t.Run("change instance", func(t *testing.T) {
		list, err := nc.GetChangeInstancesWithContext(liveCtx(), &client.GetChangeInstancesRequest{Limit: 1})
		require.NoError(t, err)
		if len(list.Results) == 0 {
			t.Skip("no change instances on this instance")
		}

		want := list.Results[0]
		got, err := nc.GetChangeInstance(liveCtx(), client.POVServiceOwner, want.ID)
		require.NoError(t, err)
		assert.Equal(t, want.ID, got.ID)
		assert.Equal(t, want.State, got.State)

		missing, err := nc.GetChangeInstance(liveCtx(), client.POVServiceOwner, missingID)
		require.Error(t, err)
		assert.Nil(t, missing)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}

// TestLiveWriteAIProcessor exercises the full CRUD cycle against a real service. It is gated
// behind NETORCA_SMOKE_WRITE because it creates and deletes a processor for real.
//
// It uses the `optimiser` action type deliberately: config and verify are likely already taken on
// a pack-enabled service, and the (service, action_type) pair is unique.
//
//nolint:funlen // one CRUD cycle; splitting it would separate the create from its cleanup
func TestLiveWriteAIProcessor(t *testing.T) {
	requireWrites(t)
	nc := liveClient(t)

	serviceID := discoverPackService(t, nc)
	catalogue, err := nc.ListLLMModels(liveCtx(), nil)
	require.NoError(t, err)
	require.NotEmpty(t, catalogue.Results)
	modelID := catalogue.Results[0].ID

	const actionType = client.PackActionOptimiser

	// Leave nothing behind, even if an assertion fails part way through.
	cleanup := func() {
		existing, err := nc.FindAIProcessor(liveCtx(), client.POVServiceOwner, serviceID, actionType)
		if err != nil {
			return
		}
		if err := nc.DeleteAIProcessor(liveCtx(), client.POVServiceOwner, existing.ID); err != nil {
			t.Logf("cleanup: could not delete processor %d: %v", existing.ID, err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	created, err := nc.CreateAIProcessor(liveCtx(), client.POVServiceOwner, &client.AIProcessorWrite{
		Service:    client.RefID(serviceID),
		ActionType: actionType,
		Name:       "netorca_go_smoke",
		LLMModel:   client.RefID(modelID),
		Prompt:     "Smoke test processor created by the netorca-go integration suite. Safe to delete.",
		// The optimiser type requires both schedule keys: the platform rejects the write with
		// {"extra_data":{"schedule_crontab":["This field is required."]}} when either is absent,
		// even with scheduling switched off.
		ExtraData: map[string]any{
			"schedule_enabled": false,
			"schedule_crontab": "0 3 * * *",
		},
	})
	require.NoError(t, err)
	t.Logf("created processor %d on service %d", created.ID, serviceID)

	assert.Equal(t, "netorca_go_smoke", created.Name)
	assert.Equal(t, serviceID, created.Service.Int())
	assert.Equal(t, modelID, created.LLMModel.Int())

	t.Run("partial patch changes only what it names", func(t *testing.T) {
		updated, err := nc.UpdateAIProcessor(liveCtx(), client.POVServiceOwner, created.ID,
			map[string]any{"prompt": "Updated by the smoke suite."})
		require.NoError(t, err)

		assert.Equal(t, "Updated by the smoke suite.", updated.Prompt)
		// The fields the patch did not name must be untouched. This is the property that
		// makes partial PATCH safe, and the reason Update takes a map rather than a struct.
		assert.Equal(t, created.Name, updated.Name, "an unnamed field changed")
		assert.Equal(t, created.LLMModel.Int(), updated.LLMModel.Int(), "an unnamed field changed")
		assert.Equal(t, created.ActionType, updated.ActionType, "an unnamed field changed")
	})

	t.Run("find locates it by the identity pair", func(t *testing.T) {
		found, err := nc.FindAIProcessor(liveCtx(), client.POVServiceOwner, serviceID, actionType)
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("delete removes it", func(t *testing.T) {
		require.NoError(t, nc.DeleteAIProcessor(liveCtx(), client.POVServiceOwner, created.ID))

		gone, err := nc.GetAIProcessor(liveCtx(), client.POVServiceOwner, created.ID)
		require.Error(t, err)
		assert.Nil(t, gone)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}

// TestLiveWritePackProfile updates a real pack profile and puts it back. It is gated behind
// NETORCA_SMOKE_WRITE because it writes to a shared instance.
//
// It deliberately does not create or delete a profile: deleting one reverts the service to
// platform defaults, and because pack_enabled defaults to true that can switch Pack ON for a
// service someone deliberately turned it off for.
func TestLiveWritePackProfile(t *testing.T) {
	requireWrites(t)
	nc := liveClient(t)

	serviceID := discoverPackService(t, nc)
	profile, err := nc.FindPackProfile(liveCtx(), client.POVServiceOwner, serviceID)
	require.NoError(t, err)
	t.Logf("using profile %d for service %d", profile.ID, serviceID)

	// Remember the value so it can be restored exactly, including "it was unset".
	originalTopK := profile.TopK
	t.Cleanup(func() {
		restore := map[string]any{"top_k": nil}
		if originalTopK != nil {
			restore["top_k"] = *originalTopK
		}
		if _, err := nc.UpdatePackProfile(liveCtx(), client.POVServiceOwner, profile.ID, restore); err != nil {
			t.Logf("cleanup: could not restore top_k on profile %d: %v", profile.ID, err)
		}
	})

	const probeTopK = 7
	updated, err := nc.UpdatePackProfile(liveCtx(), client.POVServiceOwner, profile.ID,
		map[string]any{"top_k": probeTopK})
	require.NoError(t, err)
	require.NotNil(t, updated.TopK)
	assert.Equal(t, probeTopK, *updated.TopK)

	// The patch named only top_k, so the master switch must be exactly as it was. If a partial
	// patch were sending the whole object, this is where it would show up.
	assert.Equal(t, fmt.Sprint(profile.PackEnabled != nil && *profile.PackEnabled),
		fmt.Sprint(updated.PackEnabled != nil && *updated.PackEnabled),
		"pack_enabled changed while patching top_k")
}

// TestLiveWriteDeployedItem creates a deployed item against a real service item, then checks the
// platform's byte-for-byte dedupe behaviour. Gated behind NETORCA_SMOKE_WRITE.
//
// Deployed items cannot be meaningfully cleaned up - they are a versioned audit trail - so this
// writes one small record and leaves it.
func TestLiveWriteDeployedItem(t *testing.T) {
	requireWrites(t)
	nc := liveClient(t)

	items, err := nc.ListDeployedItems(liveCtx(), &client.ListDeployedItemsRequest{Limit: 1})
	require.NoError(t, err)
	if len(items.Results) == 0 {
		t.Skip("no deployed items to derive a service item from")
	}
	serviceItemID, err := items.Results[0].ServiceItemID()
	require.NoError(t, err)

	payload := json.RawMessage(`{"source":"netorca-go smoke","safe_to_delete":true}`)

	created, err := nc.CreateDeployedItem(liveCtx(), client.POVServiceOwner, &client.DeployedItemWrite{
		ServiceItemID: serviceItemID,
		Data:          payload,
	})
	require.NoError(t, err)
	t.Logf("created deployed item %d v%d on service item %d", created.ID, created.Version, serviceItemID)

	t.Run("an identical create is deduped rather than versioned", func(t *testing.T) {
		again, err := nc.CreateDeployedItem(liveCtx(), client.POVServiceOwner, &client.DeployedItemWrite{
			ServiceItemID: serviceItemID,
			Data:          payload,
		})
		require.NoError(t, err)

		// The platform answers 200 with the existing record rather than 201 with a new one.
		// Success is therefore not proof that a new version exists - compare the version.
		assert.Equal(t, created.Version, again.Version,
			"an identical create produced a new version; the dedupe behaviour has changed")
	})

	t.Run("one parent only", func(t *testing.T) {
		_, err := nc.CreateDeployedItem(liveCtx(), client.POVServiceOwner, &client.DeployedItemWrite{
			ServiceItemID:    serviceItemID,
			ChangeInstanceID: 1,
			Data:             payload,
		})
		require.Error(t, err, "a body naming both parents must be rejected client-side")
	})
}
