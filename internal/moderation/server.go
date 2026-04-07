package moderation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	commentv1 "github.com/idityaGE/redyx/gen/redyx/comment/v1"
	commonv1 "github.com/idityaGE/redyx/gen/redyx/common/v1"
	commv1 "github.com/idityaGE/redyx/gen/redyx/community/v1"
	modv1 "github.com/idityaGE/redyx/gen/redyx/moderation/v1"
	postv1 "github.com/idityaGE/redyx/gen/redyx/post/v1"
	"github.com/idityaGE/redyx/internal/platform/auth"
	perrors "github.com/idityaGE/redyx/internal/platform/errors"
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

	// Call post or comment service to soft-delete the content via moderator RPCs
	switch req.ContentType {
	case modv1.ContentType_CONTENT_TYPE_POST:
		_, err = s.postClient.ModeratorRemovePost(ctx, &postv1.ModeratorRemovePostRequest{
			PostId:        req.ContentId,
			CommunityName: req.CommunityName,
		})
	case modv1.ContentType_CONTENT_TYPE_COMMENT:
		_, err = s.commentClient.ModeratorRemoveComment(ctx, &commentv1.ModeratorRemoveCommentRequest{
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
		if _, rmErr := s.postClient.RemovePostsByUser(ctx, &postv1.RemovePostsByUserRequest{
			UserId:        req.UserId,
			CommunityName: req.CommunityName,
		}); rmErr != nil {
			s.logger.Warn("failed to remove posts by banned user", zap.Error(rmErr))
		}
		if _, rmErr := s.commentClient.RemoveCommentsByUser(ctx, &commentv1.RemoveCommentsByUserRequest{
			UserId:        req.UserId,
			CommunityName: req.CommunityName,
		}); rmErr != nil {
			s.logger.Warn("failed to remove comments by banned user", zap.Error(rmErr))
		}
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
func (s *Server) PinPost(ctx context.Context, req *modv1.PinPostRequest) (*modv1.PinPostResponse, error) {
	claims, communityID, err := s.verifyModerator(ctx, req.CommunityName)
	if err != nil {
		return nil, err
	}

	// Check existing pin count — max 2 pinned posts per community
	countResp, err := s.postClient.CountPinnedPosts(ctx, &postv1.CountPinnedPostsRequest{
		CommunityName: req.CommunityName,
	})
	if err != nil {
		return nil, fmt.Errorf("count pinned posts: %w", err)
	}
	if countResp.Count >= 2 {
		return nil, fmt.Errorf("maximum 2 pinned posts per community: %w", perrors.ErrInvalidInput)
	}

	// Set post as pinned via post-service
	_, err = s.postClient.SetPostPinned(ctx, &postv1.SetPostPinnedRequest{
		PostId:        req.PostId,
		CommunityName: req.CommunityName,
		Pinned:        true,
	})
	if err != nil {
		return nil, fmt.Errorf("pin post: %w", err)
	}

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
func (s *Server) UnpinPost(ctx context.Context, req *modv1.UnpinPostRequest) (*modv1.UnpinPostResponse, error) {
	claims, communityID, err := s.verifyModerator(ctx, req.CommunityName)
	if err != nil {
		return nil, err
	}

	// Unpin post via post-service
	_, err = s.postClient.SetPostPinned(ctx, &postv1.SetPostPinnedRequest{
		PostId:        req.PostId,
		CommunityName: req.CommunityName,
		Pinned:        false,
	})
	if err != nil {
		return nil, fmt.Errorf("unpin post: %w", err)
	}

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

		// Enrich report with content title and author from post/comment service
		if r.ContentType == 1 { // CONTENT_TYPE_POST
			postResp, postErr := s.postClient.GetPost(ctx, &postv1.GetPostRequest{
				PostId: r.ContentID,
			})
			if postErr == nil && postResp.Post != nil {
				report.ContentTitle = postResp.Post.Title
				report.ContentAuthor = postResp.Post.AuthorUsername
			}
		} else if r.ContentType == 2 { // CONTENT_TYPE_COMMENT
			commentResp, commentErr := s.commentClient.GetComment(ctx, &commentv1.GetCommentRequest{
				CommentId: r.ContentID,
			})
			if commentErr == nil && commentResp.Comment != nil {
				report.ContentTitle = commentResp.Comment.Body
				report.ContentAuthor = commentResp.Comment.AuthorUsername
			}
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

// SubmitReport allows an authenticated user or an internal service to report content.
// User-submitted reports require JWT auth; service-to-service calls (e.g., spam service)
// may omit auth — in that case, reporter_id is set to "system".
func (s *Server) SubmitReport(ctx context.Context, req *modv1.SubmitReportRequest) (*modv1.SubmitReportResponse, error) {
	// Determine reporter and source based on caller
	claims := auth.ClaimsFromContext(ctx)
	reporterID := "system"
	source := "spam"
	if claims != nil {
		reporterID = claims.UserID
		source = "user"
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
		ReporterID:    reporterID,
		Reason:        req.Reason,
		Source:        source,
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

// RestoreContent re-shows previously removed content and reactivates reports (moderator only).
func (s *Server) RestoreContent(ctx context.Context, req *modv1.RestoreContentRequest) (*modv1.RestoreContentResponse, error) {
	claims, communityID, err := s.verifyModerator(ctx, req.CommunityName)
	if err != nil {
		return nil, err
	}

	// Call post/comment service to unset is_deleted
	switch req.ContentType {
	case modv1.ContentType_CONTENT_TYPE_POST:
		_, restoreErr := s.postClient.ModeratorRestorePost(ctx, &postv1.ModeratorRestorePostRequest{
			PostId:        req.ContentId,
			CommunityName: req.CommunityName,
		})
		if restoreErr != nil {
			s.logger.Warn("failed to restore post via post-service", zap.Error(restoreErr))
		}
	case modv1.ContentType_CONTENT_TYPE_COMMENT:
		_, restoreErr := s.commentClient.ModeratorRestoreComment(ctx, &commentv1.ModeratorRestoreCommentRequest{
			CommentId: req.ContentId,
		})
		if restoreErr != nil {
			s.logger.Warn("failed to restore comment via comment-service", zap.Error(restoreErr))
		}
	}

	// Reactivate reports — move them back to "active" status
	if err := s.store.ReactivateReports(ctx, req.ContentId, int16(req.ContentType)); err != nil {
		s.logger.Error("failed to reactivate reports", zap.Error(err))
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
// If user_id is empty, uses the authenticated user's ID from JWT claims.
func (s *Server) CheckBan(ctx context.Context, req *modv1.CheckBanRequest) (*modv1.CheckBanResponse, error) {
	// If no user_id provided, use the authenticated user's ID
	userID := req.UserId
	if userID == "" {
		claims := auth.ClaimsFromContext(ctx)
		if claims == nil {
			return nil, fmt.Errorf("user_id required or must be authenticated: %w", perrors.ErrUnauthenticated)
		}
		userID = claims.UserID
	}

	// First, resolve community ID from name
	resp, err := s.communityClient.GetCommunity(ctx, &commv1.GetCommunityRequest{
		Name: req.CommunityName,
	})
	if err != nil {
		return nil, fmt.Errorf("get community: %w", err)
	}
	communityID := resp.Community.CommunityId

	// Check Redis cache first
	key := banCacheKey(communityID, userID)
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
	ban, err := s.store.GetBan(ctx, communityID, userID)
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
