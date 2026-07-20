package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// PackPipelineState is the lifecycle state of a pack pipeline run.
type PackPipelineState string

const (
	// PackPipelineOK means the run finished successfully.
	PackPipelineOK PackPipelineState = "OK"
	// PackPipelineFailed means the run finished unsuccessfully; inspect its stage data.
	PackPipelineFailed PackPipelineState = "FAILED"
	// PackPipelineScheduled means the run is queued by a scheduled (crontab) processor.
	PackPipelineScheduled PackPipelineState = "SCHEDULED"
	// PackPipelineWaitingForResponse means the run is parked waiting for an external
	// executor to report a stage result. This is the state the executor loop drains.
	PackPipelineWaitingForResponse PackPipelineState = "WAITING_FOR_RESPONSE"
)

// PackPipeline is one recorded run of the pack framework against a scoped object.
//
// Every field except Applied is produced by the platform and is read-only; Applied is the
// executor's acknowledgement that it has acted on the run's output. A run with State OK and
// Applied false is work waiting to be done - that pair is the executor work queue.
type PackPipeline struct {
	// ID is the unique identifier for the pipeline run.
	ID int `json:"id"`
	// Version increments once per run of the same scoped object, so a retrigger produces
	// a higher version. Waiting for a version increase is how you tell a re-render from
	// the run you already saw.
	Version int `json:"version"`
	// State is the run's lifecycle state; compare against the PackPipeline* constants.
	State string `json:"state"`
	// CurrentStage is the stage the run has reached (config, verify or execution).
	CurrentStage string `json:"current_stage"`
	// Applied records whether an executor has acted on this run. The only writable field.
	Applied bool `json:"applied"`
	// Cost is the accumulated LLM spend for the run. The API renders it as a decimal
	// string to avoid float rounding; use ParseCost for a float64.
	Cost json.Number `json:"cost"`
	// Created is the timestamp when the run started.
	Created time.Time `json:"created"`
	// Config is the config stage's data, embedded. Nil until the stage produces output.
	Config *PackData `json:"config"`
	// Verify is the verify stage's data, embedded. Nil when the stage did not run.
	Verify *PackData `json:"verify"`
	// Execution is the execution stage's data, embedded. Nil until an executor reports.
	Execution *PackData `json:"execution"`
	// AIProcessorResponseConfig is the raw processor response for the config stage.
	AIProcessorResponseConfig json.RawMessage `json:"ai_processor_response_config"`
	// AIProcessorResponseVerify is the raw processor response for the verify stage.
	AIProcessorResponseVerify json.RawMessage `json:"ai_processor_response_verify"`
	// AIProcessorResponseExecution is the raw processor response for the execution stage.
	AIProcessorResponseExecution json.RawMessage `json:"ai_processor_response_execution"`
}

// ParseCost returns the run's LLM cost as a float64. It returns 0 when the API sent no cost.
func (p *PackPipeline) ParseCost() (float64, error) {
	if p.Cost == "" {
		return 0, nil
	}
	return p.Cost.Float64()
}

// PackPipelineVersion is one entry of a scoped object's run history - a cheap way to detect
// that a new run exists without fetching every full pipeline.
type PackPipelineVersion struct {
	// ID is the pipeline run's identifier.
	ID int `json:"id"`
	// Version is the run's version number.
	Version int `json:"version"`
}

// ListPackPipelinesRequest filters a pack pipeline listing. Every field is optional; the zero
// value lists everything the API key can see.
//
// The executor work queue is State: []string{"OK"} with Applied pointing at false.
type ListPackPipelinesRequest struct {
	// POV is the point of view to query from. Defaults to serviceowner.
	POV POV
	// ApplicationID restricts results to these applications.
	ApplicationID []int
	// ConsumerTeamID restricts results to these consumer teams.
	ConsumerTeamID []int
	// ServiceID restricts results to these services.
	ServiceID []int
	// ServiceItemID restricts results to these service items.
	ServiceItemID []int
	// Applied filters on the executor acknowledgement flag. Nil means "either";
	// this is a pointer because applied=false is a meaningful query, not an absent one.
	Applied *bool
	// State restricts results to these run states.
	State []string
	// Version restricts results to these version numbers.
	Version []int
	// StartDate restricts results to runs created at or after this time.
	StartDate time.Time
	// EndDate restricts results to runs created at or before this time.
	EndDate time.Time
	// Declaration matches config stage data fields exactly.
	Declaration map[string]any
	// DeclarationRegex matches config stage data fields against regular expressions.
	DeclarationRegex map[string]any
	// DeclarationContains matches config stage data fields by substring.
	DeclarationContains map[string]any
	// Limit caps the number of results returned. Zero means no cap.
	Limit int
	// Offset skips this many results.
	Offset int
	// Ordering names the field to sort by; prefix with "-" to reverse.
	Ordering string
}

// ToQueryParams renders the filters as a percent-encoded query string, "" when empty.
func (r *ListPackPipelinesRequest) ToQueryParams() (string, error) {
	params := newQueryParams()
	params.SetInts("application_id", r.ApplicationID)
	params.SetInts("consumer_team_id", r.ConsumerTeamID)
	params.SetInts("service_id", r.ServiceID)
	params.SetInts("service_item_id", r.ServiceItemID)
	params.SetBool("applied", r.Applied)
	params.SetStrings("state", r.State)
	params.SetInts("version", r.Version)
	params.SetTime("start_date", r.StartDate)
	params.SetTime("end_date", r.EndDate)
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

// ListPackPipelinesResponse is the paginated envelope the API returns for a pipeline listing.
type ListPackPipelinesResponse struct {
	// Count is the total number of matching runs, across all pages.
	Count int `json:"count"`
	// Next is the URL of the next page, nil on the last page.
	Next *string `json:"next"`
	// Previous is the URL of the previous page, nil on the first page.
	Previous *string `json:"previous"`
	// Results is this page of runs.
	Results []PackPipeline `json:"results"`
}

// ListPackPipelines returns pack pipeline runs matching the given filters.
//
// Passing State: []string{"OK"} with Applied set to false yields the executor work queue:
// successful runs that nobody has acted on yet.
func (c *Client) ListPackPipelines(
	ctx context.Context,
	filters *ListPackPipelinesRequest,
) (*ListPackPipelinesResponse, error) {
	if filters == nil {
		filters = &ListPackPipelinesRequest{}
	}

	query, err := filters.ToQueryParams()
	if err != nil {
		return nil, fmt.Errorf("failed to convert filters to query params: %w", err)
	}

	endpoint := fmt.Sprintf("external/%s/pack/pipelines/%s", filters.POV.orDefault(), query)

	var response ListPackPipelinesResponse
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetPackPipeline returns a single pipeline run by id.
// It returns an error wrapping ErrNotFound when no such run exists.
func (c *Client) GetPackPipeline(ctx context.Context, pov POV, id int) (*PackPipeline, error) {
	endpoint := fmt.Sprintf("external/%s/pack/pipelines/%d/", pov.orDefault(), id)

	var response PackPipeline
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetLatestPackPipeline returns the newest run for a scoped object - the call to poll after
// triggering, to watch a run reach a terminal state.
//
// It returns an error wrapping ErrNotFound when the object has never had a run.
func (c *Client) GetLatestPackPipeline(
	ctx context.Context,
	pov POV,
	scope PackScope,
	objectID int,
) (*PackPipeline, error) {
	endpoint := fmt.Sprintf(
		"external/%s/pack/pipelines/latest/%s/%d/",
		pov.orDefault(), scope.orDefault(), objectID,
	)

	var response PackPipeline
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// ListPackPipelineVersions returns a scoped object's run history as bare {version, id} pairs.
// It is much cheaper than listing full pipelines, so it is the right call for detecting that
// a retrigger has produced a new run.
func (c *Client) ListPackPipelineVersions(
	ctx context.Context,
	pov POV,
	scope PackScope,
	objectID int,
) ([]PackPipelineVersion, error) {
	endpoint := fmt.Sprintf(
		"external/%s/pack/pipelines/versions/%s/%d/",
		pov.orDefault(), scope.orDefault(), objectID,
	)

	// This route answers with a bare array rather than the paginated envelope the
	// sibling list route uses.
	var response []PackPipelineVersion
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return response, nil
}

// setPackPipelineAppliedRequest is the request body for SetPackPipelineApplied.
// Applied is the only pipeline field the platform accepts from clients.
type setPackPipelineAppliedRequest struct {
	Applied bool `json:"applied"`
}

// SetPackPipelineApplied marks a run as applied (or un-applied) - the executor's
// acknowledgement that it has acted on the run's output, which takes it off the work queue.
//
// Report the stage result with PushPackData before calling this: a run marked applied but
// never reported leaves the pipeline with no record of what happened.
func (c *Client) SetPackPipelineApplied(
	ctx context.Context,
	pov POV,
	id int,
	applied bool,
) (*PackPipeline, error) {
	endpoint := fmt.Sprintf("external/%s/pack/pipelines/%d/", pov.orDefault(), id)

	var response PackPipeline
	body := setPackPipelineAppliedRequest{Applied: applied}
	if err := c.doRequest(ctx, "PATCH", endpoint, body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
