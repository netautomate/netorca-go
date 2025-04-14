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



## Configuration

Configure the client with the following options:

- `BaseURL`: API endpoint URL
- `APIKey`: Authentication key 
- `APIVersion`: API version
- `RequestTimeout`: request timeout duration