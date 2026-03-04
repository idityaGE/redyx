package notification

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	postv1 "github.com/redyx/redyx/gen/redyx/post/v1"
)

// PostResolver resolves post metadata (author, community) via gRPC.
type PostResolver struct {
	client postv1.PostServiceClient
	conn   *grpc.ClientConn
	logger *zap.Logger
}

// PostInfo contains resolved post metadata for notification enrichment.
type PostInfo struct {
	AuthorID       string
	AuthorUsername string
	CommunityName  string
}

// NewPostResolver creates a gRPC client to the post-service.
func NewPostResolver(postServiceAddr string, logger *zap.Logger) (*PostResolver, error) {
	conn, err := grpc.NewClient(postServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to post-service: %w", err)
	}

	return &PostResolver{
		client: postv1.NewPostServiceClient(conn),
		conn:   conn,
		logger: logger,
	}, nil
}

// Resolve fetches post metadata by post ID.
func (r *PostResolver) Resolve(ctx context.Context, postID string) (*PostInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := r.client.GetPost(ctx, &postv1.GetPostRequest{PostId: postID})
	if err != nil {
		return nil, fmt.Errorf("get post: %w", err)
	}

	post := resp.GetPost()
	return &PostInfo{
		AuthorID:       post.GetAuthorId(),
		AuthorUsername: post.GetAuthorUsername(),
		CommunityName:  post.GetCommunityName(),
	}, nil
}

// Close shuts down the gRPC connection.
func (r *PostResolver) Close() {
	if r.conn != nil {
		r.conn.Close()
	}
}
