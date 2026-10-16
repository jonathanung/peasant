package village

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/peasant-labs/schema"
)

// ReadObservationMemberships reads the complete membership input from the
// canonical route, without changing the existing live suggestion API.
func (c *VillageClient) ReadObservationMemberships(ctx context.Context) ([]schema.VillageUserGroup, error) {
	var rows []schema.VillageUserGroup
	if err := c.readObservationJSON(ctx, "read complete collective memberships", "/api/v1/collectives", &rows); err != nil {
		return nil, err
	}
	if rows == nil {
		return nil, fmt.Errorf("read complete collective memberships: missing or null array; no membership reasons can be replaced; Village must emit the complete array before retrying")
	}
	seen := make(map[schema.VillageUUID]bool, len(rows))
	for _, row := range rows {
		if !IsCollectiveID(row.ID) || seen[row.ID] {
			return nil, fmt.Errorf("read complete collective memberships: invalid or repeated collective ID; no membership reasons can be replaced; correct the Village response and retry")
		}
		if row.Name == "" || row.MemberSince.IsZero() || !slices.Contains(schema.AllVillageGroupRoles, row.Role) {
			return nil, fmt.Errorf("read complete collective memberships: missing name/member time or unknown role; no membership reasons can be replaced; correct the Village response and retry")
		}
		if err := validateObservationPolicy(row.AcceptanceMode, row.DataAccess); err != nil {
			return nil, err
		}
		seen[row.ID] = true
	}
	return rows, nil
}

// ReadObservationCollective returns only the permitted collective projection.
// Rosters, transcript lists and organization members from the detail response
// are transport-local and never become observation-cache discovery inputs.
func (c *VillageClient) ReadObservationCollective(ctx context.Context, id schema.VillageUUID) (schema.VillageGroupDetailRecord, error) {
	if !IsCollectiveID(id) {
		return schema.VillageGroupDetailRecord{}, fmt.Errorf("read collective metadata: invalid collective UUID; no request sent; resolve an exact canonical collective ID and retry")
	}
	var response schema.VillageGroupDetailResponse
	path := "/api/v1/collectives/" + url.PathEscape(id.String())
	if err := c.readObservationJSON(ctx, "read visible collective metadata", path, &response); err != nil {
		return schema.VillageGroupDetailRecord{}, err
	}
	row := response.Group
	if row.ID != id || row.Name == "" {
		return schema.VillageGroupDetailRecord{}, fmt.Errorf("read visible collective metadata: missing metadata or wrong collective identity; no metadata can be installed; correct the Village response and retry")
	}
	if err := validateObservationPolicy(row.AcceptanceMode, row.DataAccess); err != nil {
		return schema.VillageGroupDetailRecord{}, err
	}
	return row, nil
}

func validateObservationPolicy(acceptance schema.VillageGroupAcceptanceMode, access schema.VillageGroupDataAccess) error {
	if !slices.Contains(schema.AllVillageGroupAcceptanceModes, acceptance) || !slices.Contains(schema.AllVillageGroupDataAccessPolicies, access) {
		return fmt.Errorf("read collective metadata: unknown acceptance or data-access policy; no policy observation can be installed; update the client or correct the Village response before retrying")
	}
	return nil
}

// ReadOwnShareEvents reads a complete owner-only pair history. The schema owns
// individual attempt validation and all mutable-attempt transition semantics.
// A 403/404 is returned unchanged for explicit unavailable coverage, never []
// that would imply the owner's retained business history had become empty.
func (c *VillageClient) ReadOwnShareEvents(ctx context.Context, id schema.TranscriptID, collective schema.VillageUUID) ([]schema.VillageShareEvent, error) {
	if _, err := schema.NewTranscriptID(id.String()); err != nil {
		return nil, fmt.Errorf("read own pair history: invalid transcript ID: %w; no request sent; resolve its canonical ID and retry", err)
	}
	if !IsCollectiveID(collective) {
		return nil, fmt.Errorf("read own pair history: invalid collective UUID; no request sent; resolve its canonical ID and retry")
	}
	var events []schema.VillageShareEvent
	if err := c.readObservationJSON(ctx, "read complete own contribution attempts", shareEventsPath(id, collective), &events); err != nil {
		return nil, err
	}
	if events == nil {
		return nil, fmt.Errorf("read own pair history: missing or null complete array; retained history is unchanged; Village must emit an explicit complete array before retrying")
	}
	seen := make(map[int32]bool, len(events))
	for _, event := range events {
		if err := event.Validate(); err != nil {
			return nil, fmt.Errorf("read own pair history: %w", err)
		}
		if seen[event.EventNum] {
			return nil, fmt.Errorf("read own pair history: repeated attempt ordinal in one complete pair response; no prefix can be installed; correct the Village history response and retry")
		}
		seen[event.EventNum] = true
	}
	return events, nil
}

// ReadObservationOrganizations reads the caller's complete organization input.
// The separate permitted linked-collective projection is still required before
// this input can establish complete organization-derived catalog relevance.
func (c *VillageClient) ReadObservationOrganizations(ctx context.Context) ([]schema.VillageMetadataUserOrganization, error) {
	var rows []schema.VillageMetadataUserOrganization
	if err := c.readObservationJSON(ctx, "read selected account organizations", "/api/v1/auth/orgs", &rows); err != nil {
		return nil, err
	}
	if rows == nil {
		return nil, fmt.Errorf("read selected account organizations: missing or null complete array; organization relevance cannot be replaced; Village must emit the complete array before retrying")
	}
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		login := strings.ToLower(row.OrgLogin)
		invalidIdentity := login == "" || row.OrgID < 1 || seen[login]
		if invalidIdentity || row.FetchedAt.IsZero() {
			return nil, fmt.Errorf("read selected account organizations: missing or repeated organization identity or observation time; organization relevance cannot be replaced; correct the Village response and retry")
		}
		seen[login] = true
	}
	return rows, nil
}
