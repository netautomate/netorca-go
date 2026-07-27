package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DeployedItem is a service owner's record of what they actually built in response to a
// consumer's request. The platform never interprets it: it stores it, versions it and makes
// it searchable, so the service owner can record whatever identifies the real-world object
// they created - a VIP address, a VM id, a rendered as3 declaration.
//
// A deployed item hangs off exactly one parent, a service item or a change instance, and the
// platform normalises the change instance case down to that change instance's own service
// item on write. Its natural key is therefore its parent plus its version rather than its id:
// each accepted write creates a new deployed item one version higher instead of editing the
// last one, and "the deployed item" for a service item means its highest version. Reconcile
// identity with FindDeployedItemForServiceItem, not with a remembered id.
type DeployedItem struct {
	// ID is the unique identifier for this version of the deployed item.
	ID int `json:"id"`
	// URL is the API's own point-of-view-scoped link to this deployed item.
	URL string `json:"url"`
	// Version is its position in the parent service item's history, counting from 1. The
	// platform assigns it on create and refuses to let a client set it.
	Version int `json:"version"`
	// Data is whatever the service owner chose to record about the deployment. It is
	// freeform by design - the platform has no schema for it, so neither does this package -
	// and is left raw for the call site to decode into a type it does have a schema for.
	Data json.RawMessage `json:"data"`
	// ServiceItem is the parent service item as a hyperlink, for instance
	// "https://api.netorca.io/v1/orcabase/serviceowner/service_items/389/", and "" when the
	// API sent null.
	//
	// This is deliberately not a RefID. The orcabase deployed item serializer models both
	// relations with DRF's HyperlinkedRelatedField, so unlike the nested {"id": ...} objects
	// elsewhere in this package they are URL strings coming out and URL strings going in.
	// Use ServiceItemID to get at the id.
	ServiceItem string `json:"service_item"`
	// ChangeInstance is the change instance this item was recorded against, as a hyperlink,
	// and "" when the item was written against a service item directly. It is only ever set
	// by the caller that created the record; the platform copies the change instance's
	// service item into ServiceItem regardless, so ServiceItem is the reliable parent.
	ChangeInstance string `json:"change_instance"`
	// ConsumerTeam is the team that asked for the service, derived by the platform from the
	// parent. Nil when the API could not resolve one.
	ConsumerTeam *Team `json:"consumer_team"`
	// ServiceOwnerTeam is the team that owns the service, derived from the parent in the
	// same way. Nil when the API could not resolve one.
	ServiceOwnerTeam *Team `json:"service_owner_team"`
	// Created is when this version was recorded.
	Created time.Time `json:"created"`
	// Modified is when it was last written to.
	Modified time.Time `json:"modified"`
}

// ServiceItemID returns the id of the parent service item, parsed out of its hyperlink.
// It returns 0 when the deployed item carries no service item link, and an error only when
// the link is not in the shape this package expects.
func (d *DeployedItem) ServiceItemID() (int, error) {
	id, err := deployedItemLinkID(d.ServiceItem)
	if err != nil {
		return 0, fmt.Errorf("failed to read the service item id of deployed item %d: %w", d.ID, err)
	}
	return id, nil
}

// ChangeInstanceID returns the id of the change instance this item was recorded against,
// parsed out of its hyperlink. It returns 0 for the common case of an item written straight
// against a service item, which carries no change instance link at all.
func (d *DeployedItem) ChangeInstanceID() (int, error) {
	id, err := deployedItemLinkID(d.ChangeInstance)
	if err != nil {
		return 0, fmt.Errorf("failed to read the change instance id of deployed item %d: %w", d.ID, err)
	}
	return id, nil
}

// deployedItemLinkID extracts the trailing primary key from a hyperlinked relation, which is
// the only part of it a caller cares about. An empty link yields 0 rather than an error,
// because an absent relation is a normal state and not a malformed one.
func deployedItemLinkID(link string) (int, error) {
	if link == "" {
		return 0, nil
	}

	// Drop any query string or fragment before hunting for the last path segment: DRF
	// appends "?format=json" when a format suffix has been negotiated.
	trimmed, _, _ := strings.Cut(link, "?")
	trimmed, _, _ = strings.Cut(trimmed, "#")
	trimmed = strings.TrimRight(trimmed, "/")

	segment := trimmed
	if index := strings.LastIndex(trimmed, "/"); index >= 0 {
		segment = trimmed[index+1:]
	}

	id, err := strconv.Atoi(segment)
	if err != nil {
		return 0, fmt.Errorf("reference %q does not end in an id: %w", link, err)
	}
	return id, nil
}

// DeployedItemWrite is the body of a create: the freeform data, plus exactly one parent -
// a service item or a change instance, never both and never neither.
//
// The one-parent rule is not house style, it is what keeps the API from misbehaving. Sending
// both is accepted but quietly discards the service item, because the platform overwrites it
// with the change instance's own service item, so the caller gets a record pointing somewhere
// they did not name. Sending neither slips past the server's own guard - its service item
// field carries a default of null, which makes the guard's presence check pass - and then
// fails deep inside validation with nothing useful to read. Validate rejects both shapes up
// front so the caller gets a sentence instead of a surprise.
type DeployedItemWrite struct {
	// Data is what to record about the deployment. Freeform JSON; nil is sent as {}.
	Data json.RawMessage
	// ServiceItemID names the parent service item. Leave it zero when setting
	// ChangeInstanceID.
	ServiceItemID int
	// ChangeInstanceID names the change instance to record against, which is the natural
	// choice while fulfilling one: the platform resolves it to that change instance's
	// service item. Leave it zero when setting ServiceItemID.
	ChangeInstanceID int
}

// Validate reports whether the body satisfies the one-parent rule and carries usable data.
// CreateDeployedItem calls it, so a caller only needs it directly to fail early - at
// Terraform plan time, for instance, rather than at apply time.
func (w *DeployedItemWrite) Validate() error {
	switch {
	case w.ServiceItemID != 0 && w.ChangeInstanceID != 0:
		return fmt.Errorf(
			"a deployed item takes one parent: set ServiceItemID or ChangeInstanceID, not both",
		)
	case w.ServiceItemID == 0 && w.ChangeInstanceID == 0:
		return fmt.Errorf(
			"a deployed item needs a parent: set either ServiceItemID or ChangeInstanceID",
		)
	}

	// Catching this here turns an opaque marshalling failure at request time into a
	// pointed one, and keeps malformed bytes from ever reaching the wire.
	if len(w.Data) > 0 && !json.Valid(w.Data) {
		return fmt.Errorf("deployed item data is not valid JSON")
	}
	return nil
}

// deployedItemBody is the JSON the API accepts for a create or an update. Both relations are
// hyperlinks rather than ids, and omitempty keeps the unused one out of the payload
// altogether - neither relation allows null, so naming one explicitly as null is rejected
// outright rather than read as "leave it alone".
type deployedItemBody struct {
	Data           json.RawMessage `json:"data"`
	ServiceItem    string          `json:"service_item,omitempty"`
	ChangeInstance string          `json:"change_instance,omitempty"`
}

// deployedItemData normalises freeform data for the wire. The API's data field has a model
// default of {} but no serializer default, so omitting it or sending null fails; an empty
// object is the honest way to say "nothing worth recording".
func deployedItemData(data json.RawMessage) json.RawMessage {
	if len(data) == 0 {
		return json.RawMessage("{}")
	}
	return data
}

// deployedItemParentLink renders the hyperlink the API expects for a deployed item's parent.
//
// The relations are hyperlinked, so a bare id is rejected: the server resolves the URL back
// to an object through its own URL configuration, and that configuration includes the point
// of view. The link therefore has to be built from the client's base URL and the pov of the
// request it will travel in, which is why this is a method rather than a package function.
func (c *Client) deployedItemParentLink(pov POV, collection string, id int) string {
	// NewClient guarantees the trailing slash, but a hand-built Client need not have one.
	base := c.BaseURL
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return fmt.Sprintf("%sorcabase/%s/%s/%d/", base, pov.orDefault(), collection, id)
}

// ListDeployedItemsRequest filters a deployed item listing. Every field is optional; the zero
// value lists everything the API key can see, from the serviceowner point of view.
//
// The API rejects an unrecognised filter outright rather than ignoring it, so this struct is
// deliberately limited to the filters the platform actually registers.
type ListDeployedItemsRequest struct {
	// POV is the point of view to query from. Defaults to serviceowner. Both points of view
	// may read deployed items; only a service owner may write them.
	POV POV
	// ApplicationID restricts results to items whose service item belongs to these
	// applications.
	ApplicationID []int
	// ConsumerTeamID restricts results to these consumer teams.
	ConsumerTeamID []int
	// ServiceOwnerTeamID restricts results to these service owner teams.
	ServiceOwnerTeamID []int
	// ServiceID restricts results to these services.
	ServiceID []int
	// ServiceItemID restricts results to these service items. This is the filter that
	// answers "what is deployed for this request", and the one
	// FindDeployedItemForServiceItem is built on.
	ServiceItemID []int
	// Declaration matches data fields exactly. The API names this filter "declaration"
	// even though it searches the data field, for symmetry with the service item and
	// change instance listings; the three below are the same search the rest of the
	// platform offers over consumer declarations.
	Declaration map[string]any
	// DeclarationRegex matches data fields against regular expressions.
	DeclarationRegex map[string]any
	// DeclarationContains matches data fields by containment.
	DeclarationContains map[string]any
	// Limit caps the number of results returned. Zero leaves the server's page size, 20.
	Limit int
	// Offset skips this many results.
	Offset int
	// Ordering names the field to sort by; prefix with "-" to reverse. "-version" is the
	// useful one, since it puts a service item's current deployed item first.
	Ordering string
}

// ToQueryParams renders the filters as a percent-encoded query string, "" when empty.
func (r *ListDeployedItemsRequest) ToQueryParams() (string, error) {
	params := newQueryParams()
	params.SetInts("application_id", r.ApplicationID)
	params.SetInts("consumer_team_id", r.ConsumerTeamID)
	params.SetInts("service_owner_team_id", r.ServiceOwnerTeamID)
	params.SetInts("service_id", r.ServiceID)
	params.SetInts("service_item_id", r.ServiceItemID)
	params.SetInt("limit", r.Limit)
	params.SetInt("offset", r.Offset)
	params.SetString("ordering", r.Ordering)

	for name, value := range map[string]map[string]any{
		"declaration":          r.Declaration,
		"declaration_regex":    r.DeclarationRegex,
		"declaration_contains": r.DeclarationContains,
	} {
		if len(value) == 0 {
			continue
		}
		if err := params.SetJSON(name, value); err != nil {
			return "", err
		}
	}

	return params.Encode(), nil
}

// ListDeployedItemsResponse is the paginated envelope the API returns for a listing.
type ListDeployedItemsResponse struct {
	// Count is the total number of matching items, across all pages.
	Count int `json:"count"`
	// Next is the URL of the next page, nil on the last page.
	Next *string `json:"next"`
	// Previous is the URL of the previous page, nil on the first page.
	Previous *string `json:"previous"`
	// Results is this page of deployed items.
	Results []DeployedItem `json:"results"`
}

// ListDeployedItems returns deployed items matching the given filters.
//
// Remember that a listing spans versions as well as service items: filtering by one service
// item still yields its whole history, newest last unless Ordering says otherwise.
func (c *Client) ListDeployedItems(
	ctx context.Context,
	filters *ListDeployedItemsRequest,
) (*ListDeployedItemsResponse, error) {
	if filters == nil {
		filters = &ListDeployedItemsRequest{}
	}

	query, err := filters.ToQueryParams()
	if err != nil {
		return nil, fmt.Errorf("failed to convert filters to query params: %w", err)
	}

	endpoint := fmt.Sprintf("orcabase/%s/deployed_items/%s", filters.POV.orDefault(), query)

	var response ListDeployedItemsResponse
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetDeployedItem returns a single deployed item by id.
// It returns an error wrapping ErrNotFound when no such item exists, or when it exists but
// belongs to a team the API key cannot see from this point of view.
func (c *Client) GetDeployedItem(ctx context.Context, pov POV, id int) (*DeployedItem, error) {
	endpoint := fmt.Sprintf("orcabase/%s/deployed_items/%d/", pov.orDefault(), id)

	var response DeployedItem
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// FindDeployedItemForServiceItem returns the current deployed item for a service item.
//
// This is the call that reconciles identity. A deployed item's natural key is its parent, not
// its id: a create cuts a new version rather than editing the last, so a caller that remembered
// an id from an earlier create would go on reading a superseded version. Looking the current
// one up by service item is the stable way to ask "what does the platform think is deployed
// here" - an in-place update rewrites the current version, but which record is current can only
// be answered by parent.
//
// "Current" means the highest version, which is what the platform itself resolves a service
// item's deployed item to. It returns an error wrapping ErrNotFound when the service item has
// never had one, which a provider should read as drift rather than as a failure.
func (c *Client) FindDeployedItemForServiceItem(
	ctx context.Context,
	pov POV,
	serviceItemID int,
) (*DeployedItem, error) {
	// The current record is the highest version, and a service item's history can run past one
	// page - 32 versions against a page size of 20 has been seen. Taking the max over a single
	// page returns a stale version the moment the history outgrows a page, so this pages through
	// every result and takes the true max. Because it scans everything, the answer does not
	// depend on the server honouring any particular ordering.
	const pageSize = 100

	var current *DeployedItem
	seen := 0
	for offset := 0; ; offset += pageSize {
		response, err := c.ListDeployedItems(ctx, &ListDeployedItemsRequest{
			POV:           pov,
			ServiceItemID: []int{serviceItemID},
			Limit:         pageSize,
			Offset:        offset,
		})
		if err != nil {
			return nil, fmt.Errorf(
				"failed to look up the deployed item for service item %d: %w", serviceItemID, err,
			)
		}

		for index := range response.Results {
			if current == nil || response.Results[index].Version > current.Version {
				candidate := response.Results[index]
				current = &candidate
			}
		}

		// Stop on a short page or once every counted item has been read. Guarding on Count as
		// well as page length means a server that ignores the limit cannot spin this loop.
		seen += len(response.Results)
		if len(response.Results) < pageSize || seen >= response.Count {
			break
		}
	}

	if current == nil {
		return nil, fmt.Errorf(
			"%w: no deployed item for service item %d", ErrNotFound, serviceItemID,
		)
	}
	return current, nil
}

// CreateDeployedItem records a deployment against exactly one parent, returning the item the
// platform stored - which is where to read the version it assigned.
//
// A create whose data repeats the previous version byte for byte is answered with 200 and the
// existing item rather than 201 and a new one: the platform calls that "no change detected".
// This makes a repeated apply cheap, but it does mean success is not proof that a new version
// exists. Compare the returned Version against the one you had if that distinction matters.
func (c *Client) CreateDeployedItem(
	ctx context.Context,
	pov POV,
	body *DeployedItemWrite,
) (*DeployedItem, error) {
	if body == nil {
		return nil, fmt.Errorf("a deployed item body is required")
	}
	if err := body.Validate(); err != nil {
		return nil, fmt.Errorf("invalid deployed item: %w", err)
	}

	payload := deployedItemBody{Data: deployedItemData(body.Data)}
	if body.ServiceItemID != 0 {
		payload.ServiceItem = c.deployedItemParentLink(pov, "service_items", body.ServiceItemID)
	} else {
		payload.ChangeInstance = c.deployedItemParentLink(
			pov, "change_instances", body.ChangeInstanceID,
		)
	}

	endpoint := fmt.Sprintf("orcabase/%s/deployed_items/", pov.orDefault())

	var response DeployedItem
	if err := c.doRequest(ctx, "POST", endpoint, payload, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// UpdateDeployedItem rewrites a deployed item's data in place. Data is the only field a
// client may change: the id, the version and the parent are the platform's to assign, and a
// deployed item that pointed somewhere else would no longer be the same record.
//
// The API's serializer insists on a parent link in the body of every write, a partial one
// included, so this reads the item first and echoes its existing parent straight back. That
// costs one extra round trip and buys a clear ErrNotFound when the item has been removed out
// from under the caller - which is what a Terraform provider wants to learn anyway.
func (c *Client) UpdateDeployedItem(
	ctx context.Context,
	pov POV,
	id int,
	data json.RawMessage,
) (*DeployedItem, error) {
	if len(data) > 0 && !json.Valid(data) {
		return nil, fmt.Errorf("deployed item data is not valid JSON")
	}

	existing, err := c.GetDeployedItem(ctx, pov, id)
	if err != nil {
		return nil, fmt.Errorf("failed to read deployed item %d before updating it: %w", id, err)
	}

	// Prefer the service item: the platform sets it on every item it accepts, including the
	// ones created against a change instance, so it is the parent that is always there.
	payload := deployedItemBody{Data: deployedItemData(data)}
	switch {
	case existing.ServiceItem != "":
		payload.ServiceItem = existing.ServiceItem
	case existing.ChangeInstance != "":
		payload.ChangeInstance = existing.ChangeInstance
	default:
		return nil, fmt.Errorf(
			"deployed item %d has no parent to write back, so the API will not accept an update", id,
		)
	}

	endpoint := fmt.Sprintf("orcabase/%s/deployed_items/%d/", pov.orDefault(), id)

	var response DeployedItem
	if err := c.doRequest(ctx, "PATCH", endpoint, payload, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteDeployedItem removes one version of a deployed item.
//
// Deleting is not the same as un-deploying: because versions accumulate, removing the current
// one promotes the previous version to current rather than leaving the service item with
// nothing recorded. Delete the whole history when the intent is "nothing is deployed here".
func (c *Client) DeleteDeployedItem(ctx context.Context, pov POV, id int) error {
	endpoint := fmt.Sprintf("orcabase/%s/deployed_items/%d/", pov.orDefault(), id)

	return c.doRequest(ctx, "DELETE", endpoint, nil, nil)
}
