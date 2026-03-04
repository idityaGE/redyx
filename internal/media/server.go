package media

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	mediav1 "github.com/redyx/redyx/gen/redyx/media/v1"
	"github.com/redyx/redyx/internal/platform/auth"
)

// Allowed content types and size limits.
var (
	allowedImageTypes = map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
	allowedVideoTypes = map[string]bool{
		"video/mp4":  true,
		"video/webm": true,
	}

	maxImageSize int64 = 20 * 1024 * 1024  // 20MB
	maxVideoSize int64 = 100 * 1024 * 1024 // 100MB
)

// Server implements the MediaServiceServer gRPC interface.
type Server struct {
	mediav1.UnimplementedMediaServiceServer
	store  *Store
	s3     *S3Client
	logger *zap.Logger
}

// NewServer creates a new media gRPC server.
func NewServer(store *Store, s3 *S3Client, logger *zap.Logger) *Server {
	return &Server{
		store:  store,
		s3:     s3,
		logger: logger,
	}
}

// InitUpload validates the upload request, creates a PENDING media record,
// and returns a presigned PUT URL for direct client-to-S3 upload.
func (s *Server) InitUpload(ctx context.Context, req *mediav1.InitUploadRequest) (*mediav1.InitUploadResponse, error) {
	// Extract user_id from context (auth required)
	userID := auth.UserIDFromContext(ctx)
	if userID == "" {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Validate filename
	if req.Filename == "" {
		return nil, status.Error(codes.InvalidArgument, "filename is required")
	}

	// Validate content type and size
	contentType := strings.ToLower(req.ContentType)
	if allowedImageTypes[contentType] {
		if req.SizeBytes > maxImageSize {
			return nil, status.Errorf(codes.InvalidArgument, "image size exceeds maximum of %d bytes", maxImageSize)
		}
	} else if allowedVideoTypes[contentType] {
		if req.SizeBytes > maxVideoSize {
			return nil, status.Errorf(codes.InvalidArgument, "video size exceeds maximum of %d bytes", maxVideoSize)
		}
	} else {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported content type: %s", req.ContentType)
	}

	if req.SizeBytes <= 0 {
		return nil, status.Error(codes.InvalidArgument, "size_bytes must be positive")
	}

	// Generate unique S3 key: media/{user_id}/{uuid}/{filename}
	fileUUID := uuid.New().String()
	s3Key := fmt.Sprintf("media/%s/%s/%s", userID, fileUUID, req.Filename)

	// Determine media type string for storage
	mediaType := mediaTypeToString(req.MediaType)

	// Create PENDING media record in store
	item := MediaItem{
		UserID:      userID,
		Filename:    req.Filename,
		ContentType: req.ContentType,
		SizeBytes:   req.SizeBytes,
		MediaType:   mediaType,
		Status:      "pending",
		S3Key:       s3Key,
	}

	mediaID, err := s.store.Create(ctx, item)
	if err != nil {
		s.logger.Error("failed to create media record", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to create media record")
	}

	// Generate presigned PUT URL
	uploadURL, expiresAt, err := s.s3.GeneratePresignedPUT(ctx, s3Key, req.ContentType, req.SizeBytes)
	if err != nil {
		s.logger.Error("failed to generate presigned URL", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to generate upload URL")
	}

	s.logger.Info("init upload",
		zap.String("media_id", mediaID),
		zap.String("user_id", userID),
		zap.String("content_type", req.ContentType),
		zap.Int64("size_bytes", req.SizeBytes),
	)

	return &mediav1.InitUploadResponse{
		MediaId:   mediaID,
		UploadUrl: uploadURL,
		ExpiresAt: timestamppb.New(expiresAt),
	}, nil
}

// CompleteUpload verifies the uploaded object exists in S3, generates a thumbnail
// (for images), and updates the media record status to READY.
func (s *Server) CompleteUpload(ctx context.Context, req *mediav1.CompleteUploadRequest) (*mediav1.CompleteUploadResponse, error) {
	// Extract user_id from context (auth required)
	userID := auth.UserIDFromContext(ctx)
	if userID == "" {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	if req.MediaId == "" {
		return nil, status.Error(codes.InvalidArgument, "media_id is required")
	}

	// Get media record from store
	item, err := s.store.Get(ctx, req.MediaId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "media item not found: %s", req.MediaId)
	}

	// Verify ownership
	if item.UserID != userID {
		return nil, status.Error(codes.PermissionDenied, "not authorized to complete this upload")
	}

	// Verify object exists in S3
	exists, err := s.s3.ObjectExists(ctx, item.S3Key)
	if err != nil {
		s.logger.Error("failed to check S3 object", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to verify upload")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "upload not found in storage — file may not have been uploaded yet")
	}

	// Update status to PROCESSING
	if err := s.store.UpdateStatus(ctx, req.MediaId, "processing", "", "", ""); err != nil {
		s.logger.Error("failed to update status to processing", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to update media status")
	}

	// Generate object URL
	objectURL := s.s3.GetObjectURL(item.S3Key)

	// Generate thumbnail for images (synchronous v1 — fast for images)
	var thumbnailURL string
	if isImageType(item.ContentType) {
		ext := filepath.Ext(item.Filename)
		thumbKey := fmt.Sprintf("%s/thumb_%s", filepath.Dir(item.S3Key), strings.TrimSuffix(filepath.Base(item.S3Key), ext)+".jpg")

		if err := GenerateThumbnail(ctx, s.s3, item.S3Key, thumbKey); err != nil {
			// Thumbnail failure is non-fatal — proceed to READY with empty thumbnail_url
			s.logger.Warn("thumbnail generation failed, proceeding without thumbnail",
				zap.String("media_id", req.MediaId),
				zap.Error(err),
			)
		} else {
			thumbnailURL = s.s3.GetObjectURL(thumbKey)
		}
	}
	// For videos: thumbnail generation deferred to future version, return empty thumbnail_url

	// Update status to READY with URLs
	if err := s.store.UpdateStatus(ctx, req.MediaId, "ready", objectURL, thumbnailURL, ""); err != nil {
		s.logger.Error("failed to update status to ready", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to finalize media")
	}

	s.logger.Info("complete upload",
		zap.String("media_id", req.MediaId),
		zap.String("user_id", userID),
		zap.String("url", objectURL),
		zap.String("thumbnail_url", thumbnailURL),
	)

	return &mediav1.CompleteUploadResponse{
		Url:          objectURL,
		ThumbnailUrl: thumbnailURL,
		Status:       mediav1.MediaStatus_MEDIA_STATUS_READY,
	}, nil
}

// GetMedia returns metadata for an uploaded media item. This is a public endpoint.
func (s *Server) GetMedia(ctx context.Context, req *mediav1.GetMediaRequest) (*mediav1.GetMediaResponse, error) {
	if req.MediaId == "" {
		return nil, status.Error(codes.InvalidArgument, "media_id is required")
	}

	item, err := s.store.Get(ctx, req.MediaId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "media item not found: %s", req.MediaId)
	}

	return &mediav1.GetMediaResponse{
		MediaId:      item.ID,
		Url:          item.URL,
		ThumbnailUrl: item.ThumbnailURL,
		MediaType:    stringToMediaType(item.MediaType),
		Status:       stringToMediaStatus(item.Status),
		ContentType:  item.ContentType,
		SizeBytes:    item.SizeBytes,
		CreatedAt:    timestamppb.New(item.CreatedAt),
	}, nil
}

// mediaTypeToString converts a proto MediaType enum to a database string.
func mediaTypeToString(mt mediav1.MediaType) string {
	switch mt {
	case mediav1.MediaType_MEDIA_TYPE_IMAGE:
		return "image"
	case mediav1.MediaType_MEDIA_TYPE_VIDEO:
		return "video"
	case mediav1.MediaType_MEDIA_TYPE_GIF:
		return "gif"
	default:
		return "image"
	}
}

// stringToMediaType converts a database string to a proto MediaType enum.
func stringToMediaType(s string) mediav1.MediaType {
	switch s {
	case "image":
		return mediav1.MediaType_MEDIA_TYPE_IMAGE
	case "video":
		return mediav1.MediaType_MEDIA_TYPE_VIDEO
	case "gif":
		return mediav1.MediaType_MEDIA_TYPE_GIF
	default:
		return mediav1.MediaType_MEDIA_TYPE_UNSPECIFIED
	}
}

// stringToMediaStatus converts a database string to a proto MediaStatus enum.
func stringToMediaStatus(s string) mediav1.MediaStatus {
	switch s {
	case "pending":
		return mediav1.MediaStatus_MEDIA_STATUS_PENDING
	case "processing":
		return mediav1.MediaStatus_MEDIA_STATUS_PROCESSING
	case "ready":
		return mediav1.MediaStatus_MEDIA_STATUS_READY
	case "failed":
		return mediav1.MediaStatus_MEDIA_STATUS_FAILED
	default:
		return mediav1.MediaStatus_MEDIA_STATUS_UNSPECIFIED
	}
}

// isImageType checks if the content type is an image type that supports thumbnailing.
func isImageType(contentType string) bool {
	ct := strings.ToLower(contentType)
	return allowedImageTypes[ct]
}
