package moderation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	commentv1 "github.com/redyx/redyx/gen/redyx/comment/v1"
	commonv1 "github.com/redyx/redyx/gen/redyx/common/v1"
	commv1 "github.com/redyx/redyx/gen/redyx/community/v1"
	modv1 "github.com/redyx/redyx/gen/redyx/moderation/v1"
	postv1 "github.com/redyx/redyx/gen/redyx/post/v1"
	"github.com/redyx/redyx/internal/platform/auth"
	perrors "github.com/redyx/redyx/internal/platform/errors"
)

// Server implements the ModerationServiceServer gRPC interface.
type Server struct {
	modv1.UnimplementedModerationServiceServer

	store           *Store
	logger          *zap.Logger
	communityClient commv1.CommunityServiceClient
	postClient      postv1.PostServiceClient
	commentClient   commentv1.CommentServiceClient
	redisClient     *redis.Client
}

// NewServer creates a new moderation service server.
func NewServer(
	store *Store,
	logger *zap.Logger,
	communityClient commv1.CommunityServiceClient,
	postClient postv1.PostServiceClient,
	commentClient commentv1.CommentServiceClient,
	redisClient *redis.Client,
) *Server {
	return &Server{
		store:           store,
		logger:          logger,
		communityClient: communityClient,
		postClient:      postClient,
		commentClient:   commentClient,
		redisClient:     redisClient,
	}
}

// verifyModerator checks that the request is from an authenticated moderator
// of the given community. Returns the claims if verified, or an error.
func (s *Server) verifyModerator(ctx context.Context, communityName string) (*auth.Claims, string, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, "", fmt.Errorf("unauthenticated: %w", perrors.ErrUnauthenticated)
	}

	// Call community-service to verify role
	resp, err := s.communityClient.GetCommunity(ctx, &commv1.GetCommunityRequest{
		Name: communityName,
	})
	if err != nil {
		return nil, "", fmt.Errorf("verify community: %w", err)
	}
	if !resp.IsModerator {
		return nil, "", fmt.Errorf("not a moderator: %w", perrors.ErrForbidden)
	}

	communityID := resp.Community.CommunityId
	return claims, communityID, nil
}

// modActionString converts a ModAction enum to the string stored in the database.
func modActionString(action modv1.ModAction) string {
	switch action {
	case modv1.ModAction_MOD_ACTION_REMOVE_POST:
		return "remove_post"
	case modv1.ModAction_MOD_ACTION_REMOVE_COMMENT:
		return "remove_comment"
	case modv1.ModAction_MOD_ACTION_BAN_USER:
		return "ban_user"
	case modv1.ModAction_MOD_ACTION_UNBAN_USER:
		return "unban_user"
	case modv1.ModAction_MOD_ACTION_PIN_POST:
		return "pin_post"
	case modv1.ModAction_MOD_ACTION_UNPIN_POST:
		return "unpin_post"
	case modv1.ModAction_MOD_ACTION_DISMISS_REPORT:
		return "dismiss_report"
	case modv1.ModAction_MOD_ACTION_RESTORE_CONTENT:
		return "restore_content"
	default:
		return "unknown"
	}
}

// modActionFromString converts a database action string to the proto enum.
func modActionFromString(action string) modv1.ModAction {
	switch action {
	case "remove_post":
		return modv1.ModAction_MOD_ACTION_REMOVE_POST
	case "remove_comment":
		return modv1.ModAction_MOD_ACTION_REMOVE_COMMENT
	case "ban_user":
		return modv1.ModAction_MOD_ACTION_BAN_USER
	case "unban_user":
		return modv1.ModAction_MOD_ACTION_UNBAN_USER
	case "pin_post":
		return modv1.ModAction_MOD_ACTION_PIN_POST
	case "unpin_post":
		return modv1.ModAction_MOD_ACTION_UNPIN_POST
	case "dismiss_report":
		return modv1.ModAction_MOD_ACTION_DISMISS_REPORT
	case "restore_content":
		return modv1.ModAction_MOD_ACTION_RESTORE_CONTENT
	default:
		return modv1.ModAction_MOD_ACTION_UNSPECIFIED
	}
}

// banCacheKey returns the Redis key for a ban cache entry.
func banCacheKey(communityID, userID string) string {
	return fmt.Sprintf("ban:%s:%s", communityID, userID)
}

// banCacheEntry is the JSON structure stored in Redis for ban caching.
type banCacheEntry struct {
	Reason    string     `json:"reason"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// RemoveContent hides a post or comment from regular users (moderator only).
func (s *Server) RemoveContent(ctx context.Context, req *modv1.RemoveContentRequest) (*modv1.RemoveContentResponse, error) {
	claims, communityID, err := s.verifyModerator(ctx, req.CommunityName)
	if err != nil {
		return nil, err
	}

	// Call post or comment service to soft-delete the content
	switch req.ContentType {
	case modv1.ContentType_CONTENT_TYPE_POST:
		_, err = s.postClient.DeletePost(ctx, &postv1.DeletePostRequest{
			PostId: req.ContentId,
		})
	case modv1.ContentType_CONTENT_TYPE_COMMENT:
		_, err = s.commentClient.DeleteComment(ctx, &commentv1.DeleteCommentRequest{
			CommentId: req.ContentId,
		})
	default:
		return nil, fmt.Errorf("invalid content type: %w", perrors.ErrInvalidInput)
	}
	if err != nil {
		return nil, fmt.Errorf("remove content: %w", err)
	}

	// Determine action and target type for mod log
	action := modv1.ModAction_MOD_ACTION_REMOVE_POST
	targetType := "post"
	if req.ContentType == modv1.ContentType_CONTENT_TYPE_COMMENT {
		action = modv1.ModAction_MOD_ACTION_REMOVE_COMMENT
		targetType = "comment"
	}

	// Log to mod_log
	if _, err := s.store.CreateModLogEntry(ctx, &ModLogRecord{
		CommunityID:       communityID,
		CommunityName:     req.CommunityName,
		ModeratorID:       claims.UserID,
		ModeratorUsername: claims.Username,
		Action:            modActionString(action),
		TargetID:          req.ContentId,
		TargetType:        targetType,
		Reason:            req.Reason,
	}); err != nil {
		s.logger.Error("failed to create mod log entry", zap.Error(err))
	}

	// Mark related reports as resolved with "removed" action
	if err := s.store.UpdateReportStatus(ctx, req.ContentId, int16(req.ContentType), "removed", claims.UserID); err != nil {
		s.logger.Error("failed to update report status", zap.Error(err))
	}

	return &modv1.RemoveContentResponse{}, nil
}

// BanUser bans a user from a community (moderator only).
func (s *Server) BanUser(ctx context.Context, req *modv1.BanUserRequest) (*modv1.BanUserResponse, error) {
	claims, communityID, err := s.verifyModerator(ctx, req.CommunityName)
	if err != nil {
		return nil, err
	}

	// Calculate expiry
	var expiresAt *time.Time
	if req.DurationSeconds > 0 {
		t := time.Now().Add(time.Duration(req.DurationSeconds) * time.Second)
		expiresAt = &t
	}

	// Create ban in database
	_, err = s.store.CreateBan(ctx, &BanRecord{
		CommunityID:      communityID,
		CommunityName:    req.CommunityName,
		UserID:           req.UserId,
		Username:         req.Username,
		Reason:           req.Reason,
		BannedBy:         claims.UserID,
		BannedByUsername: claims.Username,
		DurationSeconds:  req.DurationSeconds,
		ExpiresAt:        expiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("create ban: %w", err)
	}

	// Cache ban in Redis
	entry := banCacheEntry{
		Reason:    req.Reason,
		ExpiresAt: expiresAt,
	}
	entryJSON, _ := json.Marshal(entry)
	key := banCacheKey(communityID, req.UserId)

	if req.DurationSeconds > 0 {
		s.redisClient.Set(ctx, key, entryJSON, time.Duration(req.DurationSeconds)*time.Second)
	} else {
		// Permanent ban — no TTL
		s.redisClient.Set(ctx, key, entryJSON, 0)
	}

	// Log to mod_log
	if _, err := s.store.CreateModLogEntry(ctx, &ModLogRecord{
		CommunityID:       communityID,
		CommunityName:     req.CommunityName,
		ModeratorID:       claims.UserID,
		ModeratorUsername: claims.Username,
		Action:            modActionString(modv1.ModAction_MOD_ACTION_BAN_USER),
		TargetID:          req.UserId,
		TargetType:        "user",
		Reason:            req.Reason,
	}); err != nil {
		s.logger.Error("failed to create mod log entry", zap.Error(err))
	}

	// Optionally remove all user content in the community
	if req.RemoveContent {
		s.logger.Info("remove_content flag set for ban — content removal will be handled by post/comment services in Plan 03",
			zap.String("user_id", req.UserId),
			zap.String("community", req.CommunityName),
		)
	}

	return &modv1.BanUserResponse{}, nil
}

// UnbanUser removes a ban from a user in a community (moderator only).
func (s *Server) UnbanUser(ctx context.Context, req *modv1.UnbanUserRequest) (*modv1.UnbanUserResponse, error) {
	claims, communityID, err := s.verifyModerator(ctx, req.CommunityName)
	if err != nil {
		return nil, err
	}

	// Delete ban from database
	if err := s.store.DeleteBan(ctx, communityID, req.UserId); err != nil {
		return nil, fmt.Errorf("delete ban: %w", err)
	}

	// Remove ban from Redis cache
	s.redisClient.Del(ctx, banCacheKey(communityID, req.UserId))

	// Log to mod_log
	if _, err := s.store.CreateModLogEntry(ctx, &ModLogRecord{
		CommunityID:       communityID,
		CommunityName:     req.CommunityName,
		ModeratorID:       claims.UserID,
		ModeratorUsername: claims.Username,
		Action:            modActionString(modv1.ModAction_MOD_ACTION_UNBAN_USER),
		TargetID:          req.UserId,
		TargetType:        "user",
	}); err != nil {
		s.logger.Error("failed to create mod log entry", zap.Error(err))
	}

	return &modv1.UnbanUserResponse{}, nil
}

// PinPost pins a post to the top of a community feed (max 2 pins).
// NOTE: This requires an internal RPC on post-service to set is_pinned, which will be
// added in Plan 03. For now, this logs the action and records in mod_log.
func (s *Server) PinPost(ctx context.Context, req *modv1.PinPostRequest) (*modv1.PinPostResponse, error) {
	claims, communityID, err := s.verifyModerator(ctx, req.CommunityName)
	if err != nil {
		return nil, err
	}

	// TODO(Plan-03): Call postClient to count existing pins and reject if >= 2.
	// TODO(Plan-03): Call postClient.SetPinned(postId, true) to update is_pinned column.
	s.logger.Info("PinPost: post-service internal RPC needed (Plan 03)",
		zap.String("post_id", req.PostId),
		zap.String("community", req.CommunityName),
	)

	// Log to mod_log
	if _, err := s.store.CreateModLogEntry(ctx, &ModLogRecord{
		CommunityID:       communityID,
		CommunityName:     req.CommunityName,
		ModeratorID:       claims.UserID,
		ModeratorUsername: claims.Username,
		Action:            modActionString(modv1.ModAction_MOD_ACTION_PIN_POST),
		TargetID:          req.PostId,
		TargetType:        "post",
	}); err != nil {
		s.logger.Error("failed to create mod log entry", zap.Error(err))
	}

	return &modv1.PinPostResponse{}, nil
}

// UnpinPost removes a post from the pinned position.
// NOTE: Requires internal RPC on post-service (Plan 03).
func (s *Server) UnpinPost(ctx context.Context, req *modv1.UnpinPostRequest) (*modv1.UnpinPostResponse, error) {
	claims, communityID, err := s.verifyModerator(ctx, req.CommunityName)
	if err != nil {
		return nil, err
	}

	// TODO(Plan-03): Call postClient.SetPinned(postId, false) to clear is_pinned column.
	s.logger.Info("UnpinPost: post-service internal RPC needed (Plan 03)",
		zap.String("post_id", req.PostId),
		zap.String("community", req.CommunityName),
	)

	// Log to mod_log
	if _, err := s.store.CreateModLogEntry(ctx, &ModLogRecord{
		CommunityID:       communityID,
		CommunityName:     req.CommunityName,
		ModeratorID:       claims.UserID,
		ModeratorUsername: claims.Username,
		Action:            modActionString(modv1.ModAction_MOD_ACTION_UNPIN_POST),
		TargetID:          req.PostId,
		TargetType:        "post",
	}); err != nil {
		s.logger.Error("failed to create mod log entry", zap.Error(err))
	}

	return &modv1.UnpinPostResponse{}, nil
}

// GetModLog returns paginated moderation action history.
func (s *Server) GetModLog(ctx context.Context, req *modv1.GetModLogRequest) (*modv1.GetModLogResponse, error) {
	_, communityID, err := s.verifyModerator(ctx, req.CommunityName)
	if err != nil {
		return nil, err
	}

	limit := int32(25)
	offset := int32(0)
	if req.Pagination != nil {
		if req.Pagination.Limit > 0 {
			limit = req.Pagination.Limit
		}
	}

	actionFilter := ""
	if req.ActionFilter != modv1.ModAction_MOD_ACTION_UNSPECIFIED {
		actionFilter = modActionString(req.ActionFilter)
	}

	entries, totalCount, err := s.store.ListModLog(ctx, communityID, actionFilter, int(limit), int(offset))
	if err != nil {
		return nil, fmt.Errorf("list mod log: %w", err)
	}

	var pbEntries []*modv1.ModLogEntry
	for _, e := range entries {
		pbEntries = append(pbEntries, &modv1.ModLogEntry{
			EntryId:           e.ID,
			Action:            modActionFromString(e.Action),
			ModeratorId:       e.ModeratorID,
			ModeratorUsername: e.ModeratorUsername,
			TargetId:          e.TargetID,
			TargetType:        e.TargetType,
			Reason:            e.Reason,
			CreatedAt:         timestamppb.New(e.CreatedAt),
		})
	}

	return &modv1.GetModLogResponse{
		Entries: pbEntries,
		Pagination: &commonv1.PaginationResponse{
			TotalCount: int32(totalCount),
			HasMore:    int(offset)+int(limit) < totalCount,
		},
	}, nil
}

// ListReportQueue returns reported content awaiting review (moderator only).
func (s *Server) ListReportQueue(ctx context.Context, req *modv1.ListReportQueueRequest) (*modv1.ListReportQueueResponse, error) {
	_, communityID, err := s.verifyModerator(ctx, req.CommunityName)
	if err != nil {
		return nil, err
	}

	limit := int32(25)
	offset := int32(0)
	if req.Pagination != nil {
		if req.Pagination.Limit > 0 {
			limit = req.Pagination.Limit
		}
	}

	status := req.Status
	if status == "" {
		status = "active" // default to active reports
	}

	reports, totalCount, err := s.store.ListReports(ctx, communityID, status, req.Source, int(limit), int(offset))
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}

	var pbReports []*modv1.Report
	for _, r := range reports {
		report := &modv1.Report{
			ReportId:    r.ID,
			ContentId:   r.ContentID,
			ContentType: modv1.ContentType(r.ContentType),
			ReporterId:  r.ReporterID,
			Reason:      r.Reason,
			ReportCount: r.ReportCount,
			CreatedAt:   timestamppb.New(r.CreatedAt),
			Source:      r.Source,
			Status:      r.Status,
		}
		if r.ResolvedAction != nil {
			report.ResolvedAction = *r.ResolvedAction
		}
		pbReports = append(pbReports, report)
	}

	return &modv1.ListReportQueueResponse{
		Reports: pbReports,
		Pagination: &commonv1.PaginationResponse{
			TotalCount: int32(totalCount),
			HasMore:    int(offset)+int(limit) < totalCount,
		},
	}, nil
}

// SubmitReport allows an authenticated user to report content.
func (s *Server) SubmitReport(ctx context.Context, req *modv1.SubmitReportRequest) (*modv1.SubmitReportResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("must be logged in to report: %w", perrors.ErrUnauthenticated)
	}

	// Resolve community ID
	resp, err := s.communityClient.GetCommunity(ctx, &commv1.GetCommunityRequest{
		Name: req.CommunityName,
	})
	if err != nil {
		return nil, fmt.Errorf("get community: %w", err)
	}

	id, err := s.store.CreateReport(ctx, &ReportRecord{
		CommunityID:   resp.Community.CommunityId,
		CommunityName: req.CommunityName,
		ContentID:     req.ContentId,
		ContentType:   int16(req.ContentType),
		ReporterID:    claims.UserID,
		Reason:        req.Reason,
		Source:        "user",
	})
	if err != nil {
		return nil, fmt.Errorf("create report: %w", err)
	}

	return &modv1.SubmitReportResponse{
		ReportId: id,
	}, nil
}

// DismissReport marks a report as resolved without removing content (moderator only).
func (s *Server) DismissReport(ctx context.Context, req *modv1.DismissReportRequest) (*modv1.DismissReportResponse, error) {
	claims, communityID, err := s.verifyModerator(ctx, req.CommunityName)
	if err != nil {
		return nil, err
	}

	// Update report status to resolved/dismissed
	if err := s.store.UpdateReportStatus(ctx, req.ContentId, int16(req.ContentType), "dismissed", claims.UserID); err != nil {
		return nil, fmt.Errorf("dismiss report: %w", err)
	}

	// Log to mod_log
	if _, err := s.store.CreateModLogEntry(ctx, &ModLogRecord{
		CommunityID:       communityID,
		CommunityName:     req.CommunityName,
		ModeratorID:       claims.UserID,
		ModeratorUsername: claims.Username,
		Action:            modActionString(modv1.ModAction_MOD_ACTION_DISMISS_REPORT),
		TargetID:          req.ContentId,
		TargetType:        contentTypeString(req.ContentType),
	}); err != nil {
		s.logger.Error("failed to create mod log entry", zap.Error(err))
	}

	return &modv1.DismissReportResponse{}, nil
}

// RestoreContent re-shows previously removed content (moderator only).
// NOTE: Requires internal RPCs on post/comment services to undelete (Plan 03).
func (s *Server) RestoreContent(ctx context.Context, req *modv1.RestoreContentRequest) (*modv1.RestoreContentResponse, error) {
	claims, communityID, err := s.verifyModerator(ctx, req.CommunityName)
	if err != nil {
		return nil, err
	}

	// TODO(Plan-03): Call post/comment service to unset is_deleted.
	s.logger.Info("RestoreContent: post/comment service internal undelete RPC needed (Plan 03)",
		zap.String("content_id", req.ContentId),
		zap.String("content_type", contentTypeString(req.ContentType)),
	)

	// Mark related reports as resolved with "restored" action
	if err := s.store.UpdateReportStatus(ctx, req.ContentId, int16(req.ContentType), "restored", claims.UserID); err != nil {
		s.logger.Error("failed to update report status", zap.Error(err))
	}

	// Log to mod_log
	if _, err := s.store.CreateModLogEntry(ctx, &ModLogRecord{
		CommunityID:       communityID,
		CommunityName:     req.CommunityName,
		ModeratorID:       claims.UserID,
		ModeratorUsername: claims.Username,
		Action:            modActionString(modv1.ModAction_MOD_ACTION_RESTORE_CONTENT),
		TargetID:          req.ContentId,
		TargetType:        contentTypeString(req.ContentType),
	}); err != nil {
		s.logger.Error("failed to create mod log entry", zap.Error(err))
	}

	return &modv1.RestoreContentResponse{}, nil
}

// ListBans returns active bans for a community (moderator only).
func (s *Server) ListBans(ctx context.Context, req *modv1.ListBansRequest) (*modv1.ListBansResponse, error) {
	_, communityID, err := s.verifyModerator(ctx, req.CommunityName)
	if err != nil {
		return nil, err
	}

	limit := int32(25)
	offset := int32(0)
	if req.Pagination != nil {
		if req.Pagination.Limit > 0 {
			limit = req.Pagination.Limit
		}
	}

	bans, totalCount, err := s.store.ListBans(ctx, communityID, int(limit), int(offset))
	if err != nil {
		return nil, fmt.Errorf("list bans: %w", err)
	}

	var pbBans []*modv1.Ban
	for _, b := range bans {
		ban := &modv1.Ban{
			BanId:            b.ID,
			UserId:           b.UserID,
			Username:         b.Username,
			Reason:           b.Reason,
			BannedBy:         b.BannedBy,
			BannedByUsername: b.BannedByUsername,
			DurationSeconds:  b.DurationSeconds,
			CreatedAt:        timestamppb.New(b.CreatedAt),
		}
		if b.ExpiresAt != nil {
			ban.ExpiresAt = timestamppb.New(*b.ExpiresAt)
		}
		pbBans = append(pbBans, ban)
	}

	return &modv1.ListBansResponse{
		Bans: pbBans,
		Pagination: &commonv1.PaginationResponse{
			TotalCount: int32(totalCount),
			HasMore:    int(offset)+int(limit) < totalCount,
		},
	}, nil
}

// CheckBan checks if a user is banned from a community.
// Checks Redis cache first, falls back to database.
func (s *Server) CheckBan(ctx context.Context, req *modv1.CheckBanRequest) (*modv1.CheckBanResponse, error) {
	// First, resolve community ID from name
	resp, err := s.communityClient.GetCommunity(ctx, &commv1.GetCommunityRequest{
		Name: req.CommunityName,
	})
	if err != nil {
		return nil, fmt.Errorf("get community: %w", err)
	}
	communityID := resp.Community.CommunityId

	// Check Redis cache first
	key := banCacheKey(communityID, req.UserId)
	cached, err := s.redisClient.Get(ctx, key).Result()
	if err == nil {
		// Cache hit
		var entry banCacheEntry
		if err := json.Unmarshal([]byte(cached), &entry); err == nil {
			result := &modv1.CheckBanResponse{
				IsBanned: true,
				Reason:   entry.Reason,
			}
			if entry.ExpiresAt != nil {
				result.ExpiresAt = timestamppb.New(*entry.ExpiresAt)
			}
			return result, nil
		}
	}

	// Cache miss — check database
	ban, err := s.store.GetBan(ctx, communityID, req.UserId)
	if err != nil {
		return nil, fmt.Errorf("check ban: %w", err)
	}

	if ban == nil {
		return &modv1.CheckBanResponse{
			IsBanned: false,
		}, nil
	}

	// Cache the ban in Redis for future lookups
	entry := banCacheEntry{
		Reason:    ban.Reason,
		ExpiresAt: ban.ExpiresAt,
	}
	entryJSON, _ := json.Marshal(entry)
	if ban.ExpiresAt != nil {
		ttl := time.Until(*ban.ExpiresAt)
		if ttl > 0 {
			s.redisClient.Set(ctx, key, entryJSON, ttl)
		}
	} else {
		// Permanent ban — cache for 1 hour, re-check periodically
		s.redisClient.Set(ctx, key, entryJSON, time.Hour)
	}

	result := &modv1.CheckBanResponse{
		IsBanned: true,
		Reason:   ban.Reason,
	}
	if ban.ExpiresAt != nil {
		result.ExpiresAt = timestamppb.New(*ban.ExpiresAt)
	}
	return result, nil
}

// contentTypeString returns a human-readable string for a ContentType enum value.
func contentTypeString(ct modv1.ContentType) string {
	switch ct {
	case modv1.ContentType_CONTENT_TYPE_POST:
		return "post"
	case modv1.ContentType_CONTENT_TYPE_COMMENT:
		return "comment"
	default:
		return "unknown"
	}
}
