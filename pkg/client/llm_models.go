package client

import (
	"context"
	"fmt"
	"time"
)

// This file models the platform's LLM catalogue: the models a NetOrca administrator has
// configured for the Pack framework, and which every AI processor runs on.
//
// It deliberately implements List and Get only. The platform guards these routes with
// (IsSuperUser | ReadOnlyPermissions), so an API key - which is never a superuser - is granted
// the safe methods and nothing else; a create, update or delete sent with one comes back 403
// without touching the catalogue. LLM models are provisioned by a platform administrator
// through the GUI. Do not add write methods here: they cannot succeed, and shipping them would
// only invite callers to encode provider credentials in configuration this library has no way
// to keep secret. The Ansible collection omits its write module for the same reason.

// LLMProvider names the upstream service an LLM model talks to. The provider decides how the
// platform interprets a model's extra_data, so it is the field to branch on when inspecting a
// model's configuration shape.
//
// The constants below are the providers the platform recognises today. LLMModel.Provider is
// not restricted to them: an unrecognised value decodes verbatim, so a platform that has
// gained a provider since this library was built still round-trips rather than failing.
type LLMProvider string

const (
	// LLMProviderOpenAI is OpenAI's chat completions API.
	LLMProviderOpenAI LLMProvider = "openai"
	// LLMProviderOpenAIAssistant is OpenAI's Assistants API, which is configured with an
	// assistant id alongside the key rather than with a bare model name.
	LLMProviderOpenAIAssistant LLMProvider = "openai_assistant"
	// LLMProviderAnthropic is Anthropic's API.
	LLMProviderAnthropic LLMProvider = "anthropic"
	// LLMProviderMistral is Mistral's API.
	LLMProviderMistral LLMProvider = "mistral"
	// LLMProviderAzure is Azure OpenAI.
	LLMProviderAzure LLMProvider = "azure"
	// LLMProviderCustom is any OpenAI-compatible HTTP endpoint, typically a self-hosted or
	// on-premise gateway.
	LLMProviderCustom LLMProvider = "custom"
	// LLMProviderXAI is xAI's API.
	LLMProviderXAI LLMProvider = "xai"
	// LLMProviderGemini is Google Gemini.
	LLMProviderGemini LLMProvider = "gemini"
	// LLMProviderCohere is Cohere's API.
	LLMProviderCohere LLMProvider = "cohere"
	// LLMProviderGenAI is the token-exchange GenAI gateway, configured with a token URL and a
	// request URL rather than with a single endpoint.
	LLMProviderGenAI LLMProvider = "genai"
)

// redactedPlaceholder is what Redacted substitutes for every extra_data value. It is a fixed
// literal rather than a mask of the original so that nothing about the secret - not even its
// length - survives redaction.
const redactedPlaceholder = "REDACTED"

// LLMModel is one entry of the platform's LLM catalogue: a provider, the provider-side model to
// call, a platform-level system prompt, the credentials to call it with, and the pricing the
// platform uses to account for pipeline spend.
//
// The catalogue is platform-global rather than per-team, which is why nothing in this file
// takes a POV: these are the only routes in the API with no point-of-view segment.
//
// Records are read-only to an API key; see the note at the top of this file. Treat anything
// under ExtraData as a secret, and prefer Redacted before handing a model to something that
// persists or prints it.
type LLMModel struct {
	// ID is the unique identifier for the model, and the value an AI processor's llm_model
	// field references.
	ID int `json:"id"`
	// Name is the administrator's label for the model, e.g. "Claude 4.6". It is the handle
	// humans configure against, so it is what FindLLMModelByName resolves.
	Name string `json:"name"`
	// Provider is the upstream service; compare against the LLMProvider* constants.
	Provider LLMProvider `json:"provider"`
	// ModelName is the provider's own identifier for the model, e.g. "claude-4-6".
	ModelName string `json:"model_name"`
	// Prompt is the platform-level system prompt prepended to every processor run.
	Prompt string `json:"prompt"`
	// ExtraData is the provider-specific connection configuration, and carries the provider
	// credentials - api_key, OPENAI_API_KEY or credentials, depending on the provider. The
	// platform masks the fields it has declared secret and returns the rest verbatim, so a
	// record straight off the wire is not safe to persist; see Redacted.
	ExtraData map[string]any `json:"extra_data"`
	// Timeout is how long, in seconds, the platform waits for the provider to respond.
	Timeout int `json:"timeout"`
	// InputPricePer1MTokens is the USD price per million input tokens, nil when the
	// administrator has not priced the model. It is a pointer because "unpriced" and "free"
	// both have to be expressible: collapsing them onto 0 would make cost accounting report
	// zero spend for a model nobody has costed, which reads as a working budget rather than
	// as missing data.
	InputPricePer1MTokens *float64 `json:"input_price_per_1m_tokens"`
	// OutputPricePer1MTokens is the USD price per million output tokens, nil when unpriced.
	OutputPricePer1MTokens *float64 `json:"output_price_per_1m_tokens"`
	// Metadata is free-form administrator annotation - tags, capabilities, benchmarks.
	Metadata map[string]any `json:"metadata"`
	// IsActive reports whether the model may currently be used. A disabled model stays in the
	// catalogue and stays referenceable, so check this before selecting one to run on.
	IsActive bool `json:"is_active"`
	// IsDeleted is the platform's soft-delete flag, which keeps the historical cost records
	// that reference the model intact. The catalogue routes filter soft-deleted models out, so
	// this is false on everything this client returns; it is modelled because the API sends it.
	IsDeleted bool `json:"is_deleted"`
	// DeletedAt is when the model was soft deleted, nil while it is live.
	DeletedAt *time.Time `json:"deleted_at"`
}

// Redacted returns a copy of the model with every ExtraData value replaced by "REDACTED",
// leaving the keys intact.
//
// ExtraData carries the credentials the platform uses to reach the provider. The platform masks
// the fields it has declared secret (api_key, OPENAI_API_KEY, credentials) but returns every
// other value verbatim - base URLs, assistant ids, on-premise gateway endpoints - so trusting
// the server to have masked the right things is not a policy, it is a guess. A Terraform data
// source writes its result into state, and state is not a safe place for provider credentials
// or for the shape of an internal endpoint: it is written to disk, frequently committed, and
// routinely shared. So redact wholesale at the client, unconditionally.
//
// The keys survive because they are the useful part: they say how a model is configured ("this
// one authenticates with OPENAI_API_KEY") without saying what the secret is, which is enough to
// verify a model's shape. This mirrors what the Ansible collection returns from
// netorca_llm_model_info.
//
// The receiver is not modified, and the returned model owns a freshly allocated ExtraData map,
// so a later write to either copy cannot leak into the other.
func (m LLMModel) Redacted() LLMModel {
	if m.ExtraData == nil {
		return m
	}

	// A value receiver copies the map header, not the map, so the copy has to be explicit:
	// assigning into m.ExtraData would otherwise overwrite the caller's own credentials.
	redacted := make(map[string]any, len(m.ExtraData))
	for key := range m.ExtraData {
		redacted[key] = redactedPlaceholder
	}
	m.ExtraData = redacted
	return m
}

// ListLLMModelsRequest paginates a catalogue listing. Every field is optional; the zero value
// asks for the server's default page.
//
// There are no field filters. The platform registers no filterset for this route, so name,
// provider and is_active cannot be narrowed server-side; selecting on those is a client-side
// scan, which is what FindLLMModelByName and FirstActiveLLMModel do on your behalf.
type ListLLMModelsRequest struct {
	// Limit caps the number of results returned. Zero leaves the server's page size in place.
	Limit int
	// Offset skips this many results.
	Offset int
	// Ordering names the field to sort by; prefix with "-" to reverse. The catalogue has no
	// default order, so set this whenever you page: without it the server may hand back
	// overlapping pages and a scan can miss a model entirely.
	Ordering string
}

// ToQueryParams renders the filters as a percent-encoded query string, "" when empty.
//
// The error is always nil. It is part of the signature so this request matches its siblings,
// whose structured filters genuinely can fail to encode.
func (r *ListLLMModelsRequest) ToQueryParams() (string, error) {
	params := newQueryParams()
	params.SetInt("limit", r.Limit)
	params.SetInt("offset", r.Offset)
	params.SetString("ordering", r.Ordering)
	return params.Encode(), nil
}

// ListLLMModelsResponse is the paginated envelope the API returns for a catalogue listing.
type ListLLMModelsResponse struct {
	// Count is the total number of models in the catalogue, across all pages.
	Count int `json:"count"`
	// Next is the URL of the next page, nil on the last page.
	Next *string `json:"next"`
	// Previous is the URL of the previous page, nil on the first page.
	Previous *string `json:"previous"`
	// Results is this page of models.
	Results []LLMModel `json:"results"`
}

// ListLLMModels returns a page of the platform's LLM catalogue - the models an AI processor
// can be pointed at, and the prices its pipeline runs are billed against.
//
// It takes no POV. The catalogue is platform-global, the same for every team, so this and
// GetLLMModel are the only routes in the API whose paths carry no point-of-view segment.
//
// Results carry provider connection details in ExtraData; call Redacted on anything you are
// about to persist or print.
func (c *Client) ListLLMModels(
	ctx context.Context,
	filters *ListLLMModelsRequest,
) (*ListLLMModelsResponse, error) {
	if filters == nil {
		filters = &ListLLMModelsRequest{}
	}

	query, err := filters.ToQueryParams()
	if err != nil {
		return nil, fmt.Errorf("failed to convert filters to query params: %w", err)
	}

	endpoint := "ai/llm_models/" + query

	var response ListLLMModelsResponse
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetLLMModel returns a single model from the catalogue by id - the call that turns an AI
// processor's llm_model reference into the model it actually runs on.
//
// It takes no POV, for the reason given on ListLLMModels. It returns an error wrapping
// ErrNotFound when no such model exists.
func (c *Client) GetLLMModel(ctx context.Context, id int) (*LLMModel, error) {
	endpoint := fmt.Sprintf("ai/llm_models/%d/", id)

	var response LLMModel
	if err := c.doRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// llmModelScanPageSize is how many models each page of a catalogue scan asks for. A platform
// runs a handful of models, so a scan is normally one request; the size is generous enough that
// paging a second time is the exception rather than the rule.
const llmModelScanPageSize = 100

// scanLLMModels walks the catalogue and returns the first model satisfying match, or nil when
// none does. It pages rather than reading a single response, because the route accepts no
// filters to push the predicate into and the server's default page is smaller than a catalogue
// may be - a single-page scan would report "no such model" for anything past the first page.
//
// It orders by id so paging is stable: the catalogue has no default order, and an unordered
// paginated queryset can repeat or skip rows between pages.
func (c *Client) scanLLMModels(
	ctx context.Context,
	match func(LLMModel) bool,
) (*LLMModel, error) {
	offset := 0
	for {
		page, err := c.ListLLMModels(ctx, &ListLLMModelsRequest{
			Limit:    llmModelScanPageSize,
			Offset:   offset,
			Ordering: "id",
		})
		if err != nil {
			return nil, err
		}

		for _, model := range page.Results {
			if match(model) {
				// Copy out of the loop variable: returning its address would hand the
				// caller a pointer into this iteration rather than into the match.
				found := model
				return &found, nil
			}
		}

		// Stop on the last page. The empty-page check is belt and braces against a server
		// that keeps offering a next page, so a scan cannot spin forever.
		if page.Next == nil || len(page.Results) == 0 {
			return nil, nil
		}

		// Advance by what arrived rather than by the requested limit, so a short page
		// cannot make the next request skip records.
		offset += len(page.Results)
	}
}

// FindLLMModelByName returns the catalogue entry with exactly this name.
//
// Resolving a name to an id is the most common thing a caller does with the catalogue: an AI
// processor references its model by id, but configuration - a playbook, a Terraform resource -
// names it. The match is exact and case-sensitive, so configuration keyed on a name cannot
// quietly rebind to a different model once an administrator adds a similarly named one.
//
// Naming a model returns it whether or not it is active; check IsActive before running on it.
// It returns an error wrapping ErrNotFound when no model carries that name.
func (c *Client) FindLLMModelByName(ctx context.Context, name string) (*LLMModel, error) {
	model, err := c.scanLLMModels(ctx, func(candidate LLMModel) bool {
		return candidate.Name == name
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search the LLM catalogue for %q: %w", name, err)
	}
	if model == nil {
		return nil, fmt.Errorf("%w: no LLM model named %q", ErrNotFound, name)
	}
	return model, nil
}

// FirstActiveLLMModel returns the first active model in the catalogue.
//
// It is the documented fallback for a caller that needs a model but has not been told which
// one: an AI processor must reference something, and on a platform with a single configured
// model that is the only sensible choice. Prefer FindLLMModelByName whenever a name is
// available - binding to "whichever model sorts first" produces configuration that changes
// meaning the day an administrator adds a model.
//
// "First" means lowest id, which is stable across calls. It returns an error wrapping
// ErrNotFound when the catalogue holds no active model.
func (c *Client) FirstActiveLLMModel(ctx context.Context) (*LLMModel, error) {
	model, err := c.scanLLMModels(ctx, func(candidate LLMModel) bool {
		return candidate.IsActive
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search the LLM catalogue for an active model: %w", err)
	}
	if model == nil {
		return nil, fmt.Errorf("%w: the LLM catalogue has no active model", ErrNotFound)
	}
	return model, nil
}
