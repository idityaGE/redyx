package community

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/redyx/redyx/gen/redyx/common/v1"
	commv1 "github.com/redyx/redyx/gen/redyx/community/v1"
	"github.com/redyx/redyx/internal/platform/auth"
	perrors "github.com/redyx/redyx/internal/platform/errors"
	"github.com/redyx/redyx/internal/platform/pagination"
	"github.com/redyx/redyx/internal/platform/ratelimit"
)

var nameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,21}$`)

// Server implements the CommunityServiceServer gRPC interface.
type Server struct {
	commv1.UnimplementedCommunityServiceServer
	db      *pgxpool.Pool
	cache   *Cache
	limiter *ratelimit.Limiter
	logger  *zap.Logger
}

// NewServer creates a new community gRPC server.
func NewServer(db *pgxpool.Pool, cache *Cache, logger *zap.Logger, opts ...ServerOption) *Server {
	s := &Server{
		db:     db,
		cache:  cache,
		logger: logger,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ServerOption configures optional dependencies for the community server.
type ServerOption func(*Server)

// WithLimiter configures the rate limiter for action-specific rate limiting.
func WithLimiter(limiter *ratelimit.Limiter) ServerOption {
	return func(s *Server) {
		s.limiter = limiter
	}
}

// CreateCommunity creates a new community and assigns the caller as owner.
func (s *Server) CreateCommunity(ctx context.Context, req *commv1.CreateCommunityRequest) (*commv1.CreateCommunityResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("create community: %w", perrors.ErrUnauthenticated)
	}

	// Action-specific rate limit: 1 community per day
	if s.limiter != nil {
		key := fmt.Sprintf("action:community:%s", claims.UserID)
		cfg := ratelimit.ActionLimits["community"]
		result, err := s.limiter.Check(ctx, key, cfg.Limit, cfg.WindowSec)
		if err != nil {
			s.logger.Warn("rate limit check failed, allowing request (fail-open)",
				zap.String("user_id", claims.UserID),
				zap.Error(err),
			)
		} else if !result.Allowed {
			return nil, fmt.Errorf("rate limit exceeded: you can create %d community per day, retry after %v: %w",
				cfg.Limit, result.RetryAfter.Round(time.Second), perrors.ErrRateLimited)
		}
	}

	if !nameRegex.MatchString(req.GetName()) {
		return nil, fmt.Errorf("community name must be 3-21 alphanumeric or underscore characters: %w", perrors.ErrInvalidInput)
	}

	visibility := req.GetVisibility()
	if visibility == commv1.Visibility_VISIBILITY_UNSPECIFIED {
		visibility = commv1.Visibility_VISIBILITY_PUBLIC
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var communityID string
	var createdAt time.Time
	err = tx.QueryRow(ctx,
		`INSERT INTO communities (name, description, visibility, owner_id)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		req.GetName(), req.GetDescription(), int16(visibility), claims.UserID,
	).Scan(&communityID, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("insert community: %w", err)
	}

	// Insert creator as owner member
	_, err = tx.Exec(ctx,
		`INSERT INTO community_members (community_id, user_id, username, role)
		 VALUES ($1, $2, $3, 'owner')`,
		communityID, claims.UserID, claims.Username,
	)
	if err != nil {
		return nil, fmt.Errorf("insert owner member: %w", err)
	}

	// Set member count to 1
	_, err = tx.Exec(ctx,
		`UPDATE communities SET member_count = 1 WHERE id = $1`,
		communityID,
	)
	if err != nil {
		return nil, fmt.Errorf("update member count: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &commv1.CreateCommunityResponse{
		Community: &commv1.Community{
			CommunityId: communityID,
			Name:        req.GetName(),
			Description: req.GetDescription(),
			Visibility:  visibility,
			MemberCount: 1,
			OwnerId:     claims.UserID,
			CreatedAt:   timestamppb.New(createdAt),
		},
	}, nil
}

// GetCommunity returns a community by name, checking visibility and membership.
func (s *Server) GetCommunity(ctx context.Context, req *commv1.GetCommunityRequest) (*commv1.GetCommunityResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	name := req.GetName()

	// Check cache first
	cached, err := s.cache.Get(ctx, name)
	if err != nil {
		s.logger.Warn("cache get error", zap.Error(err))
	}

	var comm *commv1.Community
	if cached != nil {
		comm = cachedToProto(cached)
	} else {
		// Query DB
		comm, err = s.getCommunityFromDB(ctx, name)
		if err != nil {
			return nil, err
		}

		// Cache the result
		if cacheErr := s.cache.Set(ctx, name, protoToCached(comm)); cacheErr != nil {
			s.logger.Warn("cache set error", zap.Error(cacheErr))
		}
	}

	// Visibility check: private communities require membership
	if comm.Visibility == commv1.Visibility_VISIBILITY_PRIVATE {
		if claims == nil {
			return nil, fmt.Errorf("private community: %w", perrors.ErrForbidden)
		}
		isMember, err := s.checkMembership(ctx, comm.CommunityId, claims.UserID)
		if err != nil {
			return nil, err
		}
		if !isMember {
			return nil, fmt.Errorf("private community: %w", perrors.ErrForbidden)
		}
	}

	resp := &commv1.GetCommunityResponse{
		Community: comm,
	}

	// Populate membership info if authenticated
	if claims != nil {
		role, err := s.getMemberRole(ctx, comm.CommunityId, claims.UserID)
		if err != nil {
			return nil, err
		}
		if role != "" {
			resp.IsMember = true
			resp.IsModerator = role == "moderator" || role == "owner"
		}
	}

	return resp, nil
}

// UpdateCommunity updates community settings (owner/moderator only).
func (s *Server) UpdateCommunity(ctx context.Context, req *commv1.UpdateCommunityRequest) (*commv1.UpdateCommunityResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("update community: %w", perrors.ErrUnauthenticated)
	}

	// Look up community
	comm, err := s.getCommunityFromDB(ctx, req.GetName())
	if err != nil {
		return nil, err
	}

	// Check permissions: owner or moderator
	role, err := s.getMemberRole(ctx, comm.CommunityId, claims.UserID)
	if err != nil {
		return nil, err
	}
	if role != "owner" && role != "moderator" {
		return nil, fmt.Errorf("only owner or moderator can update community: %w", perrors.ErrForbidden)
	}

	// Build dynamic update - only update rules if provided (non-empty)
	// This prevents partial updates from accidentally clearing rules
	var rulesJSON interface{} = nil // Explicitly nil for SQL NULL
	if len(req.GetRules()) > 0 {
		jsonBytes, err := json.Marshal(communityRulesToSlice(req.GetRules()))
		if err != nil {
			return nil, fmt.Errorf("marshal rules: %w", err)
		}
		rulesJSON = jsonBytes
	}

	visibility := req.GetVisibility()
	if visibility == commv1.Visibility_VISIBILITY_UNSPECIFIED {
		visibility = comm.Visibility
	}

	// Use conditional update for rules: only update if rulesJSON is provided (not nil)
	// COALESCE($2, rules) will keep existing rules when $2 is NULL
	_, err = s.db.Exec(ctx,
		`UPDATE communities
		 SET description = COALESCE(NULLIF($1, ''), description),
		     rules = COALESCE($2, rules),
		     banner_url = COALESCE(NULLIF($3, ''), banner_url),
		     icon_url = COALESCE(NULLIF($4, ''), icon_url),
		     visibility = $5,
		     updated_at = now()
		 WHERE id = $6`,
		req.GetDescription(), rulesJSON, req.GetBannerUrl(), req.GetIconUrl(), int16(visibility), comm.CommunityId,
	)
	if err != nil {
		return nil, fmt.Errorf("update community: %w", err)
	}

	// Invalidate cache
	if cacheErr := s.cache.Invalidate(ctx, req.GetName()); cacheErr != nil {
		s.logger.Warn("cache invalidate error", zap.Error(cacheErr))
	}

	// Re-fetch updated community
	updated, err := s.getCommunityFromDB(ctx, req.GetName())
	if err != nil {
		return nil, err
	}

	return &commv1.UpdateCommunityResponse{Community: updated}, nil
}

// ListCommunities returns a paginated list of communities.
func (s *Server) ListCommunities(ctx context.Context, req *commv1.ListCommunitiesRequest) (*commv1.ListCommunitiesResponse, error) {
	pag := req.GetPagination()
	limit := pagination.DefaultLimit(pag.GetLimit(), 25, 100)
	// Fetch one extra to determine if there are more pages
	fetchLimit := limit + 1

	var args []any
	argIdx := 1
	query := `SELECT id, name, description, rules, banner_url, icon_url, visibility, member_count, owner_id, created_at
		FROM communities`

	// Build WHERE clauses
	var whereClauses []string

	if req.GetQuery() != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("name ILIKE '%%' || $%d || '%%'", argIdx))
		args = append(args, req.GetQuery())
		argIdx++
	}

	if pag.GetCursor() != "" {
		cursorID, cursorTime, err := pagination.DecodeCursor(pag.GetCursor())
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", perrors.ErrInvalidInput)
		}
		whereClauses = append(whereClauses, fmt.Sprintf(
			"(member_count, created_at, id) < ($%d, $%d, $%d)",
			argIdx, argIdx+1, argIdx+2,
		))
		// For member_count DESC ordering, we use the member_count from the cursor
		// We encode member_count in cursor ID field
		args = append(args, cursorID, cursorTime, cursorID)
		argIdx += 3
		_ = cursorID // cursor uses ID for tiebreaker
	}

	if len(whereClauses) > 0 {
		query += " WHERE "
		for i, clause := range whereClauses {
			if i > 0 {
				query += " AND "
			}
			query += clause
		}
	}

	query += " ORDER BY member_count DESC, created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, fetchLimit)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list communities: %w", err)
	}
	defer rows.Close()

	var communities []*commv1.Community
	for rows.Next() {
		comm, err := scanCommunity(rows)
		if err != nil {
			return nil, err
		}
		communities = append(communities, comm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list communities rows: %w", err)
	}

	hasMore := len(communities) > int(limit)
	if hasMore {
		communities = communities[:limit]
	}

	var nextCursor string
	if hasMore && len(communities) > 0 {
		last := communities[len(communities)-1]
		nextCursor = pagination.EncodeCursor(last.CommunityId, last.CreatedAt.AsTime())
	}

	return &commv1.ListCommunitiesResponse{
		Communities: communities,
		Pagination: &commonv1.PaginationResponse{
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	}, nil
}

// JoinCommunity adds the authenticated user to a community.
func (s *Server) JoinCommunity(ctx context.Context, req *commv1.JoinCommunityRequest) (*commv1.JoinCommunityResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("join community: %w", perrors.ErrUnauthenticated)
	}

	comm, err := s.getCommunityFromDB(ctx, req.GetName())
	if err != nil {
		return nil, err
	}

	// Private communities can't be joined (invite-only, deferred)
	if comm.Visibility == commv1.Visibility_VISIBILITY_PRIVATE {
		return nil, fmt.Errorf("private communities are invite-only: %w", perrors.ErrForbidden)
	}

	// Insert member (ON CONFLICT DO NOTHING for idempotency)
	tag, err := s.db.Exec(ctx,
		`INSERT INTO community_members (community_id, user_id, username, role)
		 VALUES ($1, $2, $3, 'member')
		 ON CONFLICT (community_id, user_id) DO NOTHING`,
		comm.CommunityId, claims.UserID, claims.Username,
	)
	if err != nil {
		return nil, fmt.Errorf("join community: %w", err)
	}

	// Only update member count if a new row was inserted
	if tag.RowsAffected() > 0 {
		_, err = s.db.Exec(ctx,
			`UPDATE communities SET member_count = (
				SELECT COUNT(*) FROM community_members WHERE community_id = $1
			) WHERE id = $1`,
			comm.CommunityId,
		)
		if err != nil {
			return nil, fmt.Errorf("update member count: %w", err)
		}
	}

	return &commv1.JoinCommunityResponse{}, nil
}

// LeaveCommunity removes the authenticated user from a community.
func (s *Server) LeaveCommunity(ctx context.Context, req *commv1.LeaveCommunityRequest) (*commv1.LeaveCommunityResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("leave community: %w", perrors.ErrUnauthenticated)
	}

	comm, err := s.getCommunityFromDB(ctx, req.GetName())
	if err != nil {
		return nil, err
	}

	// Check role — owners cannot leave
	role, err := s.getMemberRole(ctx, comm.CommunityId, claims.UserID)
	if err != nil {
		return nil, err
	}
	if role == "" {
		return nil, fmt.Errorf("not a member: %w", perrors.ErrNotFound)
	}
	if role == "owner" {
		return nil, fmt.Errorf("owner cannot leave community: %w", perrors.ErrForbidden)
	}

	_, err = s.db.Exec(ctx,
		`DELETE FROM community_members WHERE community_id = $1 AND user_id = $2`,
		comm.CommunityId, claims.UserID,
	)
	if err != nil {
		return nil, fmt.Errorf("leave community: %w", err)
	}

	// Update member count from actual count
	_, err = s.db.Exec(ctx,
		`UPDATE communities SET member_count = (
			SELECT COUNT(*) FROM community_members WHERE community_id = $1
		) WHERE id = $1`,
		comm.CommunityId,
	)
	if err != nil {
		return nil, fmt.Errorf("update member count: %w", err)
	}

	// Invalidate cache
	if cacheErr := s.cache.Invalidate(ctx, req.GetName()); cacheErr != nil {
		s.logger.Warn("cache invalidate error", zap.Error(cacheErr))
	}

	return &commv1.LeaveCommunityResponse{}, nil
}

// ListMembers returns paginated members of a community.
func (s *Server) ListMembers(ctx context.Context, req *commv1.ListMembersRequest) (*commv1.ListMembersResponse, error) {
	comm, err := s.getCommunityFromDB(ctx, req.GetName())
	if err != nil {
		return nil, err
	}

	pag := req.GetPagination()
	limit := pagination.DefaultLimit(pag.GetLimit(), 25, 100)
	fetchLimit := limit + 1

	var args []any
	argIdx := 1

	query := `SELECT user_id, username, role, joined_at
		FROM community_members
		WHERE community_id = $1`
	args = append(args, comm.CommunityId)
	argIdx++

	if pag.GetCursor() != "" {
		_, cursorTime, err := pagination.DecodeCursor(pag.GetCursor())
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", perrors.ErrInvalidInput)
		}
		query += fmt.Sprintf(" AND joined_at > $%d", argIdx)
		args = append(args, cursorTime)
		argIdx++
	}

	query += " ORDER BY joined_at ASC"
	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, fetchLimit)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var members []*commv1.Member
	for rows.Next() {
		var userID, username, role string
		var joinedAt time.Time
		if err := rows.Scan(&userID, &username, &role, &joinedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, &commv1.Member{
			UserId:   userID,
			Username: username,
			Role:     role,
			JoinedAt: timestamppb.New(joinedAt),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list members rows: %w", err)
	}

	hasMore := len(members) > int(limit)
	if hasMore {
		members = members[:limit]
	}

	var nextCursor string
	if hasMore && len(members) > 0 {
		last := members[len(members)-1]
		nextCursor = pagination.EncodeCursor(last.UserId, last.JoinedAt.AsTime())
	}

	return &commv1.ListMembersResponse{
		Members: members,
		Pagination: &commonv1.PaginationResponse{
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	}, nil
}

// AssignModerator grants moderator role to a community member (owner only).
func (s *Server) AssignModerator(ctx context.Context, req *commv1.AssignModeratorRequest) (*commv1.AssignModeratorResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("assign moderator: %w", perrors.ErrUnauthenticated)
	}

	comm, err := s.getCommunityFromDB(ctx, req.GetName())
	if err != nil {
		return nil, err
	}

	// Only owner can assign moderators
	callerRole, err := s.getMemberRole(ctx, comm.CommunityId, claims.UserID)
	if err != nil {
		return nil, err
	}
	if callerRole != "owner" {
		return nil, fmt.Errorf("only the owner can assign moderators: %w", perrors.ErrForbidden)
	}

	// Get target user's username for the member record
	// We need to look it up from user-service or the request
	targetUserID := req.GetUserId()
	if targetUserID == "" {
		return nil, fmt.Errorf("user_id is required: %w", perrors.ErrInvalidInput)
	}

	// Username is provided by frontend (from user profile lookup)
	targetUsername := req.GetUsername()
	if targetUsername == "" {
		return nil, fmt.Errorf("username is required: %w", perrors.ErrInvalidInput)
	}

	// Check if target is already a member
	targetRole, err := s.getMemberRole(ctx, comm.CommunityId, targetUserID)
	if err != nil {
		return nil, err
	}

	// If not a member, auto-add them as a moderator directly
	// (They get added and promoted in one step)
	if targetRole == "" {
		// Insert as moderator directly
		_, err = s.db.Exec(ctx,
			`INSERT INTO community_members (community_id, user_id, username, role)
			 VALUES ($1, $2, $3, 'moderator')
			 ON CONFLICT (community_id, user_id) DO UPDATE SET role = 'moderator'`,
			comm.CommunityId, targetUserID, targetUsername,
		)
		if err != nil {
			return nil, fmt.Errorf("add moderator: %w", err)
		}

		// Update member count
		_, err = s.db.Exec(ctx,
			`UPDATE communities SET member_count = (
				SELECT COUNT(*) FROM community_members WHERE community_id = $1
			) WHERE id = $1`,
			comm.CommunityId,
		)
		if err != nil {
			return nil, fmt.Errorf("update member count: %w", err)
		}
	} else {
		// Already a member, just update role to moderator
		_, err = s.db.Exec(ctx,
			`UPDATE community_members SET role = 'moderator'
			 WHERE community_id = $1 AND user_id = $2`,
			comm.CommunityId, targetUserID,
		)
		if err != nil {
			return nil, fmt.Errorf("assign moderator: %w", err)
		}
	}

	// Invalidate cache
	if cacheErr := s.cache.Invalidate(ctx, req.GetName()); cacheErr != nil {
		s.logger.Warn("cache invalidate error", zap.Error(cacheErr))
	}

	return &commv1.AssignModeratorResponse{}, nil
}

// RevokeModerator removes moderator role from a community member (owner only).
func (s *Server) RevokeModerator(ctx context.Context, req *commv1.RevokeModeratorRequest) (*commv1.RevokeModeratorResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("revoke moderator: %w", perrors.ErrUnauthenticated)
	}

	comm, err := s.getCommunityFromDB(ctx, req.GetName())
	if err != nil {
		return nil, err
	}

	// Only owner can revoke moderators
	callerRole, err := s.getMemberRole(ctx, comm.CommunityId, claims.UserID)
	if err != nil {
		return nil, err
	}
	if callerRole != "owner" {
		return nil, fmt.Errorf("only the owner can revoke moderators: %w", perrors.ErrForbidden)
	}

	// Cannot revoke self
	if req.GetUserId() == claims.UserID {
		return nil, fmt.Errorf("cannot revoke own moderator role: %w", perrors.ErrInvalidInput)
	}

	_, err = s.db.Exec(ctx,
		`UPDATE community_members SET role = 'member'
		 WHERE community_id = $1 AND user_id = $2`,
		comm.CommunityId, req.GetUserId(),
	)
	if err != nil {
		return nil, fmt.Errorf("revoke moderator: %w", err)
	}

	// Invalidate cache
	if cacheErr := s.cache.Invalidate(ctx, req.GetName()); cacheErr != nil {
		s.logger.Warn("cache invalidate error", zap.Error(cacheErr))
	}

	return &commv1.RevokeModeratorResponse{}, nil
}

// --- Internal helpers ---

// getCommunityFromDB fetches a community by name from the database.
func (s *Server) getCommunityFromDB(ctx context.Context, name string) (*commv1.Community, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, name, description, rules, banner_url, icon_url, visibility, member_count, owner_id, created_at
		 FROM communities WHERE name = $1`, name)

	comm, err := scanCommunityRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("community %q: %w", name, perrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get community: %w", err)
	}
	return comm, nil
}

// checkMembership returns true if the user is a member of the community.
func (s *Server) checkMembership(ctx context.Context, communityID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM community_members WHERE community_id = $1 AND user_id = $2)`,
		communityID, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check membership: %w", err)
	}
	return exists, nil
}

// getMemberRole returns the role of a user in a community, or "" if not a member.
func (s *Server) getMemberRole(ctx context.Context, communityID, userID string) (string, error) {
	var role string
	err := s.db.QueryRow(ctx,
		`SELECT role FROM community_members WHERE community_id = $1 AND user_id = $2`,
		communityID, userID,
	).Scan(&role)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get member role: %w", err)
	}
	return role, nil
}

// scanCommunity scans a community from a pgx.Rows row.
func scanCommunity(rows pgx.Rows) (*commv1.Community, error) {
	var (
		id, name, description, bannerURL, iconURL, ownerID string
		rulesJSON                                          []byte
		visibility                                         int16
		memberCount                                        int32
		createdAt                                          time.Time
	)

	if err := rows.Scan(&id, &name, &description, &rulesJSON, &bannerURL, &iconURL, &visibility, &memberCount, &ownerID, &createdAt); err != nil {
		return nil, fmt.Errorf("scan community: %w", err)
	}

	return buildCommunityProto(id, name, description, rulesJSON, bannerURL, iconURL, visibility, memberCount, ownerID, createdAt)
}

// scanCommunityRow scans a community from a pgx.Row.
func scanCommunityRow(row pgx.Row) (*commv1.Community, error) {
	var (
		id, name, description, bannerURL, iconURL, ownerID string
		rulesJSON                                          []byte
		visibility                                         int16
		memberCount                                        int32
		createdAt                                          time.Time
	)

	if err := row.Scan(&id, &name, &description, &rulesJSON, &bannerURL, &iconURL, &visibility, &memberCount, &ownerID, &createdAt); err != nil {
		return nil, err
	}

	return buildCommunityProto(id, name, description, rulesJSON, bannerURL, iconURL, visibility, memberCount, ownerID, createdAt)
}

func buildCommunityProto(id, name, description string, rulesJSON []byte, bannerURL, iconURL string, visibility int16, memberCount int32, ownerID string, createdAt time.Time) (*commv1.Community, error) {
	var rules []struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if len(rulesJSON) > 0 {
		if err := json.Unmarshal(rulesJSON, &rules); err != nil {
			return nil, fmt.Errorf("unmarshal rules: %w", err)
		}
	}

	var protoRules []*commv1.CommunityRule
	for _, r := range rules {
		protoRules = append(protoRules, &commv1.CommunityRule{
			Title:       r.Title,
			Description: r.Description,
		})
	}

	return &commv1.Community{
		CommunityId: id,
		Name:        name,
		Description: description,
		Rules:       protoRules,
		BannerUrl:   bannerURL,
		IconUrl:     iconURL,
		Visibility:  commv1.Visibility(visibility),
		MemberCount: memberCount,
		OwnerId:     ownerID,
		CreatedAt:   timestamppb.New(createdAt),
	}, nil
}

// communityRulesToSlice converts proto rules to a serializable slice.
func communityRulesToSlice(rules []*commv1.CommunityRule) []map[string]string {
	result := make([]map[string]string, 0, len(rules))
	for _, r := range rules {
		result = append(result, map[string]string{
			"title":       r.GetTitle(),
			"description": r.GetDescription(),
		})
	}
	return result
}

// --- Cache conversion helpers ---

func protoToCached(c *commv1.Community) *cachedCommunity {
	cc := &cachedCommunity{
		ID:          c.CommunityId,
		Name:        c.Name,
		Description: c.Description,
		BannerURL:   c.BannerUrl,
		IconURL:     c.IconUrl,
		Visibility:  int32(c.Visibility),
		MemberCount: c.MemberCount,
		OwnerID:     c.OwnerId,
	}
	if c.CreatedAt != nil {
		cc.CreatedAt = c.CreatedAt.AsTime().Unix()
	}
	for _, r := range c.Rules {
		cc.Rules = append(cc.Rules, struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		}{
			Title:       r.Title,
			Description: r.Description,
		})
	}
	return cc
}

// ListUserCommunities returns the communities a user has joined.
func (s *Server) ListUserCommunities(ctx context.Context, req *commv1.ListUserCommunitiesRequest) (*commv1.ListUserCommunitiesResponse, error) {
	userID := req.GetUserId()
	if userID == "" {
		return nil, fmt.Errorf("user_id is required: %w", perrors.ErrInvalidInput)
	}

	rows, err := s.db.Query(ctx,
		`SELECT c.id, c.name FROM community_members cm
		 JOIN communities c ON c.id = cm.community_id
		 WHERE cm.user_id = $1
		 ORDER BY cm.joined_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list user communities: %w", err)
	}
	defer rows.Close()

	var communities []*commv1.UserCommunity
	for rows.Next() {
		var communityID, name string
		if err := rows.Scan(&communityID, &name); err != nil {
			return nil, fmt.Errorf("scan user community: %w", err)
		}
		communities = append(communities, &commv1.UserCommunity{
			CommunityId: communityID,
			Name:        name,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("user communities rows: %w", err)
	}

	return &commv1.ListUserCommunitiesResponse{
		Communities: communities,
		Pagination: &commonv1.PaginationResponse{
			HasMore: false,
		},
	}, nil
}

func cachedToProto(c *cachedCommunity) *commv1.Community {
	comm := &commv1.Community{
		CommunityId: c.ID,
		Name:        c.Name,
		Description: c.Description,
		BannerUrl:   c.BannerURL,
		IconUrl:     c.IconURL,
		Visibility:  commv1.Visibility(c.Visibility),
		MemberCount: c.MemberCount,
		OwnerId:     c.OwnerID,
		CreatedAt:   timestamppb.New(time.Unix(c.CreatedAt, 0)),
	}
	for _, r := range c.Rules {
		comm.Rules = append(comm.Rules, &commv1.CommunityRule{
			Title:       r.Title,
			Description: r.Description,
		})
	}
	return comm
}
