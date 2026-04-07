package spam

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	spamv1 "github.com/idityaGE/redyx/gen/redyx/spam/v1"
)

// Server implements the SpamServiceServer gRPC interface.
type Server struct {
	spamv1.UnimplementedSpamServiceServer
	blocklist *Blocklist
	dedup     *DedupChecker
	logger    *zap.Logger
}

// NewServer creates a new spam gRPC server.
func NewServer(blocklist *Blocklist, dedup *DedupChecker, logger *zap.Logger) *Server {
	return &Server{
		blocklist: blocklist,
		dedup:     dedup,
		logger:    logger,
	}
}

// CheckContent evaluates content for spam before publishing.
// Checks are performed in order:
//  1. Keyword blocklist matching (case-insensitive)
//  2. URL domain blocklist matching (from request urls + extracted from content)
//  3. Duplicate content detection (SHA-256 hash + Redis)
//
// Returns vague reasons only — never includes the specific keyword or URL that matched.
func (s *Server) CheckContent(ctx context.Context, req *spamv1.CheckContentRequest) (*spamv1.CheckContentResponse, error) {
	resp := &spamv1.CheckContentResponse{
		Result: spamv1.SpamCheckResult_SPAM_CHECK_RESULT_CLEAN,
	}

	var reasons []string

	// 1. Check keywords in content
	if matched, kw := s.blocklist.CheckKeywords(req.GetContent()); matched {
		s.logger.Debug("blocked keyword detected",
			zap.String("user_id", req.GetUserId()),
			zap.String("keyword", kw),
		)
		reasons = append(reasons, "blocked_content")
		resp.Result = spamv1.SpamCheckResult_SPAM_CHECK_RESULT_SPAM
	}

	// 2. Extract URLs from content field and combine with request URLs
	allURLs := make([]string, 0, len(req.GetUrls()))
	allURLs = append(allURLs, req.GetUrls()...)
	contentURLs := ExtractURLs(req.GetContent())
	allURLs = append(allURLs, contentURLs...)

	// Check URLs against domain blocklist
	if matched, domain := s.blocklist.CheckURLs(allURLs); matched {
		s.logger.Debug("blocked URL domain detected",
			zap.String("user_id", req.GetUserId()),
			zap.String("domain", domain),
		)
		// Only add reason if not already spam from keywords
		if resp.Result != spamv1.SpamCheckResult_SPAM_CHECK_RESULT_SPAM {
			resp.Result = spamv1.SpamCheckResult_SPAM_CHECK_RESULT_SPAM
		}
		reasons = append(reasons, "blocked_url")
	}

	// 3. Check duplicate via dedup
	hash, isDuplicate, err := s.dedup.Check(ctx, req.GetUserId(), req.GetContent())
	if err != nil {
		s.logger.Error("dedup check failed",
			zap.String("user_id", req.GetUserId()),
			zap.Error(err),
		)
		// Don't fail the request on dedup errors — just skip dedup check
	} else {
		resp.ContentHash = hash
		resp.IsDuplicate = isDuplicate
	}

	resp.Reasons = reasons
	return resp, nil
}

// ReportSpam allows users to report content as spam.
// For now, generates a report ID and logs the report.
// Full integration with moderation service SubmitReport happens via the behavior consumer.
func (s *Server) ReportSpam(_ context.Context, req *spamv1.ReportSpamRequest) (*spamv1.ReportSpamResponse, error) {
	reportID := uuid.New().String()

	s.logger.Info("spam report submitted",
		zap.String("report_id", reportID),
		zap.String("content_id", req.GetContentId()),
		zap.String("content_type", req.GetContentType()),
		zap.String("reporter_id", req.GetReporterId()),
		zap.String("reason", req.GetReason()),
	)

	return &spamv1.ReportSpamResponse{
		ReportId: reportID,
	}, nil
}
