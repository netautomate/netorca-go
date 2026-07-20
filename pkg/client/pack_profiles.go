package client

import (
	"context"
	"encoding/json"
	"fmt"
)

// VectorQueryConfig narrows what the pack retrieval layer sees of a service item declaration.
//
// Both lists name top-level fields of the service's declaration schema. ExcludeFields drops
// fields from the retrieval query entirely - the place for noisy or sensitive values that would
// only dilute a similarity match. ExactSearch promotes fields to literal matching instead of
// vector similarity, which is what identifiers and enumerations want: "prod" and "preprod" embed
// close together but must never match each other.
//
// The platform validates both lists against the service's schema and rejects a name that is not
// in it, or that appears in both lists, with ErrBadRequest. APIError.Body then carries the set of
// field names it would have accepted, which is the fastest way to find the typo.
type VectorQueryConfig struct {
	// ExcludeFields names declaration fields to drop from the retrieval query.
	ExcludeFields []string `json:"exclude_fields,omitempty"`
	// ExactSearch names declaration fields to match literally rather than by similarity.
	// A field named here must not also appear in ExcludeFields.
	ExactSearch []string `json:"exact_search,omitempty"`
}

// PackProfile is the per-service tuning record for the pack framework - one profile per service,
// holding how declarations are chunked, how context is retrieved, and whether pack runs at all.
//
// PackEnabled is the profile's master switch: it is the field to reach for when a service should
// be kept out of the pack framework. The platform default is true, so a service with no profile
// at all is not opted out - only an explicit false opts it out.
//
// Every tunable is a pointer because the platform, not this struct, owns the defaults. Nil means
// "the API sent no value", which is a different fact from a deliberate zero: recording 0 for a
// field the server never mentioned would read back as permanent drift against the platform
// default. The same pointers keep unset fields off the wire entirely on a write, so a partial
// update cannot silently reset a tunable somebody else configured.
type PackProfile struct {
	// ID is the unique identifier for the profile.
	ID int `json:"id"`
	// Service is the service this profile tunes. The relation is one-to-one, so this also
	// identifies the profile: see FindPackProfile.
	Service RefID `json:"service"`
	// ChunkOverlap is how many units of context adjacent chunks share, so that a match
	// straddling a chunk boundary is still retrievable. Platform default 0.
	ChunkOverlap *int `json:"chunk_overlap,omitempty"`
	// MaxLines caps the number of lines in one chunk. Platform default 10.
	MaxLines *int `json:"max_lines,omitempty"`
	// MaxChars caps the number of characters in one chunk. Platform default 256.
	MaxChars *int `json:"max_chars,omitempty"`
	// TopK is how many chunks the retriever returns for a query - the main lever on how much
	// context reaches the LLM, and therefore on cost. Platform default 10.
	TopK *int `json:"top_k,omitempty"`
	// ReturnAllDocuments bypasses retrieval and feeds every enabled document to the processor.
	// Useful for a small, curated document set; expensive for anything else. Default false.
	ReturnAllDocuments *bool `json:"return_all_documents,omitempty"`
	// CosineSimilarityThreshold is the minimum similarity a chunk must reach to be considered
	// relevant. Raising it trades recall for precision. Platform default 0.8.
	CosineSimilarityThreshold *float64 `json:"cosine_similarity_threshold,omitempty"`
	// QueryConfig narrows which declaration fields take part in retrieval. The API renders an
	// unconfigured profile as an empty object rather than null.
	QueryConfig *VectorQueryConfig `json:"query_config,omitempty"`
	// EmbeddingModel names the sentence-transformers model used to embed chunks, at most 50
	// characters. Platform default "all-MiniLM-L6-v2". Unlike the numeric tunables this needs
	// no pointer: the API rejects an empty string, so "" already means "unset" unambiguously.
	EmbeddingModel string `json:"embedding_model,omitempty"`
	// PackEnabled is the master switch described above. Platform default true.
	PackEnabled *bool `json:"pack_enabled,omitempty"`
	// UniversalExecutorEnabled switches the service onto the platform's shared executor
	// prompts and response schema instead of the service owner's own. With it on, the platform
	// refuses a custom response_schema on the service's AI processors, so turning it on can
	// invalidate an AIProcessor write that used to succeed. Platform default false.
	UniversalExecutorEnabled *bool `json:"universal_executor_enabled,omitempty"`
}

// PackProfileWrite is the request body for CreatePackProfile.
//
// Service is the only required field; leave a tunable nil and the platform default applies. The
// pointers are load-bearing rather than stylistic - a nil field is omitted from the JSON, whereas
// a plain int would serialise as 0 and clobber the default it was meant to leave alone.
type PackProfileWrite struct {
	// Service is the id of the service to tune. Sent as a bare integer; note that the
	// corresponding list filter is named service_id, not service.
	Service RefID `json:"service"`
	// ChunkOverlap is how many units of context adjacent chunks share. Nil keeps the default.
	ChunkOverlap *int `json:"chunk_overlap,omitempty"`
	// MaxLines caps the number of lines in one chunk. Nil keeps the default.
	MaxLines *int `json:"max_lines,omitempty"`
	// MaxChars caps the number of characters in one chunk. Nil keeps the default.
	MaxChars *int `json:"max_chars,omitempty"`
	// TopK is how many chunks the retriever returns. Nil keeps the default.
	TopK *int `json:"top_k,omitempty"`
	// ReturnAllDocuments bypasses retrieval entirely. Nil keeps the default.
	ReturnAllDocuments *bool `json:"return_all_documents,omitempty"`
	// CosineSimilarityThreshold is the minimum similarity for relevance. Nil keeps the default.
	CosineSimilarityThreshold *float64 `json:"cosine_similarity_threshold,omitempty"`
	// QueryConfig narrows which declaration fields take part in retrieval. Nil keeps the
	// default; a non-nil pointer to a zero value clears the configuration.
	QueryConfig *VectorQueryConfig `json:"query_config,omitempty"`
	// EmbeddingModel names the embedding model. Empty keeps the default.
	EmbeddingModel string `json:"embedding_model,omitempty"`
	// PackEnabled is the master switch. Nil keeps the default, which is true.
	PackEnabled *bool `json:"pack_enabled,omitempty"`
	// UniversalExecutorEnabled switches the service onto the shared executor prompts.
	// Nil keeps the default, which is false.
	UniversalExecutorEnabled *bool `json:"universal_executor_enabled,omitempty"`
}

// ListPackProfilesRequest filters a pack profile listing. Every field is optional; the zero value
// lists every profile the API key can see.
//
// The filter set really is this small. The platform rejects any query parameter it does not
// recognise with ErrBadRequest ("Filter param not found."), so there is no undocumented filter to
// reach for - to find a profile by service, use ServiceID, or FindPackProfile for the common case.
type ListPackProfilesRequest struct {
	// POV is the point of view to query from. Defaults to serviceowner.
	POV POV
	// ServiceID restricts results to the profiles of these services.
	ServiceID []int
	// Limit caps the number of results returned. Zero means the platform's page size.
	Limit int
	// Offset skips this many results.
	Offset int
	// Ordering names the field to sort by; prefix with "-" to reverse.
	Ordering string
}

// ToQueryParams renders the filters as a percent-encoded query string, "" when empty.
func (r *ListPackProfilesRequest) ToQueryParams() (string, error) {
	params := newQueryParams()
	params.SetInts("service_id", r.ServiceID)
	params.SetInt("limit", r.Limit)
	params.SetInt("offset", r.Offset)
	params.SetString("ordering", r.Ordering)

	// No filter here can fail to encode; the error is part of the signature so this request
	// stays interchangeable with the other list requests, which have structured filters.
	return params.Encode(), nil
}

// ListPackProfilesResponse is the paginated envelope the API returns for a profile listing.
type ListPackProfilesResponse struct {
	// Count is the total number of matching profiles, across all pages.
	Count int `json:"count"`
	// Next is the URL of the next page, nil on the last page.
	Next *string `json:"next"`
	// Previous is the URL of the previous page, nil on the first page.
	Previous *string `json:"previous"`
	// Results is this page of profiles.
	Results []PackProfile `json:"results"`
}

// packProfilesPath builds the pack profile collection route, with the given query appended.
//
// Profiles live under the "ai/" prefix rather than the "external/" one the rest of the pack
// surface uses, because they are served by a different Django app. Getting this wrong yields an
// unexplained 404, so both helpers exist to keep the prefix in one place.
func packProfilesPath(pov POV, query string) string {
	return fmt.Sprintf("ai/%s/pack/profiles/%s", pov.orDefault(), query)
}

// packProfilePath builds the detail route for one profile. The trailing slash is the canonical
// DRF route; without it the API replies 301 to the slashed URL.
func packProfilePath(pov POV, id int) string {
	return fmt.Sprintf("ai/%s/pack/profiles/%d/", pov.orDefault(), id)
}

// ListPackProfiles returns the pack profiles matching the given filters. Pass nil for no filters.
//
// Only services somebody has configured have a profile row, so this lists the services that have
// been tuned, not every service the key can see.
func (c *Client) ListPackProfiles(
	ctx context.Context,
	filters *ListPackProfilesRequest,
) (*ListPackProfilesResponse, error) {
	if filters == nil {
		filters = &ListPackProfilesRequest{}
	}

	query, err := filters.ToQueryParams()
	if err != nil {
		return nil, fmt.Errorf("failed to convert filters to query params: %w", err)
	}

	var response ListPackProfilesResponse
	if err := c.doRequest(ctx, "GET", packProfilesPath(filters.POV, query), nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetPackProfile returns a single pack profile by id.
// It returns an error wrapping ErrNotFound when no such profile exists.
func (c *Client) GetPackProfile(ctx context.Context, pov POV, id int) (*PackProfile, error) {
	var response PackProfile
	if err := c.doRequest(ctx, "GET", packProfilePath(pov, id), nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// FindPackProfile returns the pack profile for a service, looked up by service id rather than by
// profile id.
//
// A service has at most one profile, so this is the natural way in for anything that knows which
// service it is configuring but not which profile row that became - a Terraform provider reading
// back a resource it created, for instance. It returns an error wrapping ErrNotFound when the
// service has no profile yet, which callers should read as "not configured" rather than as a
// failure: an unconfigured service runs on the platform defaults.
//
// This deliberately lists rather than reading the platform's resolved-config route, because that
// route materialises a default profile row as a side effect of being read. A read that creates
// rows cannot be used by anything that reconciles state.
func (c *Client) FindPackProfile(ctx context.Context, pov POV, serviceID int) (*PackProfile, error) {
	if serviceID <= 0 {
		return nil, fmt.Errorf("service id must be positive, got %d", serviceID)
	}

	response, err := c.ListPackProfiles(ctx, &ListPackProfilesRequest{
		POV:       pov,
		ServiceID: []int{serviceID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to look up the pack profile for service %d: %w", serviceID, err)
	}

	switch len(response.Results) {
	case 0:
		return nil, fmt.Errorf("%w: no pack profile for service %d", ErrNotFound, serviceID)
	case 1:
		profile := response.Results[0]
		return &profile, nil
	}

	// The service-to-profile relation is one-to-one, so this cannot happen against a healthy
	// platform. Report it rather than picking one arbitrarily: a caller that reconciles state
	// would otherwise flap between two rows on successive reads.
	return nil, fmt.Errorf(
		"expected at most one pack profile for service %d, got %d",
		serviceID, len(response.Results),
	)
}

// packProfileConfigurePath builds the service-scoped configure route, which is the only write
// path the platform actually serves for pack profiles - see UpsertPackProfileForService.
func packProfileConfigurePath(pov POV, serviceID int) string {
	return fmt.Sprintf("ai/%s/pack/profiles/service/%d/", pov.orDefault(), serviceID)
}

// UpsertPackProfileForService creates or updates a service's pack profile in a single call, and
// is the way to write one. The patch holds JSON field names - "pack_enabled", "top_k",
// "query_config" and so on; fields it does not name keep their stored values.
//
// It targets the platform's service-scoped configure route rather than the profile's own detail
// route, for two reasons.
//
// The first is that the collection and detail write routes do not work: POST to the collection
// and PATCH or PUT to a profile's detail route all answer 500 with a Django error page rather
// than a DRF response, whatever the body - including a body that only sets pack_enabled. The
// configure route is the only write path that succeeds, and is what the platform's own GUI uses.
//
// The second is that upsert is the semantics a caller actually wants. A service has at most one
// profile, and the platform materialises a default one as a side effect of anything reading the
// service's resolved config - so "create" and "update" are not reliably distinguishable from the
// outside, and a caller reconciling desired state should not have to care which happened.
func (c *Client) UpsertPackProfileForService(
	ctx context.Context,
	pov POV,
	serviceID int,
	patch map[string]any,
) (*PackProfile, error) {
	if serviceID <= 0 {
		return nil, fmt.Errorf("pack profile requires a service id, got %d", serviceID)
	}

	// The route resolves the service itself, so "service" in the body would be redundant at
	// best and contradictory at worst.
	body := make(map[string]any, len(patch))
	for key, value := range patch {
		if key == "service" {
			continue
		}
		body[key] = value
	}

	var response PackProfile
	if err := c.doRequest(ctx, "PATCH", packProfileConfigurePath(pov, serviceID), body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// CreatePackProfile creates the pack profile for a service and returns it as stored.
//
// A service can have only one profile, so this is an upsert in practice: it delegates to
// UpsertPackProfileForService, and calling it for a service that already has a profile updates
// that profile rather than failing.
func (c *Client) CreatePackProfile(ctx context.Context, pov POV, body *PackProfileWrite) (*PackProfile, error) {
	if body == nil {
		return nil, fmt.Errorf("pack profile body cannot be nil")
	}
	if body.Service <= 0 {
		return nil, fmt.Errorf("pack profile requires a service id, got %d", body.Service.Int())
	}

	patch, err := packProfileWriteToPatch(body)
	if err != nil {
		return nil, err
	}
	return c.UpsertPackProfileForService(ctx, pov, body.Service.Int(), patch)
}

// packProfileWriteToPatch turns a write struct into the field map the configure route takes,
// preserving the struct's omitempty behaviour so unset tunables stay unset.
func packProfileWriteToPatch(body *PackProfileWrite) (map[string]any, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode pack profile body: %w", err)
	}

	var patch map[string]any
	if err := json.Unmarshal(encoded, &patch); err != nil {
		return nil, fmt.Errorf("failed to decode pack profile body: %w", err)
	}
	delete(patch, "service")
	return patch, nil
}

// UpdatePackProfile applies a partial update to a pack profile and returns it as stored.
//
// The patch is a map rather than a struct so the caller decides exactly which keys go on the
// wire: the API changes only the fields present in the body and leaves the rest alone, and that
// is the whole point of the call. Keys are the JSON field names - "top_k", "pack_enabled",
// "query_config" and so on. An empty patch is rejected rather than sent, since it would cost a
// round trip to change nothing.
//
//	profile, err := c.UpdatePackProfile(ctx, client.POVServiceOwner, 7, map[string]any{
//		"pack_enabled": false,
//	})
//
// It costs one extra GET, because the write it delegates to is keyed on the service rather than
// the profile and the profile's own detail route cannot be written to. Where the service id is
// already known, call UpsertPackProfileForService directly and skip the lookup.
func (c *Client) UpdatePackProfile(
	ctx context.Context,
	pov POV,
	id int,
	patch map[string]any,
) (*PackProfile, error) {
	if len(patch) == 0 {
		return nil, fmt.Errorf("pack profile patch cannot be empty")
	}

	profile, err := c.GetPackProfile(ctx, pov, id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve the service of pack profile %d: %w", id, err)
	}

	return c.UpsertPackProfileForService(ctx, pov, profile.Service.Int(), patch)
}

// DeletePackProfile removes a pack profile, reverting its service to the platform defaults.
//
// This does not disable pack for the service, and is not the opposite of enabling it. The
// defaults it reverts to are permissive - PackEnabled defaults to true - so deleting a profile
// that had PackEnabled false switches pack back on for that service. To keep a service out of the
// pack framework, keep its profile and patch pack_enabled to false instead.
//
// It returns an error wrapping ErrNotFound when the profile is already gone, which a caller
// reconciling state can treat as success.
func (c *Client) DeletePackProfile(ctx context.Context, pov POV, id int) error {
	return c.doRequest(ctx, "DELETE", packProfilePath(pov, id), nil, nil)
}
