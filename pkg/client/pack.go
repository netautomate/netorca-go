package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// PackActionType identifies a stage of the NetOrca AI pack pipeline, or one of the
// standalone processor types that run outside it.
//
// The pipeline proper runs config -> verify -> execution, each stage feeding the next.
// Optimiser and ChangeInstanceValidator are separate processors: the validator reviews
// incoming change instances, the optimiser runs on a schedule to reduce retrigger cycles.
//
// Trigger accepts all five; pack data exists only for the three pipeline stages.
type PackActionType string

const (
	// PackActionConfig is the "config" stage - the AI-generated desired configuration.
	PackActionConfig PackActionType = "config"
	// PackActionVerify is the "verify" stage - verification of the generated configuration.
	PackActionVerify PackActionType = "verify"
	// PackActionExecution is the "execution" stage - the execution/deployment data.
	PackActionExecution PackActionType = "execution"
	// PackActionOptimiser is the scheduled optimiser processor. Note the British spelling,
	// which is what the API expects.
	PackActionOptimiser PackActionType = "optimiser"
	// PackActionChangeInstanceValidator is the processor that reviews incoming change
	// instances, optionally approving or rejecting them automatically.
	PackActionChangeInstanceValidator PackActionType = "change_instance_validator"
)

// IsPipelineStage reports whether the action is one of the three stages that produce pack
// data. Pack data cannot be read or pushed for the optimiser or validator processors.
func (a PackActionType) IsPipelineStage() bool {
	switch a {
	case PackActionConfig, PackActionVerify, PackActionExecution:
		return true
	}
	return false
}

// PackScope is the kind of object a pack pipeline runs against. Most workflows are scoped to
// a service item - one consumer's request - but a processor can also be scoped to a whole
// service.
type PackScope string

const (
	// PackScopeServiceItem scopes pack operations to a single service item.
	PackScopeServiceItem PackScope = "service_item"
	// PackScopeService scopes pack operations to an entire service.
	PackScopeService PackScope = "service"
)

// Validate reports whether the scope is one the API recognises.
func (s PackScope) Validate() error {
	switch s {
	case PackScopeServiceItem, PackScopeService:
		return nil
	case "":
		return fmt.Errorf("pack scope cannot be empty (expected %q or %q)", PackScopeServiceItem, PackScopeService)
	}
	return fmt.Errorf("invalid pack scope %q (expected %q or %q)", s, PackScopeServiceItem, PackScopeService)
}

// orDefault returns the scope, falling back to service_item - the scope of essentially every
// real workflow, and the one the original pack getters hardcoded.
func (s PackScope) orDefault() PackScope {
	if s == "" {
		return PackScopeServiceItem
	}
	return s
}

// PackData is the persisted output of a single pack pipeline stage.
// The Data field holds the AI-generated JSON payload that callers act on; unmarshal it into your
// own shape. Server fields that are not modelled here are ignored during decoding.
type PackData struct {
	// ID is the unique identifier for the pack data record.
	ID int `json:"id"`
	// Created is the timestamp when the pack data was created.
	Created time.Time `json:"created"`
	// Modified is the timestamp when the pack data was last modified.
	Modified time.Time `json:"modified"`
	// ActionType is the pipeline stage this data belongs to (config, verify or execution).
	ActionType string `json:"action_type"`
	// Data is the AI-generated JSON payload for this stage - the field callers read.
	Data json.RawMessage `json:"data"`
	// ObjectID is the id of the scoped object (the service item) this data applies to.
	ObjectID int `json:"object_id"`
	// Scope is the scoped-object envelope the API returns ({"scope":"service_item","data":{...}}); freeform JSON.
	Scope json.RawMessage `json:"scope"`
	// SIDeclaration is the service item declaration snapshot associated with this data.
	SIDeclaration json.RawMessage `json:"si_declaration"`
}

// ScopeKind extracts the scope name ("service_item" or "service") from the scope envelope.
// The executor loop needs it to report results back against the same scope the pipeline ran on
// rather than assuming service_item.
func (p *PackData) ScopeKind() PackScope {
	var envelope struct {
		Scope string `json:"scope"`
	}
	if err := json.Unmarshal(p.Scope, &envelope); err != nil {
		return ""
	}
	return PackScope(envelope.Scope)
}

// retriggerPackRequest is the request body for RetriggerPack.
// An empty comment is omitted so the API receives an empty JSON object.
type retriggerPackRequest struct {
	// ServiceownerComment is optional feedback passed to the AI processor on retrigger.
	ServiceownerComment string `json:"serviceowner_comment,omitempty"`
}

// packDataPath builds the pack data route for a scoped object and stage. The trailing slash is
// the canonical DRF route; without it the API replies 301 to the slashed URL.
func packDataPath(pov POV, scope PackScope, objectID int, action PackActionType) string {
	return fmt.Sprintf("external/%s/pack/data/%s/%d/%s/", pov.orDefault(), scope.orDefault(), objectID, action)
}

// GetPackConfig returns the latest "config" stage data for the given service item.
// It returns ErrPackDataNotFound if the config stage has not produced data yet.
func (c *Client) GetPackConfig(serviceItemID int) (*PackData, error) {
	return c.GetPackData(context.Background(), POVServiceOwner, PackScopeServiceItem, serviceItemID, PackActionConfig)
}

// GetPackVerify returns the latest "verify" stage data for the given service item.
// It returns ErrPackDataNotFound if the verify stage has not produced data yet.
func (c *Client) GetPackVerify(serviceItemID int) (*PackData, error) {
	return c.GetPackData(context.Background(), POVServiceOwner, PackScopeServiceItem, serviceItemID, PackActionVerify)
}

// GetPackExecution returns the latest "execution" stage data for the given service item.
// It returns ErrPackDataNotFound if the execution stage has not produced data yet.
func (c *Client) GetPackExecution(serviceItemID int) (*PackData, error) {
	return c.GetPackData(context.Background(), POVServiceOwner, PackScopeServiceItem, serviceItemID, PackActionExecution)
}

// GetPackData returns the latest data for one stage of a scoped object's pack pipeline.
//
// It returns ErrPackDataNotFound (which unwraps to ErrNotFound) when the stage has not
// produced data yet - a normal, expected state while a pipeline is still running.
func (c *Client) GetPackData(
	ctx context.Context,
	pov POV,
	scope PackScope,
	objectID int,
	action PackActionType,
) (*PackData, error) {
	if !action.IsPipelineStage() {
		return nil, fmt.Errorf("pack data exists only for the config, verify and execution stages, got %q", action)
	}

	var response PackData
	err := c.doRequest(ctx, "GET", packDataPath(pov, scope, objectID, action), nil, &response)
	if err != nil {
		// A 404 means the stage has not produced data yet - report it as the sentinel
		// callers already branch on rather than as a generic failure.
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return nil, ErrPackDataNotFound
		}
		return nil, err
	}

	return &response, nil
}

// GetPackDataByID returns a single pack data record by its own id, rather than by the object and
// stage it belongs to.
//
// Use it to re-read a record whose id you already hold - one embedded in a pipeline run, or one
// PushPackData just returned - which is how a caller reads back exactly the record it saw rather
// than whatever the stage's newest has since become.
//
// Unlike GetPackData, a 404 here is a plain error wrapping ErrNotFound and not
// ErrPackDataNotFound: an unknown id is a bad reference, whereas a stage with no data yet is a
// normal phase of a running pipeline, and the two should not be handled alike.
func (c *Client) GetPackDataByID(ctx context.Context, pov POV, id int) (*PackData, error) {
	endpoint := fmt.Sprintf("external/%s/pack/data/%d/", pov.orDefault(), id)

	var response PackData
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// ListPackDataRequest paginates a pack data listing.
//
// There is deliberately nothing to filter on. The pack data view declares no filterset, so the
// platform silently ignores every query parameter beyond the paginator's limit and offset and
// the ordering backend's ordering - a filter sent here is dropped rather than rejected, which is
// worse than an error because the caller gets a plausible, unfiltered answer back. Narrow the
// results with Ordering and Limit, or use GetPackData when you know the object and stage.
type ListPackDataRequest struct {
	// POV is the point of view to query from. Defaults to serviceowner.
	POV POV
	// Limit caps the number of results returned. Zero means no cap.
	Limit int
	// Offset skips this many results.
	Offset int
	// Ordering names the field to sort by; prefix with "-" to reverse. "-created" is the
	// useful one: it puts the newest records first.
	Ordering string
}

// ToQueryParams renders the pagination parameters as a percent-encoded query string, "" when
// empty. It cannot fail, unlike its siblings, because nothing here needs encoding as JSON.
func (r *ListPackDataRequest) ToQueryParams() string {
	params := newQueryParams()
	params.SetInt("limit", r.Limit)
	params.SetInt("offset", r.Offset)
	params.SetString("ordering", r.Ordering)
	return params.Encode()
}

// ListPackDataResponse is the paginated envelope the API returns for a pack data listing.
type ListPackDataResponse struct {
	// Count is the total number of records visible to the API key, across all pages.
	Count int `json:"count"`
	// Next is the URL of the next page, nil on the last page.
	Next *string `json:"next"`
	// Previous is the URL of the previous page, nil on the first page.
	Previous *string `json:"previous"`
	// Results is this page of records.
	Results []PackData `json:"results"`
}

// ListPackData returns pack data records across every object and stage the API key can see,
// newest-first if you ask for it with Ordering.
//
// The listing is unfiltered by design (see ListPackDataRequest), so it is a sweep rather than a
// lookup: reach for it to audit what the pipelines have produced recently, and for a specific
// object's stage use GetPackData, which asks the platform to do the narrowing.
func (c *Client) ListPackData(
	ctx context.Context,
	filters *ListPackDataRequest,
) (*ListPackDataResponse, error) {
	if filters == nil {
		filters = &ListPackDataRequest{}
	}

	endpoint := fmt.Sprintf("external/%s/pack/data/%s", filters.POV.orDefault(), filters.ToQueryParams())

	var response ListPackDataResponse
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// PushPackData writes stage data into a pack pipeline - how an external executor reports the
// outcome of work the platform delegated to it.
//
// The data argument is stored verbatim as the stage's payload; there is no envelope, so pass
// the object you want the stage to hold (for example {"success": true, "deployed_at": ...}).
//
// This is deliberately not idempotent: every call creates a new stage record, because stage
// data is versioned per run and a skipped push would leave the pipeline waiting forever.
func (c *Client) PushPackData(
	ctx context.Context,
	pov POV,
	scope PackScope,
	objectID int,
	action PackActionType,
	data any,
) (*PackData, error) {
	if !action.IsPipelineStage() {
		return nil, fmt.Errorf("pack data exists only for the config, verify and execution stages, got %q", action)
	}
	if data == nil {
		return nil, fmt.Errorf("pack data payload cannot be nil")
	}

	var response PackData
	if err := c.doRequest(ctx, "POST", packDataPath(pov, scope, objectID, action), data, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// TriggerPack starts one AI processor for a scoped object and returns the API's confirmation
// message (e.g. "AI Processor has been triggered").
//
// Every trigger invokes the service's LLM and costs real money; the accumulated spend shows up
// as the pipeline's Cost field. Success means the platform accepted the trigger, not that the
// run succeeded - poll the pipeline state for the outcome.
func (c *Client) TriggerPack(
	ctx context.Context,
	pov POV,
	scope PackScope,
	objectID int,
	action PackActionType,
) (string, error) {
	endpoint := fmt.Sprintf(
		"external/%s/pack/trigger/%s/%d/%s/",
		pov.orDefault(), scope.orDefault(), objectID, action,
	)

	// The API returns a bare JSON string message.
	var message string
	if err := c.doRequest(ctx, "POST", endpoint, nil, &message); err != nil {
		return "", err
	}
	return message, nil
}

// RetriggerPack re-runs the AI pack pipeline for the given service item, always restarting from the
// config stage. The optional serviceownerComment is passed to the AI processor as feedback (for
// example, why a previous verify result was rejected); pass "" to send none. On success it returns
// the confirmation message from the API (e.g. "AI Processor has been retriggered").
func (c *Client) RetriggerPack(serviceItemID int, serviceownerComment string) (string, error) {
	return c.RetriggerPackScoped(
		context.Background(), POVServiceOwner, PackScopeServiceItem, serviceItemID, serviceownerComment,
	)
}

// RetriggerPackScoped re-runs a scoped object's pack pipeline, always restarting from the config
// stage regardless of how far the previous run got.
//
// The optional comment is folded into the AI processor's prompt as feedback - typically why the
// previous render was rejected, which is what makes the loop self-healing. Pass "" to send none.
//
// Like TriggerPack, this costs an LLM run.
func (c *Client) RetriggerPackScoped(
	ctx context.Context,
	pov POV,
	scope PackScope,
	objectID int,
	comment string,
) (string, error) {
	endpoint := fmt.Sprintf(
		"external/%s/pack/retrigger/%s/%d/",
		pov.orDefault(), scope.orDefault(), objectID,
	)

	body := retriggerPackRequest{ServiceownerComment: comment}

	// The API returns a bare JSON string message, e.g. "AI Processor has been retriggered".
	var message string
	if err := c.doRequest(ctx, "POST", endpoint, body, &message); err != nil {
		return "", err
	}
	return message, nil
}
