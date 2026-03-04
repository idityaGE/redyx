package notification

import "time"

// CommentEvent represents a comment creation event consumed from Kafka.
// This mirrors the CommentEvent proto definition in proto/redyx/common/v1/events.proto.
// Once the proto is regenerated and the comment service produces these events,
// this struct can be replaced with the generated protobuf type.
type CommentEvent struct {
	EventID               string    `json:"event_id"`
	CommentID             string    `json:"comment_id"`
	PostID                string    `json:"post_id"`
	AuthorID              string    `json:"author_id"`
	AuthorUsername        string    `json:"author_username"`
	ParentCommentID       string    `json:"parent_comment_id"`        // empty if top-level comment
	ParentCommentAuthorID string    `json:"parent_comment_author_id"` // author of parent comment
	PostAuthorID          string    `json:"post_author_id"`           // author of the post
	CommunityName         string    `json:"community_name"`
	Body                  string    `json:"body"`
	CreatedAt             time.Time `json:"created_at"`
}
