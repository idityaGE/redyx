package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/redyx/redyx/gen/redyx/common/v1"
	searchv1 "github.com/redyx/redyx/gen/redyx/search/v1"
)

// Server implements the SearchServiceServer gRPC interface.
type Server struct {
	searchv1.UnimplementedSearchServiceServer
	meili       *MeiliClient
	redisClient redis.Cmdable
	logger      *zap.Logger
}

// NewServer creates a new search gRPC server.
func NewServer(meili *MeiliClient, redisClient redis.Cmdable, logger *zap.Logger) *Server {
	return &Server{
		meili:       meili,
		redisClient: redisClient,
		logger:      logger,
	}
}

// SearchPosts searches posts by title and body text, optionally scoped to a community.
func (s *Server) SearchPosts(ctx context.Context, req *searchv1.SearchPostsRequest) (*searchv1.SearchPostsResponse, error) {
	query := strings.TrimSpace(req.GetQuery())
	if query == "" {
		return nil, status.Error(codes.InvalidArgument, "query must not be empty")
	}

	// Parse pagination.
	limit := int32(25)
	offset := int32(0)
	if pag := req.GetPagination(); pag != nil {
		if pag.GetLimit() > 0 {
			limit = pag.GetLimit()
		}
		// Use cursor as offset string for offset-based pagination.
		if pag.GetCursor() != "" {
			if off, err := parseInt32(pag.GetCursor()); err == nil {
				offset = off
			}
		}
	}

	// Cap limit at 100.
	if limit > 100 {
		limit = 100
	}

	// Default sort is relevance (empty string = Meilisearch default ranking).
	sort := ""

	results, totalHits, err := s.meili.Search(ctx, query, req.GetCommunityName(), sort, int(limit), int(offset))
	if err != nil {
		s.logger.Error("search failed", zap.String("query", query), zap.Error(err))
		return nil, status.Error(codes.Internal, "search failed")
	}

	// Map results to proto.
	protoResults := make([]*searchv1.SearchResult, 0, len(results))
	for _, r := range results {
		protoResults = append(protoResults, &searchv1.SearchResult{
			PostId:         r.PostID,
			Title:          r.Title,
			Snippet:        r.Snippet,
			AuthorUsername: r.AuthorUsername,
			CommunityName:  r.CommunityName,
			VoteScore:      r.VoteScore,
			CommentCount:   r.CommentCount,
			CreatedAt:      timestamppb.New(r.CreatedAt),
		})
	}

	// Build pagination response.
	nextOffset := offset + limit
	var nextCursor string
	if int64(nextOffset) < totalHits {
		nextCursor = fmt.Sprintf("%d", nextOffset)
	}

	return &searchv1.SearchPostsResponse{
		Results: protoResults,
		Pagination: &commonv1.PaginationResponse{
			NextCursor: nextCursor,
			HasMore:    int64(nextOffset) < totalHits,
		},
	}, nil
}

// AutocompleteCommunities provides prefix-based community name suggestions.
// Uses Redis ZRANGEBYLEX for fast prefix matching, with Meilisearch fallback.
func (s *Server) AutocompleteCommunities(ctx context.Context, req *searchv1.AutocompleteCommunitiesRequest) (*searchv1.AutocompleteCommunitiesResponse, error) {
	query := strings.TrimSpace(req.GetQuery())
	if len(query) < 2 {
		return nil, status.Error(codes.InvalidArgument, "query must be at least 2 characters")
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	// Try Redis prefix matching first.
	suggestions, err := s.autocompleteFromRedis(ctx, query, limit)
	if err != nil || len(suggestions) == 0 {
		if err != nil {
			s.logger.Warn("redis autocomplete failed, falling back to meilisearch", zap.Error(err))
		}
		// Fallback to Meilisearch.
		suggestions, err = s.autocompleteFromMeili(ctx, query, limit)
		if err != nil {
			s.logger.Error("meilisearch autocomplete failed", zap.Error(err))
			return nil, status.Error(codes.Internal, "autocomplete failed")
		}
	}

	// Map to proto.
	protoSuggestions := make([]*searchv1.CommunitySuggestion, 0, len(suggestions))
	for _, cs := range suggestions {
		protoSuggestions = append(protoSuggestions, &searchv1.CommunitySuggestion{
			Name:        cs.Name,
			IconUrl:     cs.IconURL,
			MemberCount: cs.MemberCount,
		})
	}

	return &searchv1.AutocompleteCommunitiesResponse{
		Suggestions: protoSuggestions,
	}, nil
}

// autocompleteFromRedis uses ZRANGEBYLEX on sorted set "communities:autocomplete"
// for fast prefix matching.
func (s *Server) autocompleteFromRedis(ctx context.Context, query string, limit int) ([]CommunitySuggestion, error) {
	if s.redisClient == nil {
		return nil, fmt.Errorf("redis not available")
	}

	// ZRANGEBYLEX for prefix matching: [query, query\xff)
	lowerQuery := strings.ToLower(query)
	min := "[" + lowerQuery
	max := "[" + lowerQuery + "\xff"

	names, err := s.redisClient.ZRangeByLex(ctx, "communities:autocomplete", &redis.ZRangeBy{
		Min:   min,
		Max:   max,
		Count: int64(limit),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("zrangebylex: %w", err)
	}

	suggestions := make([]CommunitySuggestion, 0, len(names))
	for _, name := range names {
		suggestions = append(suggestions, CommunitySuggestion{
			Name: name,
		})
	}

	return suggestions, nil
}

// autocompleteFromMeili falls back to Meilisearch communities index search.
func (s *Server) autocompleteFromMeili(ctx context.Context, query string, limit int) ([]CommunitySuggestion, error) {
	return s.meili.SearchCommunities(ctx, query, limit)
}

// parseInt32 parses a string to int32.
func parseInt32(s string) (int32, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	v := int32(0)
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number: %s", s)
		}
		v = v*10 + int32(c-'0')
	}
	return v, nil
}
