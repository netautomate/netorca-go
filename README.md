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

## Configuration

Configure the client with the following options:

- `BaseURL`: API endpoint URL
- `APIKey`: Authentication key 
- `APIVersion`: API version
- `RequestTimeout`: request timeout duration