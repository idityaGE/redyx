// Package search implements the search-service backend: Meilisearch integration,
// Kafka post-event indexing, and gRPC SearchService RPCs.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"go.uber.org/zap"
)

// SearchResult holds a single search hit, mapped from Meilisearch response.
type SearchResult struct {
	PostID         string
	Title          string
	Snippet        string
	AuthorUsername string
	CommunityName  string
	VoteScore      int32
	CommentCount   int32
	CreatedAt      time.Time
}

// MeiliClient wraps a Meilisearch ServiceManager and provides search index operations.
type MeiliClient struct {
	client meilisearch.ServiceManager
	logger *zap.Logger
}

// NewMeiliClient creates a Meilisearch client, configures the "posts" index with
// searchable, filterable, sortable attributes and ranking rules.
func NewMeiliClient(url, apiKey string, logger *zap.Logger) (*MeiliClient, error) {
	client := meilisearch.New(url, meilisearch.WithAPIKey(apiKey))

	mc := &MeiliClient{
		client: client,
		logger: logger,
	}

	if err := mc.configurePostsIndex(); err != nil {
		return nil, fmt.Errorf("configure posts index: %w", err)
	}

	return mc, nil
}

// configurePostsIndex ensures the "posts" index exists and has the correct settings.
func (m *MeiliClient) configurePostsIndex() error {
	idx := m.client.Index("posts")

	_, err := idx.UpdateSettings(&meilisearch.Settings{
		SearchableAttributes: []string{"title", "body"},
		FilterableAttributes: []string{"communityName"},
		SortableAttributes:   []string{"createdAt", "voteScore"},
		RankingRules: []string{
			"words",
			"typo",
			"proximity",
			"attribute",
			"sort",
			"exactness",
		},
	})
	if err != nil {
		return fmt.Errorf("update posts index settings: %w", err)
	}

	m.logger.Info("configured posts index settings")
	return nil
}

// ConfigureCommunitiesIndex ensures the "communities" index exists and has the correct settings.
func (m *MeiliClient) ConfigureCommunitiesIndex() error {
	idx := m.client.Index("communities")

	_, err := idx.UpdateSettings(&meilisearch.Settings{
		SearchableAttributes: []string{"name"},
		FilterableAttributes: []string{"name"},
		SortableAttributes:   []string{"memberCount"},
	})
	if err != nil {
		return fmt.Errorf("update communities index settings: %w", err)
	}

	m.logger.Info("configured communities index settings")
	return nil
}

// Search queries the "posts" index with optional community scoping and sort.
// Returns matching results, total hit count, and any error.
func (m *MeiliClient) Search(_ context.Context, query, communityName string, sort string, limit, offset int) ([]SearchResult, int64, error) {
	idx := m.client.Index("posts")

	req := &meilisearch.SearchRequest{
		Offset:                int64(offset),
		Limit:                 int64(limit),
		AttributesToHighlight: []string{"title", "body"},
		HighlightPreTag:       "<em>",
		HighlightPostTag:      "</em>",
		AttributesToCrop:      []string{"body:80"},
	}

	// Apply community filter if specified.
	if communityName != "" {
		req.Filter = fmt.Sprintf("communityName = '%s'", communityName)
	}

	// Apply sort if specified.
	if sort != "" {
		req.Sort = []string{sort}
	}

	resp, err := idx.Search(query, req)
	if err != nil {
		return nil, 0, fmt.Errorf("meilisearch search: %w", err)
	}

	results := make([]SearchResult, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		sr, err := parseSearchHit(hit)
		if err != nil {
			m.logger.Warn("failed to parse search hit", zap.Error(err))
			continue
		}
		results = append(results, sr)
	}

	return results, resp.EstimatedTotalHits, nil
}

// SearchCommunities queries the "communities" index for community name prefix matching.
func (m *MeiliClient) SearchCommunities(_ context.Context, query string, limit int) ([]CommunitySuggestion, error) {
	idx := m.client.Index("communities")

	req := &meilisearch.SearchRequest{
		Limit: int64(limit),
	}

	resp, err := idx.Search(query, req)
	if err != nil {
		return nil, fmt.Errorf("meilisearch community search: %w", err)
	}

	results := make([]CommunitySuggestion, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		cs, err := parseCommunityHit(hit)
		if err != nil {
			m.logger.Warn("failed to parse community hit", zap.Error(err))
			continue
		}
		results = append(results, cs)
	}

	return results, nil
}

// CommunitySuggestion holds a single community autocomplete result.
type CommunitySuggestion struct {
	Name        string
	IconURL     string
	MemberCount int32
}

// IndexPost adds or updates a post document in the "posts" index.
func (m *MeiliClient) IndexPost(postID, title, body, authorUsername, communityName string, voteScore, commentCount int32, createdAt int64) error {
	idx := m.client.Index("posts")

	doc := map[string]interface{}{
		"id":             postID,
		"title":          title,
		"body":           body,
		"authorUsername": authorUsername,
		"communityName":  communityName,
		"voteScore":      voteScore,
		"commentCount":   commentCount,
		"createdAt":      createdAt,
	}

	_, err := idx.AddDocuments([]map[string]interface{}{doc}, nil)
	if err != nil {
		return fmt.Errorf("index post %s: %w", postID, err)
	}

	return nil
}

// IndexCommunity adds or updates a community document in the "communities" index.
func (m *MeiliClient) IndexCommunity(name, iconURL string, memberCount int32) error {
	idx := m.client.Index("communities")

	doc := map[string]interface{}{
		"id":          name,
		"name":        name,
		"iconUrl":     iconURL,
		"memberCount": memberCount,
	}

	_, err := idx.AddDocuments([]map[string]interface{}{doc}, nil)
	if err != nil {
		return fmt.Errorf("index community %s: %w", name, err)
	}

	return nil
}

// DeletePost removes a post document from the "posts" index.
func (m *MeiliClient) DeletePost(postID string) error {
	idx := m.client.Index("posts")

	_, err := idx.DeleteDocument(postID, nil)
	if err != nil {
		return fmt.Errorf("delete post %s: %w", postID, err)
	}

	return nil
}

// parseSearchHit extracts a SearchResult from a Meilisearch hit.
// It reads highlighted/cropped values from the _formatted field when available.
func parseSearchHit(hit meilisearch.Hit) (SearchResult, error) {
	var sr SearchResult

	sr.PostID = rawString(hit, "id")
	sr.AuthorUsername = rawString(hit, "authorUsername")
	sr.CommunityName = rawString(hit, "communityName")
	sr.VoteScore = rawInt32(hit, "voteScore")
	sr.CommentCount = rawInt32(hit, "commentCount")

	// Parse createdAt as unix timestamp.
	if raw, ok := hit["createdAt"]; ok {
		var ts int64
		if err := json.Unmarshal(raw, &ts); err == nil {
			sr.CreatedAt = time.Unix(ts, 0)
		}
	}

	// Use _formatted for highlighted title and snippet.
	if formatted, ok := hit["_formatted"]; ok {
		var fm map[string]json.RawMessage
		if err := json.Unmarshal(formatted, &fm); err == nil {
			sr.Title = rawStringFromMap(fm, "title")
			sr.Snippet = rawStringFromMap(fm, "body")
		}
	}

	// Fall back to raw title if _formatted not available.
	if sr.Title == "" {
		sr.Title = rawString(hit, "title")
	}
	if sr.Snippet == "" {
		sr.Snippet = rawString(hit, "body")
	}

	return sr, nil
}

// parseCommunityHit extracts a CommunitySuggestion from a Meilisearch hit.
func parseCommunityHit(hit meilisearch.Hit) (CommunitySuggestion, error) {
	var cs CommunitySuggestion
	cs.Name = rawString(hit, "name")
	cs.IconURL = rawString(hit, "iconUrl")
	cs.MemberCount = rawInt32(hit, "memberCount")
	return cs, nil
}

// rawString extracts a string value from a Hit's json.RawMessage map.
func rawString(hit meilisearch.Hit, key string) string {
	raw, ok := hit[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		// Try as number (e.g., id might be numeric).
		return string(raw)
	}
	return s
}

// rawStringFromMap extracts a string value from a map of json.RawMessage.
func rawStringFromMap(m map[string]json.RawMessage, key string) string {
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return string(raw)
	}
	return s
}

// rawInt32 extracts an int32 value from a Hit's json.RawMessage map.
func rawInt32(hit meilisearch.Hit, key string) int32 {
	raw, ok := hit[key]
	if !ok {
		return 0
	}
	// Try as number first.
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		if v, err := strconv.ParseInt(string(n), 10, 32); err == nil {
			return int32(v)
		}
	}
	return 0
}
