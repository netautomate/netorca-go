# netorca-go
Netorca Golang SDK is a lightweight library, enabling easy integration with your deployed Netorca instance for seamless service interactions.

## Installation

```bash
go get github.com/netautomate/netorca-go
```

## Quick Start

```go
import (
    "github.com/netautomate/netorca-go/config"
    "github.com/netautomate/netorca-go/pkg/client"
)

// Setup configuration
cfg := config.Config{
    BaseURL:    "http://api.netorca.io",
    APIKey:     "your-api-key",
    APIVersion: "v1",
}

// Create client
nc, err := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.APIVersion, 5*time.Second)
if err != nil {
    // handle error
}

// Use the client
filters := &client.GetServiceItemsRequest{
    POV:           "serviceowner",
    Limit:         10,
    ApplicationID: "your-app-id",
}

serviceItems, err := nc.GetServiceItems(filters)
if err != nil {
    // handle error
}

// Process results
for _, item := range serviceItems.Results {
    fmt.Printf("Service: %s, State: %s\n", item.Name, item.RuntimeState)
}
```

## Features

### Service Items

Retrieve and filter service items:

```go
// Create filters
filters := &client.GetServiceItemsRequest{
    Name:         "api-service",    // Filter by name
    RuntimeState: "IN_SERVICE",     // Filter by state
    Limit:        10,               // Pagination limit
    Offset:       0,                // Pagination offset
}

// Get service items
items, err := nc.GetServiceItems(filters)
```

#### Filtering Options

The client supports various filtering options for service items:

- Name and identifier filters
- State filters (runtime state, change state)
- Declaration filters
- Team and owner filters
- Pagination and ordering





### Change Instances
Retrieve and manage change instances:

```go
// Get change instances with filters
filters := &client.GetChangeInstancesRequest{
    POV:        "serviceowner",
    ChangeType: "CREATE",
    State:      "PENDING",
    ServiceID:  "4",
    Limit:      10,
}

changeInstances, err := nc.GetChangeInstances(filters)
if err != nil {
    // handle error
}

// Process the results
for _, ci := range changeInstances.Results {
    fmt.Printf("Change Instance: %d, Type: %s, State: %s\n", 
        ci.ID, ci.ChangeType, ci.State)
}
```

#### Managing Change Instance States

The client provides methods to update change instance states:

```go
// Approve a change instance
deployedItem := json.RawMessage(`{"deployed_url": "http://deployment1.example.com"}`)
ci, err := nc.ApproveChangeInstance(53, "Reviewed and approved", deployedItem)

// Complete a change instance
ci, err := nc.CompleteChangeInstance(53, "Deployment successful", deployedItem)

// Reject a change instance
ci, err := nc.RejectChangeInstance(53, "Invalid configuration", deployedItem)

// Close a change instance
ci, err := nc.CloseChangeInstance(53, "Closed after review", deployedItem)

// Mark a change instance as error
ci, err := nc.SetErrorChangeInstance(53, "Deployment failed", deployedItem)
```

#### Change Instance States

Change instances can have the following states:
- `PENDING` - Awaiting review
- `APPROVED` - Approved but not yet completed
- `REJECTED` - Rejected during review
- `COMPLETED` - Successfully completed
- `CLOSED` - Closed (typically after completion)
- `ERROR` - Encountered an error  



### AI Pack Loop

Drive the NetOrca AI "pack" pipeline for a service item — read each stage's generated data
(`config` → `verify` → `execution`) and retrigger the pipeline when needed. All pack operations run
from the `serviceowner` point of view.

```go
import (
    "errors"
    "fmt"

    "github.com/netautomate/netorca-go/pkg/client"
)

serviceItemID := 42

// The payload you act on is PackData.Data (raw JSON) - unmarshal it into your own type.
config, err := nc.GetPackConfig(serviceItemID)
if errors.Is(err, client.ErrPackDataNotFound) {
    // The config stage has not produced data yet - a normal state while the pipeline runs.
} else if err != nil {
    // handle error
}

verify, err := nc.GetPackVerify(serviceItemID)
// ... inspect verify.Data (for example an "approved" flag) ...

execution, err := nc.GetPackExecution(serviceItemID)
// ... act on execution.Data ...

// Re-run the pipeline from the config stage, optionally with feedback for the AI processor.
msg, err := nc.RetriggerPack(serviceItemID, "verify rejected: fix the firewall rule")
if err != nil {
    // handle error
}
fmt.Println(msg) // e.g. "AI Processor has been retriggered"
```

#### Methods

- `GetPackConfig(serviceItemID int) (*PackData, error)` — latest `config` stage data
- `GetPackVerify(serviceItemID int) (*PackData, error)` — latest `verify` stage data
- `GetPackExecution(serviceItemID int) (*PackData, error)` — latest `execution` stage data
- `RetriggerPack(serviceItemID int, serviceownerComment string) (string, error)` — re-run the pipeline from `config` (pass `""` for no comment)

The three getters return `ErrPackDataNotFound` (check with `errors.Is`) when a stage has not produced
data yet. Read the generated payload from `PackData.Data` and unmarshal it into your own type.

### Driving the pack pipeline

The getters above watch a run. These drive one. All of them take a `context.Context`, a `POV` and a
`PackScope` (`PackScopeServiceItem` or `PackScopeService`), so both scopes and both points of view are
reachable.

```go
ctx := context.Background()

// Start one processor stage. Trigger accepts all five action types, including
// PackActionOptimiser and PackActionChangeInstanceValidator.
msg, err := nc.TriggerPack(ctx, client.POVServiceOwner, client.PackScopeServiceItem, 389,
    client.PackActionConfig)

// Report a stage result into a run. The body is stored verbatim as the stage's data,
// so pass the object you want the stage to hold.
_, err = nc.PushPackData(ctx, client.POVServiceOwner, client.PackScopeServiceItem, 389,
    client.PackActionExecution, map[string]any{"success": true, "deployed_at": deployedAt})

// Retrigger from config with feedback the AI folds into its prompt.
msg, err = nc.RetriggerPackScoped(ctx, client.POVServiceOwner, client.PackScopeServiceItem, 389,
    "the health endpoint is TCP-only, use a tcp monitor")
```

> Every trigger invokes the service's LLM and costs real money; the spend accumulates on the
> pipeline's `Cost`. Success means the platform accepted the trigger, not that the run succeeded.

### Pack pipelines

A pipeline is one recorded run. Everything on it except `Applied` is produced by the platform.

```go
// The executor work queue: successful runs nobody has acted on yet.
applied := false
queue, err := nc.ListPackPipelines(ctx, &client.ListPackPipelinesRequest{
    State:   []string{string(client.PackPipelineOK)},
    Applied: &applied,
})

// Poll a run to a terminal state after triggering.
run, err := nc.GetLatestPackPipeline(ctx, client.POVServiceOwner, client.PackScopeServiceItem, 389)

// Cheaper change detection: bare {id, version} pairs.
versions, err := nc.ListPackPipelineVersions(ctx, client.POVServiceOwner,
    client.PackScopeServiceItem, 389)

// Take a run off the queue once you have reported its result.
_, err = nc.SetPackPipelineApplied(ctx, client.POVServiceOwner, run.ID, true)
```

`Applied` is a `*bool` on the request deliberately: `applied=false` is a meaningful query, and must
survive the "drop unset filters" pass rather than being mistaken for absent.

### AI processors, pack profiles and the LLM catalogue

The provisioning half - what you set up once per service before any pipeline can run.

```go
// LLM models are platform-global (no POV) and read-only: the platform accepts model
// writes only from superuser GUI sessions, so this SDK deliberately has none.
model, err := nc.FindLLMModelByName(ctx, "Claude 4.6")

// A processor renders (config), checks (verify) or executes. Identity is the
// (service, action_type) pair.
processor, err := nc.CreateAIProcessor(ctx, client.POVServiceOwner, &client.AIProcessorWrite{
    Service:    client.RefID(49),
    ActionType: client.PackActionConfig,
    Name:       "vip_config",
    LLMModel:   client.RefID(model.ID),
    Prompt:     "Render F5 AS3 for the VIRTUAL_SERVER declaration...",
    ExtraData:  map[string]any{"enable_pack_context": true},
})

// Updates take a partial patch - the API applies partial-PATCH semantics, and a
// full write would reset server defaults you never declared.
_, err = nc.UpdateAIProcessor(ctx, client.POVServiceOwner, processor.ID,
    map[string]any{"prompt": revisedPrompt})

// pack_enabled is the per-service master switch. Nothing runs without it.
enabled := true
_, err = nc.CreatePackProfile(ctx, client.POVServiceOwner, &client.PackProfileWrite{
    Service:     client.RefID(49),
    PackEnabled: &enabled,
})
```

### Deployed items

A versioned JSON record of what was actually deployed to satisfy a service item.

```go
item, err := nc.CreateDeployedItem(ctx, client.POVServiceOwner, &client.DeployedItemWrite{
    ServiceItemID: 389,
    Data:          json.RawMessage(`{"vs_name":"vs_demo","vip":"10.1.10.198"}`),
})
```

Two behaviours to know: a create whose data repeats the previous version byte-for-byte returns 200
with the *existing* record and `"no change detected"` rather than a 201, so success is not proof a new
version exists - compare `Version`. And because items are versioned, filtering by service item returns
a history; `FindDeployedItemForServiceItem` returns the highest version.

## Errors

Every non-2xx response becomes an `*APIError` carrying the status code and the server's own
explanation, which for a 400 is the validation payload. It unwraps to a sentinel, so callers branch
without string matching:

```go
item, err := nc.GetServiceItem(ctx, client.POVServiceOwner, 389)
switch {
case errors.Is(err, client.ErrNotFound):
    // the object is gone - drift, not a failure
case errors.Is(err, client.ErrForbidden):
    // the key's team does not own this, or the POV is wrong
case errors.Is(err, client.ErrServerUnavailable):
    // transient, retry
}

var apiErr *client.APIError
if errors.As(err, &apiErr) {
    log.Printf("status %d: %s", apiErr.StatusCode, apiErr.Body)
}
```

Sentinels: `ErrNotFound`, `ErrUnauthorized`, `ErrForbidden`, `ErrBadRequest`, `ErrServerUnavailable`,
plus `ErrPackDataNotFound` (which itself unwraps to `ErrNotFound`).

## Configuration

The base URL may be given with or without the `/v1` suffix - both forms produce the same client.

Two optional fields on `Client` cover the cases `NewClient` does not:

```go
nc.HTTPClient = &http.Client{Transport: myTransport} // custom CA, proxy, instrumentation
nc.Logger = log.Default()                            // one line per request; nil is silent
```

Configure the client with the following options:

- `BaseURL`: API endpoint URL
- `APIKey`: Authentication key 
- `APIVersion`: API version
- `RequestTimeout`: request timeout duration