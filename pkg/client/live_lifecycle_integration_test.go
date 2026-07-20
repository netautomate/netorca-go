//go:build integration

// Live coverage for the methods the read-only suite cannot reach: the change instance state
// machine, the pack executor write path, and the remaining CRUD corners.
//
// Everything here writes, so every test is gated behind NETORCA_SMOKE_WRITE=1. The triggers,
// which bill real LLM runs, need NETORCA_SMOKE_TRIGGER=1 on top of that.
//
//	./scripts/smoke.sh --write            # everything except the billable triggers
//	./scripts/smoke.sh --write --triggers # everything, including ~3 LLM runs
//
// The change instance tests raise their own disposable changes rather than touching existing
// ones: they submit a throwaway declaration as a consumer, drive the resulting changes through
// the state machine, then withdraw the declaration and settle the DELETE changes it raises.
package client_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/netautomate/netorca-go/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// smokeAppName is the application the lifecycle tests declare under. It is namespaced so a
// failed run leaves something obviously disposable rather than something that looks real.
const smokeAppName = "netorca_go_smoke_app"

// smokeService is the service the lifecycle tests raise changes against. It needs
// approval_required so changes start PENDING, and manual approval/completion so the whole
// state machine is reachable from the API.
const smokeService = "a_record"

// triggersEnabled reports whether the billable trigger tests were opted into.
func triggersEnabled() bool { return os.Getenv("NETORCA_SMOKE_TRIGGER") == "1" }

// requireTriggers skips unless the caller has opted in to spending money.
func requireTriggers(t *testing.T) {
	t.Helper()
	requireWrites(t)
	if !triggersEnabled() {
		t.Skip("set NETORCA_SMOKE_TRIGGER=1 to run the tests that fire real (billed) LLM runs")
	}
}

// submitDeclaration PATCHes a consumer declaration and returns the change instances it raised.
//
// It goes through net/http rather than the SDK because consumer submissions are not part of the
// SDK's surface yet. PATCH is deliberate and load-bearing: a submission is declarative, and a
// POST would replace the team's entire declared state, raising DELETE changes for every service
// item this test did not mention. PATCH merges instead.
func submitDeclaration(t *testing.T, body map[string]any) []client.ChangeInstance {
	t.Helper()

	url := os.Getenv("NETORCA_API_URL")
	key := os.Getenv("NETORCA_API_KEY")
	encoded, err := json.Marshal(body)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx, "PATCH", url+"/v1/orcabase/consumer/submissions/submit/", bytes.NewReader(encoded),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Api-Key "+key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, []int{200, 201}, resp.StatusCode, "submission failed: %s", string(raw))

	var payload struct {
		ChangeInstances []client.ChangeInstance `json:"change_instances"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	return payload.ChangeInstances
}

// smokeDeclaration builds the submission envelope for the given a_record items. Passing none
// withdraws the application, which raises a DELETE change for every item previously declared.
func smokeDeclaration(records []map[string]any) map[string]any {
	services := map[string]any{}
	if len(records) > 0 {
		services[smokeService] = records
	}
	return map[string]any{
		"networks": map[string]any{
			"metadata": map[string]any{"team_name": "networks"},
			smokeAppName: map[string]any{
				"metadata": map[string]any{"owner": "netorca-go smoke", "environment": "test"},
				"services": services,
			},
		},
	}
}

// smokeRecord builds one a_record declaration.
func smokeRecord(name, address string) map[string]any {
	return map[string]any{
		"name":      name,
		"zone":      "example.local.",
		"addresses": []string{address},
	}
}

// settleChanges drives every change to a terminal state so nothing is left pending on the
// instance. Used for cleanup, where the goal is "leave nothing waiting" rather than any
// particular outcome.
func settleChanges(t *testing.T, nc *client.Client, changes []client.ChangeInstance) {
	t.Helper()
	for _, change := range changes {
		current, err := nc.GetChangeInstance(context.Background(), client.POVServiceOwner, change.ID)
		if err != nil {
			t.Logf("cleanup: could not read change %d: %v", change.ID, err)
			continue
		}
		switch current.State {
		case "PENDING":
			if _, err := nc.ApproveChangeInstance(change.ID, "smoke cleanup", nil); err != nil {
				t.Logf("cleanup: approve %d: %v", change.ID, err)
				continue
			}
			fallthrough
		case "APPROVED":
			if _, err := nc.CompleteChangeInstance(change.ID, "smoke cleanup", nil); err != nil {
				t.Logf("cleanup: complete %d: %v", change.ID, err)
			}
		}
	}
}

// TestLiveChangeInstanceLifecycle walks the whole change instance state machine against changes
// it raises itself, covering every transition helper the SDK exposes.
//
//nolint:funlen // one continuous lifecycle; splitting it would separate the setup from its cleanup
func TestLiveChangeInstanceLifecycle(t *testing.T) {
	requireWrites(t)
	nc := liveClient(t)

	// Raise two changes: one to walk the happy path, one to walk the rejection path.
	raised := submitDeclaration(t, smokeDeclaration([]map[string]any{
		smokeRecord("netorca-go-smoke-a", "10.1.1.251"),
		smokeRecord("netorca-go-smoke-b", "10.1.1.252"),
	}))
	if len(raised) < 2 {
		t.Skipf("expected 2 changes, got %d - a previous run may have left the declaration in place", len(raised))
	}
	t.Logf("raised changes %d and %d", raised[0].ID, raised[1].ID)

	// Always withdraw the declaration, whatever happens above.
	t.Cleanup(func() {
		removals := submitDeclaration(t, smokeDeclaration(nil))
		t.Logf("cleanup: withdrawal raised %d change(s)", len(removals))
		settleChanges(t, nc, removals)
	})

	happy, unhappy := raised[0].ID, raised[1].ID

	t.Run("approve then complete", func(t *testing.T) {
		approved, err := nc.ApproveChangeInstance(happy, "smoke: approved", nil)
		require.NoError(t, err)
		assert.Equal(t, string(client.ChangeInstanceAPPROVED), approved.State)

		deployed := json.RawMessage(`{"source":"netorca-go smoke","record":"netorca-go-smoke-a"}`)
		completed, err := nc.CompleteChangeInstance(happy, "smoke: deployed", deployed)
		require.NoError(t, err)
		assert.Equal(t, string(client.ChangeInstanceCOMPLETED), completed.State)
	})

	t.Run("reject, reopen, approve, error, close", func(t *testing.T) {
		rejected, err := nc.RejectChangeInstance(unhappy, "smoke: rejected on purpose", nil)
		require.NoError(t, err)
		assert.Equal(t, string(client.ChangeInstanceREJECTED), rejected.State)

		// REJECTED is not terminal - a consumer can have the decision revisited.
		reopened, err := nc.PendingChangeInstance(unhappy, "smoke: reopened", nil)
		require.NoError(t, err)
		assert.Equal(t, string(client.ChangeInstancePENDING), reopened.State)

		approved, err := nc.ApproveChangeInstance(unhappy, "smoke: approved after review", nil)
		require.NoError(t, err)
		assert.Equal(t, string(client.ChangeInstanceAPPROVED), approved.State)

		// A deployment that failed is reported honestly rather than completed.
		errored, err := nc.SetErrorChangeInstance(unhappy, "smoke: simulated deployment failure", nil)
		require.NoError(t, err)
		assert.Equal(t, string(client.ChangeInstanceERROR), errored.State)

		closed, err := nc.CloseChangeInstance(unhappy, "smoke: closed after failure", nil)
		require.NoError(t, err)
		assert.Equal(t, string(client.ChangeInstanceCLOSED), closed.State)
	})

	t.Run("the generic transition reaches the same route", func(t *testing.T) {
		// happy is COMPLETED by now. Asking for COMPLETED again is accepted as a no-op -
		// the platform only rejects a move to a *different* state that is not reachable.
		same, err := nc.UpdateChangeInstanceState(
			context.Background(), client.POVServiceOwner, happy,
			client.ChangeInstanceCOMPLETED, "smoke: repeat is a no-op", nil,
		)
		require.NoError(t, err)
		assert.Equal(t, string(client.ChangeInstanceCOMPLETED), same.State)
	})

	t.Run("an illegal transition is rejected by the platform", func(t *testing.T) {
		// COMPLETED is terminal for practical purposes: it can only be left for CLOSED.
		// The server answers 400 with a readable reason rather than silently accepting it,
		// so the error carries something a practitioner can act on.
		_, err := nc.UpdateChangeInstanceState(
			context.Background(), client.POVServiceOwner, happy,
			client.ChangeInstancePENDING, "smoke: illegal move", nil,
		)
		require.Error(t, err)
		require.ErrorIs(t, err, client.ErrBadRequest)
		assert.ErrorContains(t, err, "cannot switch from state COMPLETED to PENDING")
	})
}

// TestLiveExecutorLoop covers the pack write path - reporting a stage result into a run and
// acknowledging it. This is the executor half of the pack loop, and the headline capability of
// this release, so it is worth proving against a real instance rather than a mock.
//
// It deliberately targets a run that is already applied: nothing is waiting on it, so pushing a
// stage record and toggling its flag disturbs no live queue.
//
//nolint:funlen // one executor cycle; splitting it would separate the push from the acknowledgement
func TestLiveExecutorLoop(t *testing.T) {
	requireWrites(t)
	nc := liveClient(t)

	applied := true
	done, err := nc.ListPackPipelines(context.Background(), &client.ListPackPipelinesRequest{
		State:   []string{string(client.PackPipelineOK)},
		Applied: &applied,
		Limit:   10,
	})
	require.NoError(t, err)

	var target *client.PackPipeline
	for i := range done.Results {
		if done.Results[i].Config != nil && done.Results[i].Config.ObjectID != 0 {
			target = &done.Results[i]
			break
		}
	}
	if target == nil {
		t.Skip("no already-applied pipeline with config stage data to test against")
	}

	scope := target.Config.ScopeKind()
	objectID := target.Config.ObjectID
	t.Logf("using applied pipeline %d (%s %d)", target.ID, scope, objectID)

	t.Run("push an execution result into the run", func(t *testing.T) {
		pushed, err := nc.PushPackData(
			context.Background(), client.POVServiceOwner, scope, objectID, client.PackActionExecution,
			map[string]any{
				"success":  true,
				"executor": "netorca-go smoke",
				"note":     "written by the integration suite; safe to ignore",
			},
		)
		require.NoError(t, err)
		assert.NotZero(t, pushed.ID)
		assert.Equal(t, string(client.PackActionExecution), pushed.ActionType)
		t.Logf("pushed execution stage data id=%d", pushed.ID)

		// The push must be readable back as the stage's current data.
		read, err := nc.GetPackData(
			context.Background(), client.POVServiceOwner, scope, objectID, client.PackActionExecution,
		)
		require.NoError(t, err)
		assert.Equal(t, pushed.ID, read.ID, "the stage did not return the record just pushed")
	})

	t.Run("acknowledge and restore the applied flag", func(t *testing.T) {
		// Put it back however this ends, so the queue is left exactly as found.
		t.Cleanup(func() {
			if _, err := nc.SetPackPipelineApplied(
				context.Background(), client.POVServiceOwner, target.ID, target.Applied,
			); err != nil {
				t.Logf("cleanup: could not restore applied=%t on pipeline %d: %v",
					target.Applied, target.ID, err)
			}
		})

		unapplied, err := nc.SetPackPipelineApplied(
			context.Background(), client.POVServiceOwner, target.ID, false,
		)
		require.NoError(t, err)
		assert.False(t, unapplied.Applied, "the run should be back on the queue")

		reapplied, err := nc.SetPackPipelineApplied(
			context.Background(), client.POVServiceOwner, target.ID, true,
		)
		require.NoError(t, err)
		assert.True(t, reapplied.Applied, "the run should be off the queue again")
	})
}

// TestLivePackTriggers fires real AI processor runs. Each one costs money, so it needs a
// separate opt-in on top of the write flag.
func TestLivePackTriggers(t *testing.T) {
	requireTriggers(t)
	nc := liveClient(t)

	serviceID := discoverPackService(t, nc)
	items, err := nc.GetServiceItemsWithContext(context.Background(), &client.GetServiceItemsRequest{
		ServiceID: fmt.Sprint(serviceID),
		Limit:     1,
	})
	require.NoError(t, err)
	if len(items.Results) == 0 {
		t.Skipf("service %d has no service items to trigger against", serviceID)
	}
	itemID := items.Results[0].ID
	t.Logf("triggering against service item %d (service %d) - this bills real LLM runs", itemID, serviceID)

	t.Run("trigger a single stage", func(t *testing.T) {
		msg, err := nc.TriggerPack(
			context.Background(), client.POVServiceOwner,
			client.PackScopeServiceItem, itemID, client.PackActionConfig,
		)
		require.NoError(t, err)
		assert.NotEmpty(t, msg)
		t.Logf("trigger: %s", msg)
	})

	t.Run("retrigger with feedback", func(t *testing.T) {
		msg, err := nc.RetriggerPackScoped(
			context.Background(), client.POVServiceOwner, client.PackScopeServiceItem, itemID,
			"netorca-go smoke: no change needed, this is an integration test",
		)
		require.NoError(t, err)
		assert.NotEmpty(t, msg)
		t.Logf("retrigger (scoped): %s", msg)
	})

	t.Run("the compatibility wrapper reaches the same route", func(t *testing.T) {
		msg, err := nc.RetriggerPack(itemID, "netorca-go smoke: wrapper check")
		require.NoError(t, err)
		assert.NotEmpty(t, msg)
		t.Logf("retrigger (wrapper): %s", msg)
	})
}

// TestLiveDeployedItemUpdateDelete covers the two deployed item methods the read suite cannot:
// updating the data of an existing record and removing one.
func TestLiveDeployedItemUpdateDelete(t *testing.T) {
	requireWrites(t)
	nc := liveClient(t)

	items, err := nc.ListDeployedItems(context.Background(), &client.ListDeployedItemsRequest{Limit: 1})
	require.NoError(t, err)
	if len(items.Results) == 0 {
		t.Skip("no deployed items to derive a service item from")
	}
	serviceItemID, err := items.Results[0].ServiceItemID()
	require.NoError(t, err)

	created, err := nc.CreateDeployedItem(context.Background(), client.POVServiceOwner,
		&client.DeployedItemWrite{
			ServiceItemID: serviceItemID,
			Data:          json.RawMessage(`{"source":"netorca-go smoke","phase":"created"}`),
		})
	require.NoError(t, err)
	t.Logf("created deployed item %d v%d", created.ID, created.Version)

	t.Run("update rewrites only the data", func(t *testing.T) {
		updated, err := nc.UpdateDeployedItem(context.Background(), client.POVServiceOwner, created.ID,
			json.RawMessage(`{"source":"netorca-go smoke","phase":"updated"}`))
		require.NoError(t, err)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(updated.Data, &payload))
		assert.Equal(t, "updated", payload["phase"])
	})

	t.Run("delete removes it", func(t *testing.T) {
		require.NoError(t, nc.DeleteDeployedItem(context.Background(), client.POVServiceOwner, created.ID))

		gone, err := nc.GetDeployedItem(context.Background(), client.POVServiceOwner, created.ID)
		require.Error(t, err)
		assert.Nil(t, gone)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}

// TestLivePackProfileWriteVariants covers the profile write entry points the main suite does
// not: the direct upsert, and the create wrapper that delegates to it.
func TestLivePackProfileWriteVariants(t *testing.T) {
	requireWrites(t)
	nc := liveClient(t)

	serviceID := discoverPackService(t, nc)
	before, err := nc.FindPackProfile(context.Background(), client.POVServiceOwner, serviceID)
	require.NoError(t, err)

	originalTopK := before.TopK
	t.Cleanup(func() {
		restore := map[string]any{"top_k": nil}
		if originalTopK != nil {
			restore["top_k"] = *originalTopK
		}
		if _, err := nc.UpsertPackProfileForService(
			context.Background(), client.POVServiceOwner, serviceID, restore,
		); err != nil {
			t.Logf("cleanup: could not restore top_k on service %d: %v", serviceID, err)
		}
	})

	t.Run("upsert applies a field directly", func(t *testing.T) {
		updated, err := nc.UpsertPackProfileForService(
			context.Background(), client.POVServiceOwner, serviceID, map[string]any{"top_k": 9},
		)
		require.NoError(t, err)
		require.NotNil(t, updated.TopK)
		assert.Equal(t, 9, *updated.TopK)
	})

	t.Run("create on an existing profile behaves as an upsert", func(t *testing.T) {
		// A service has at most one profile, so "create" against one that exists must update
		// rather than fail - that is what makes the call safe to re-run.
		topK := 11
		updated, err := nc.CreatePackProfile(context.Background(), client.POVServiceOwner,
			&client.PackProfileWrite{Service: client.RefID(serviceID), TopK: &topK})
		require.NoError(t, err)
		require.NotNil(t, updated.TopK)
		assert.Equal(t, 11, *updated.TopK)
		assert.Equal(t, before.ID, updated.ID, "the upsert created a second profile")
	})

	t.Run("deleting a profile that does not exist is ErrNotFound", func(t *testing.T) {
		// The happy path cannot be exercised: DELETE on a real profile trips a platform bug
		// that returns 500. A missing id short-circuits before that check, so this at least
		// proves the route and the error mapping.
		err := nc.DeletePackProfile(context.Background(), client.POVServiceOwner, missingID)
		require.Error(t, err)
		assert.ErrorIs(t, err, client.ErrNotFound)
	})
}

// TestLiveRemainingReads covers the read helpers the main suite does not reach directly.
func TestLiveRemainingReads(t *testing.T) {
	nc := liveClient(t)

	t.Run("first active LLM model", func(t *testing.T) {
		model, err := nc.FirstActiveLLMModel(context.Background())
		require.NoError(t, err)
		assert.True(t, model.IsActive)
		t.Logf("first active model: %d %q", model.ID, model.Name)
	})

	t.Run("the context-free change instance listing", func(t *testing.T) {
		list, err := nc.GetChangeInstances(&client.GetChangeInstancesRequest{Limit: 1})
		require.NoError(t, err)
		assert.NotNil(t, list)
		t.Logf("change instances: %d", list.Count)
	})

	t.Run("the three stage getter wrappers", func(t *testing.T) {
		// These fix POV and scope, so they are a different code path from GetPackData even
		// though they delegate to it.
		pipelines, err := nc.ListPackPipelines(context.Background(),
			&client.ListPackPipelinesRequest{Limit: 20})
		require.NoError(t, err)

		var itemID int
		for i := range pipelines.Results {
			if cfg := pipelines.Results[i].Config; cfg != nil &&
				cfg.ScopeKind() == client.PackScopeServiceItem && cfg.ObjectID != 0 {
				itemID = cfg.ObjectID
				break
			}
		}
		if itemID == 0 {
			t.Skip("no service-item-scoped pipeline to read stages from")
		}

		for name, get := range map[string]func(int) (*client.PackData, error){
			"config":    nc.GetPackConfig,
			"verify":    nc.GetPackVerify,
			"execution": nc.GetPackExecution,
		} {
			data, err := get(itemID)
			if err != nil {
				// A stage with no data yet is normal, not a failure.
				require.ErrorIs(t, err, client.ErrPackDataNotFound, "stage %s", name)
				continue
			}
			assert.Equal(t, name, data.ActionType)
		}
	})
}
