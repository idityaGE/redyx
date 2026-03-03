package vote

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/redyx/redyx/gen/redyx/common/v1"
	votev1 "github.com/redyx/redyx/gen/redyx/vote/v1"
	"github.com/redyx/redyx/internal/platform/auth"
	perrors "github.com/redyx/redyx/internal/platform/errors"
)

// Server implements the VoteServiceServer gRPC interface.
type Server struct {
	votev1.UnimplementedVoteServiceServer
	store    *VoteStore
	producer *Producer
	logger   *zap.Logger
}

// NewServer creates a new vote gRPC server.
func NewServer(store *VoteStore, producer *Producer, logger *zap.Logger) *Server {
	return &Server{
		store:    store,
		producer: producer,
		logger:   logger,
	}
}

// Vote casts, changes, or removes a vote on a post or comment.
// Implements VOTE-01, VOTE-02 (atomic via Lua), VOTE-03 (<50ms Redis), VOTE-05 (idempotent).
func (s *Server) Vote(ctx context.Context, req *votev1.VoteRequest) (*votev1.VoteResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("vote: %w", perrors.ErrUnauthenticated)
	}

	// Validate target_id
	if req.GetTargetId() == "" {
		return nil, fmt.Errorf("target_id is required: %w", perrors.ErrInvalidInput)
	}

	// Validate target_type
	if req.GetTargetType() != votev1.TargetType_TARGET_TYPE_POST &&
		req.GetTargetType() != votev1.TargetType_TARGET_TYPE_COMMENT {
		return nil, fmt.Errorf("target_type must be POST or COMMENT: %w", perrors.ErrInvalidInput)
	}

	// Validate and map direction
	var direction string
	switch req.GetDirection() {
	case votev1.VoteDirection_VOTE_DIRECTION_UP:
		direction = "up"
	case votev1.VoteDirection_VOTE_DIRECTION_DOWN:
		direction = "down"
	case votev1.VoteDirection_VOTE_DIRECTION_NONE:
		direction = "none"
	default:
		return nil, fmt.Errorf("direction must be UP, DOWN, or NONE: %w", perrors.ErrInvalidInput)
	}

	// Cast vote atomically in Redis
	delta, newScore, _, err := s.store.CastVote(ctx, claims.UserID, req.GetTargetId(), direction)
	if err != nil {
		return nil, fmt.Errorf("cast vote: %w", err)
	}

	// If the vote produced a score change, publish event to Kafka (async, non-blocking)
	if delta != 0 {
		targetTypeStr := "post"
		if req.GetTargetType() == votev1.TargetType_TARGET_TYPE_COMMENT {
			targetTypeStr = "comment"
		}

		event := &commonv1.VoteEvent{
			EventId:     uuid.New().String(),
			UserId:      claims.UserID,
			TargetId:    req.GetTargetId(),
			TargetType:  targetTypeStr,
			AuthorId:    "", // Consumer looks up from post data — keeps Vote RPC fast
			ScoreDelta:  int32(delta),
			CommunityId: "", // Consumer looks up from post data
			OccurredAt:  timestamppb.Now(),
		}
		s.producer.PublishVoteEvent(ctx, event)
	}

	return &votev1.VoteResponse{
		NewScore: int32(newScore),
	}, nil
}

// GetVoteState returns the current user's vote on a specific item.
func (s *Server) GetVoteState(ctx context.Context, req *votev1.GetVoteStateRequest) (*votev1.GetVoteStateResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("get vote state: %w", perrors.ErrUnauthenticated)
	}

	state, err := s.store.GetVoteState(ctx, claims.UserID, req.GetTargetId())
	if err != nil {
		return nil, fmt.Errorf("get vote state: %w", err)
	}

	var dir votev1.VoteDirection
	switch state {
	case "up":
		dir = votev1.VoteDirection_VOTE_DIRECTION_UP
	case "down":
		dir = votev1.VoteDirection_VOTE_DIRECTION_DOWN
	default:
		dir = votev1.VoteDirection_VOTE_DIRECTION_UNSPECIFIED
	}

	return &votev1.GetVoteStateResponse{
		Direction: dir,
	}, nil
}
