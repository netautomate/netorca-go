package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// AIProcessor is one configured AI processor: the model, prompt and settings the pack framework
// runs for a service at a given stage.
//
// A processor's identity on the platform is the (Service, ActionType) pair rather than its name
// or its id - the API enforces uniqueness across the two, so a service has at most one processor
// per stage. FindAIProcessor looks one up by that pair, which is what an importer or a reconciler
// needs, since a practitioner knows the service and the stage they configured and not the
// generated id. Re-typing a processor in place succeeds only when the target pair happens to be
// free, so a caller that treats the pair as immutable never has to reason about the collision.
type AIProcessor struct {
	// ID is the unique identifier for the processor.
	ID int `json:"id"`
	// Name is the human-readable label the platform shows for the processor.
	Name string `json:"name"`
	// Service is the service the processor runs for. The API renders it as
	// {"id": 49, "name": "VIRTUAL_SERVER"} on read but accepts a bare id on write; RefID
	// absorbs both forms, so callers only ever deal with the id.
	Service RefID `json:"service"`
	// LLMModel is the language model the processor runs against, with the same read/write
	// asymmetry as Service.
	LLMModel RefID `json:"llm_model"`
	// ActionType is the pipeline stage, or the standalone processor kind, this instance
	// serves; compare against the PackAction* constants.
	ActionType PackActionType `json:"action_type"`
	// Prompt is the instruction sent to the model, and the field practitioners iterate on.
	Prompt string `json:"prompt"`
	// ExtraData holds the per-action-type settings - include_change_instance and friends for
	// the pipeline stages, schedule_crontab for the optimiser, allow_auto_approval for the
	// validator. The API fills in every default before answering, so a processor created
	// without settings still reads back a fully populated object.
	ExtraData map[string]any `json:"extra_data"`
	// ResponseSchema is the JSON Schema the model's answer must satisfy. It carries the JSON
	// literal null when the processor takes the platform's default for its action type, which
	// the change instance validator always does - the platform overwrites any schema supplied
	// for that type with its own.
	ResponseSchema json.RawMessage `json:"response_schema"`
	// Active reports whether the processor runs. An inactive processor stays configured but is
	// skipped, which is how a stage is switched off without losing its prompt.
	Active bool `json:"active"`
}

// AIProcessorWrite is the request body for creating a processor.
//
// It is deliberately a separate type from AIProcessor: the read shape carries server-rendered
// relation objects and a server-filled ExtraData, so echoing one straight back would submit
// values the caller never chose. Service and LLMModel are RefID here too, because the type
// marshals as the bare id the API wants - a value read off an AIProcessor assigns across
// unchanged.
type AIProcessorWrite struct {
	// Name labels the processor. The API rejects an empty or null one.
	Name string `json:"name"`
	// Service is the id of the service the processor runs for. Note the singular key: the
	// list filter is service_id, but the create body wants service.
	Service RefID `json:"service"`
	// LLMModel is the id of the language model to run against.
	LLMModel RefID `json:"llm_model"`
	// ActionType is the stage this processor serves. Together with Service it forms the
	// processor's identity, so the pair must still be free.
	ActionType PackActionType `json:"action_type"`
	// Prompt is the instruction sent to the model.
	Prompt string `json:"prompt"`
	// ExtraData carries the per-action-type settings, omitted when empty so the platform
	// applies its own defaults. The optimiser is the one type with required settings: it
	// wants schedule_enabled and schedule_crontab here.
	ExtraData map[string]any `json:"extra_data,omitempty"`
	// ResponseSchema is an optional JSON Schema constraining the model's answer. Omitted when
	// empty, which leaves the platform's default for the action type in force.
	ResponseSchema json.RawMessage `json:"response_schema,omitempty"`
	// Active is a pointer so that "unset" and "explicitly inactive" stay distinguishable. The
	// platform defaults a new processor to active, and a plain bool's zero value would
	// silently mean the opposite of what omitting the field does.
	Active *bool `json:"active,omitempty"`
}

// ListAIProcessorsRequest filters a processor listing. Every field is optional; the zero value
// lists every processor the API key can see.
//
// The API rejects filter names it does not recognise outright rather than ignoring them, so the
// fields here are the whole supported set.
type ListAIProcessorsRequest struct {
	// POV is the point of view to query from. Defaults to serviceowner.
	POV POV
	// ServiceID restricts results to processors belonging to these services.
	ServiceID []int
	// LLMModelID restricts results to processors running against these models.
	LLMModelID []int
	// ActionType restricts results to these stages.
	ActionType []PackActionType
	// Active filters on the enabled flag. Nil means "either"; this is a pointer because
	// active=false is a meaningful query - the processors somebody switched off - rather
	// than an absent one.
	Active *bool
	// Ordering names the field to sort by; prefix with "-" to reverse.
	Ordering string
	// Limit caps the number of results returned. Zero leaves the server's default page size
	// in force, so it is not the same as "everything".
	Limit int
	// Offset skips this many results.
	Offset int
}

// ToQueryParams renders the filters as a percent-encoded query string, "" when empty.
//
// It returns an error to match the shape of the other list requests, whose structured filters
// can fail to encode; nothing here can.
func (r *ListAIProcessorsRequest) ToQueryParams() (string, error) {
	params := newQueryParams()
	params.SetInts("service_id", r.ServiceID)
	params.SetInts("llm_model_id", r.LLMModelID)

	// PackActionType is a named type, so the values need widening before they can be
	// comma-joined into the "in" lookup the API expects.
	actionTypes := make([]string, 0, len(r.ActionType))
	for _, actionType := range r.ActionType {
		actionTypes = append(actionTypes, string(actionType))
	}
	params.SetStrings("action_type", actionTypes)

	params.SetBool("active", r.Active)
	params.SetString("ordering", r.Ordering)
	params.SetInt("limit", r.Limit)
	params.SetInt("offset", r.Offset)

	return params.Encode(), nil
}

// ListAIProcessorsResponse is the paginated envelope the API returns for a processor listing.
type ListAIProcessorsResponse struct {
	// Count is the total number of matching processors, across all pages.
	Count int `json:"count"`
	// Next is the URL of the next page, nil on the last page.
	Next *string `json:"next"`
	// Previous is the URL of the previous page, nil on the first page.
	Previous *string `json:"previous"`
	// Results is this page of processors.
	Results []AIProcessor `json:"results"`
}

// AIProcessorHistoryEntry is one recorded revision of a processor. The snapshot fields carry the
// processor as it stood at that revision, so two entries can be diffed to see what a change
// actually altered - the usual way a prompt regression is traced back to the edit that caused it.
//
// There is deliberately no Active field: the platform excludes the enabled flag from history
// tracking, so switching a processor off and on again leaves no entry behind.
type AIProcessorHistoryEntry struct {
	// HistoryID identifies the revision itself rather than the processor.
	HistoryID int `json:"history_id"`
	// HistoryDate is when the revision was recorded. Entries arrive newest first.
	HistoryDate time.Time `json:"history_date"`
	// HistoryUser is the username behind the change, nil when the platform made it itself -
	// a migration or an automated action carries no user.
	HistoryUser *string `json:"history_user"`
	// HistoryType is the kind of change: "+" for the creation, "~" for an update and "-" for
	// a deletion.
	HistoryType string `json:"history_type"`
	// Version numbers the revisions from 1 upwards, so the newest entry carries the highest
	// number. Comparing it is far cheaper than diffing snapshots when all a caller needs to
	// know is whether the processor has moved on since it last looked.
	Version int `json:"version"`
	// ID is the processor this revision belongs to.
	ID int `json:"id"`
	// Name is the processor's name at this revision.
	Name string `json:"name"`
	// Service is the service the processor belonged to at this revision.
	Service RefID `json:"service"`
	// LLMModel is the model the processor ran against at this revision.
	LLMModel RefID `json:"llm_model"`
	// ActionType is the stage the processor served at this revision.
	ActionType PackActionType `json:"action_type"`
	// Prompt is the instruction as it stood at this revision.
	Prompt string `json:"prompt"`
	// ExtraData is the settings object as it stood at this revision.
	ExtraData map[string]any `json:"extra_data"`
	// ResponseSchema is the JSON Schema as it stood at this revision.
	ResponseSchema json.RawMessage `json:"response_schema"`
}

// ListAIProcessors returns the AI processors matching the given filters.
//
// Filtering by ServiceID and ActionType together narrows to at most one processor, since the
// platform keys a processor by that pair; FindAIProcessor wraps exactly that call.
func (c *Client) ListAIProcessors(
	ctx context.Context,
	filters *ListAIProcessorsRequest,
) (*ListAIProcessorsResponse, error) {
	if filters == nil {
		filters = &ListAIProcessorsRequest{}
	}

	query, err := filters.ToQueryParams()
	if err != nil {
		return nil, fmt.Errorf("failed to convert filters to query params: %w", err)
	}

	endpoint := fmt.Sprintf("external/%s/ai_processors/%s", filters.POV.orDefault(), query)

	var response ListAIProcessorsResponse
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetAIProcessor returns a single processor by id.
// It returns an error wrapping ErrNotFound when no such processor exists.
func (c *Client) GetAIProcessor(ctx context.Context, pov POV, id int) (*AIProcessor, error) {
	endpoint := fmt.Sprintf("external/%s/ai_processors/%d/", pov.orDefault(), id)

	var response AIProcessor
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// CreateAIProcessor creates a processor and returns it as the platform stored it.
//
// The returned value is worth reading back rather than assuming: ExtraData comes back with every
// default filled in, and a change instance validator has its ResponseSchema replaced with the
// platform's own. A caller that keeps the submitted body as its record of state will disagree
// with the server on both.
//
// The (Service, ActionType) pair must be free - creating a second processor for a pair that is
// already taken fails with a 400 rather than replacing the incumbent. Check with FindAIProcessor
// first where that is a possibility.
func (c *Client) CreateAIProcessor(
	ctx context.Context,
	pov POV,
	body *AIProcessorWrite,
) (*AIProcessor, error) {
	if body == nil {
		return nil, fmt.Errorf("AI processor body cannot be nil")
	}

	endpoint := fmt.Sprintf("external/%s/ai_processors/", pov.orDefault())

	var response AIProcessor
	if err := c.doRequest(ctx, "POST", endpoint, body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// UpdateAIProcessor applies a partial update to a processor and returns the stored result.
//
// The patch is sent verbatim, so its keys must be the API's own field names ("prompt",
// "llm_model", "extra_data" and so on). It is a map rather than a struct on purpose: the platform
// applies its server-side defaults to every field a request mentions, so submitting a whole
// object would quietly reset settings the caller never declared - ExtraData above all, which
// reads back with every key populated and would carry those values straight back in.
//
// Service and ActionType can be patched, but only onto a pair that is free; the platform enforces
// uniqueness across the two.
func (c *Client) UpdateAIProcessor(
	ctx context.Context,
	pov POV,
	id int,
	patch map[string]any,
) (*AIProcessor, error) {
	// An empty patch is a caller bug rather than a no-op worth honouring: it would spend a
	// round trip to change nothing, and returns a body indistinguishable from a real update.
	if len(patch) == 0 {
		return nil, fmt.Errorf("AI processor patch cannot be empty")
	}

	endpoint := fmt.Sprintf("external/%s/ai_processors/%d/", pov.orDefault(), id)

	var response AIProcessor
	if err := c.doRequest(ctx, "PATCH", endpoint, patch, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteAIProcessor removes a processor.
//
// It returns an error wrapping ErrNotFound when the processor has already gone, which a
// reconciler should treat as the desired end state rather than as a failure.
func (c *Client) DeleteAIProcessor(ctx context.Context, pov POV, id int) error {
	endpoint := fmt.Sprintf("external/%s/ai_processors/%d/", pov.orDefault(), id)

	// The API answers 204 with no body, so there is nothing to decode into.
	return c.doRequest(ctx, "DELETE", endpoint, nil, nil)
}

// ListAIProcessorHistory returns a processor's revisions, newest first.
//
// The platform records an entry for every change bar a toggle of Active, which it excludes from
// tracking - a processor that has only ever been enabled and disabled therefore has a single
// entry, from its creation.
func (c *Client) ListAIProcessorHistory(
	ctx context.Context,
	pov POV,
	id int,
) ([]AIProcessorHistoryEntry, error) {
	endpoint := fmt.Sprintf("external/%s/ai_processors/%d/history/", pov.orDefault(), id)

	var raw json.RawMessage
	if err := c.doRequest(ctx, "GET", endpoint, nil, &raw); err != nil {
		return nil, err
	}

	// The route paginates, so the entries normally arrive inside the standard envelope. Sniff
	// the shape rather than assuming it: the view only wraps its results while a paginator is
	// configured, and pagination is an instance-wide setting an on-prem deployment can turn
	// off, which would otherwise leave this failing to decode at all.
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var entries []AIProcessorHistoryEntry
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			return nil, fmt.Errorf("failed to decode AI processor history: %w", err)
		}
		return entries, nil
	}

	var envelope struct {
		Results []AIProcessorHistoryEntry `json:"results"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return nil, fmt.Errorf("failed to decode AI processor history envelope: %w", err)
	}
	return envelope.Results, nil
}

// FindAIProcessor returns the processor for a (service, action type) pair - the pair the platform
// treats as a processor's identity.
//
// This is the lookup an import needs, because a practitioner knows their service and the stage
// they configured rather than the processor's generated id, and the lookup a reconciler needs to
// decide whether its declared processor already exists.
//
// It returns an error wrapping ErrNotFound when the pair has no processor, so "not configured"
// stays distinguishable from a genuine failure with errors.Is.
func (c *Client) FindAIProcessor(
	ctx context.Context,
	pov POV,
	serviceID int,
	actionType PackActionType,
) (*AIProcessor, error) {
	response, err := c.ListAIProcessors(ctx, &ListAIProcessorsRequest{
		POV:        pov,
		ServiceID:  []int{serviceID},
		ActionType: []PackActionType{actionType},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to look up AI processor: %w", err)
	}

	switch len(response.Results) {
	case 0:
		return nil, fmt.Errorf(
			"%w: no AI processor for service %d with action type %q",
			ErrNotFound, serviceID, actionType,
		)
	case 1:
		return &response.Results[0], nil
	}

	// The platform's uniqueness constraint should make this unreachable. Report it rather
	// than picking one arbitrarily: a caller that acted on the wrong processor would go on
	// to reconfigure the wrong stage entirely.
	return nil, fmt.Errorf(
		"expected one AI processor for service %d with action type %q, got %d",
		serviceID, actionType, len(response.Results),
	)
}
