package comment

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"go.uber.org/zap"
)

// Comment is the internal domain model for a comment row.
type Comment struct {
	CommentID      gocql.UUID
	PostID         gocql.UUID
	ParentID       gocql.UUID
	AuthorID       gocql.UUID
	AuthorUsername string
	Body           string
	Path           string
	DepthVal       int
	VoteScore      int
	Upvotes        int
	Downvotes      int
	ReplyCount     int
	IsEdited       bool
	IsDeleted      bool
	CreatedAt      time.Time
	EditedAt       time.Time
}

// Store provides ScyllaDB CRUD operations for comments.
type Store struct {
	session *gocql.Session
	logger  *zap.Logger
}

// NewStore creates a new comment store backed by a ScyllaDB session.
func NewStore(session *gocql.Session, logger *zap.Logger) *Store {
	return &Store{session: session, logger: logger}
}

// RunMigrations reads .cql file(s) from migrationsDir, splits on ";",
// and executes each statement. CREATE IF NOT EXISTS is idempotent.
func RunMigrations(session *gocql.Session, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read migrations dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".cql") {
			continue
		}
		data, err := os.ReadFile(migrationsDir + "/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		statements := strings.Split(string(data), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			// Skip comment-only lines
			lines := strings.Split(stmt, "\n")
			var cleaned []string
			for _, l := range lines {
				l = strings.TrimSpace(l)
				if l != "" && !strings.HasPrefix(l, "--") {
					cleaned = append(cleaned, l)
				}
			}
			if len(cleaned) == 0 {
				continue
			}
			if err := session.Query(stmt).Exec(); err != nil {
				return fmt.Errorf("exec migration statement: %w", err)
			}
		}
	}
	return nil
}

// CreateComment creates a new comment with materialized path generation.
func (s *Store) CreateComment(ctx context.Context, postID, parentID, authorID, authorUsername, body string) (*Comment, error) {
	postUUID, err := gocql.ParseUUID(postID)
	if err != nil {
		return nil, fmt.Errorf("invalid post_id: %w", err)
	}

	var parentUUID gocql.UUID
	var parentPath string

	// If replying to a parent, look up parent's path
	if parentID != "" {
		parentUUID, err = gocql.ParseUUID(parentID)
		if err != nil {
			return nil, fmt.Errorf("invalid parent_id: %w", err)
		}
		parent, err := s.GetComment(ctx, parentID)
		if err != nil {
			return nil, fmt.Errorf("parent comment not found: %w", err)
		}
		parentPath = parent.Path
	}

	// Increment counter for this parent path
	if err := s.session.Query(
		`UPDATE redyx_comments.comment_path_counters SET counter = counter + 1 WHERE post_id = ? AND parent_path = ?`,
		postUUID, parentPath,
	).WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("increment path counter: %w", err)
	}

	// Read counter back
	var counter int64
	if err := s.session.Query(
		`SELECT counter FROM redyx_comments.comment_path_counters WHERE post_id = ? AND parent_path = ?`,
		postUUID, parentPath,
	).WithContext(ctx).Scan(&counter); err != nil {
		return nil, fmt.Errorf("read path counter: %w", err)
	}

	// Generate path
	path := NextPath(parentPath, counter)
	depth := Depth(path)
	commentID := gocql.TimeUUID()
	now := time.Now().UTC()

	authorUUID, err := gocql.ParseUUID(authorID)
	if err != nil {
		return nil, fmt.Errorf("invalid author_id: %w", err)
	}

	// Insert into comments_by_post
	if err := s.session.Query(
		`INSERT INTO redyx_comments.comments_by_post
		 (post_id, comment_id, parent_id, author_id, author_username, body, path, depth,
		  vote_score, upvotes, downvotes, reply_count, is_edited, is_deleted, created_at, edited_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		postUUID, commentID, parentUUID, authorUUID, authorUsername, body, path, depth,
		0, 0, 0, 0, false, false, now, now,
	).WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("insert comments_by_post: %w", err)
	}

	// Insert into comments_by_id
	if err := s.session.Query(
		`INSERT INTO redyx_comments.comments_by_id
		 (comment_id, post_id, parent_id, author_id, author_username, body, path, depth,
		  vote_score, upvotes, downvotes, reply_count, is_edited, is_deleted, created_at, edited_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		commentID, postUUID, parentUUID, authorUUID, authorUsername, body, path, depth,
		0, 0, 0, 0, false, false, now, now,
	).WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("insert comments_by_id: %w", err)
	}

	// Insert into comments_by_author for profile page queries
	if err := s.session.Query(
		`INSERT INTO redyx_comments.comments_by_author
		 (author_username, created_at, comment_id, post_id, body, vote_score, is_deleted)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		authorUsername, now, commentID, postUUID, body, 0, false,
	).WithContext(ctx).Exec(); err != nil {
		s.logger.Warn("failed to insert into comments_by_author", zap.Error(err))
	}

	// If replying, increment parent's reply_count in both tables
	if parentID != "" {
		// Read current reply_count
		var currentCount int
		if err := s.session.Query(
			`SELECT reply_count FROM redyx_comments.comments_by_id WHERE comment_id = ?`,
			parentUUID,
		).WithContext(ctx).Scan(&currentCount); err != nil {
			s.logger.Warn("failed to read parent reply_count", zap.Error(err))
		} else {
			newCount := currentCount + 1
			// Update in comments_by_id
			if err := s.session.Query(
				`UPDATE redyx_comments.comments_by_id SET reply_count = ? WHERE comment_id = ?`,
				newCount, parentUUID,
			).WithContext(ctx).Exec(); err != nil {
				s.logger.Warn("failed to update parent reply_count in comments_by_id", zap.Error(err))
			}

			// Look up parent's post_id and path for comments_by_post update
			var parentPostID gocql.UUID
			var parentPathStr string
			if err := s.session.Query(
				`SELECT post_id, path FROM redyx_comments.comments_by_id WHERE comment_id = ?`,
				parentUUID,
			).WithContext(ctx).Scan(&parentPostID, &parentPathStr); err == nil {
				if err := s.session.Query(
					`UPDATE redyx_comments.comments_by_post SET reply_count = ? WHERE post_id = ? AND path = ?`,
					newCount, parentPostID, parentPathStr,
				).WithContext(ctx).Exec(); err != nil {
					s.logger.Warn("failed to update parent reply_count in comments_by_post", zap.Error(err))
				}
			}
		}
	}

	return &Comment{
		CommentID:      commentID,
		PostID:         postUUID,
		ParentID:       parentUUID,
		AuthorID:       authorUUID,
		AuthorUsername: authorUsername,
		Body:           body,
		Path:           path,
		DepthVal:       depth,
		VoteScore:      0,
		Upvotes:        0,
		Downvotes:      0,
		ReplyCount:     0,
		IsEdited:       false,
		IsDeleted:      false,
		CreatedAt:      now,
		EditedAt:       now,
	}, nil
}

// GetComment retrieves a single comment by ID.
func (s *Store) GetComment(ctx context.Context, commentID string) (*Comment, error) {
	id, err := gocql.ParseUUID(commentID)
	if err != nil {
		return nil, fmt.Errorf("invalid comment_id: %w", err)
	}

	c := &Comment{}
	if err := s.session.Query(
		`SELECT comment_id, post_id, parent_id, author_id, author_username, body, path, depth,
		        vote_score, upvotes, downvotes, reply_count, is_edited, is_deleted, created_at, edited_at
		 FROM redyx_comments.comments_by_id WHERE comment_id = ?`, id,
	).WithContext(ctx).Scan(
		&c.CommentID, &c.PostID, &c.ParentID, &c.AuthorID, &c.AuthorUsername, &c.Body,
		&c.Path, &c.DepthVal, &c.VoteScore, &c.Upvotes, &c.Downvotes, &c.ReplyCount,
		&c.IsEdited, &c.IsDeleted, &c.CreatedAt, &c.EditedAt,
	); err != nil {
		if err == gocql.ErrNotFound {
			return nil, fmt.Errorf("comment %q: not found", commentID)
		}
		return nil, fmt.Errorf("get comment: %w", err)
	}
	return c, nil
}

// UpdateComment updates the body of a comment in both tables.
func (s *Store) UpdateComment(ctx context.Context, commentID, body string) (*Comment, error) {
	c, err := s.GetComment(ctx, commentID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	// Update comments_by_id
	if err := s.session.Query(
		`UPDATE redyx_comments.comments_by_id SET body = ?, is_edited = true, edited_at = ? WHERE comment_id = ?`,
		body, now, c.CommentID,
	).WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("update comments_by_id: %w", err)
	}

	// Update comments_by_post
	if err := s.session.Query(
		`UPDATE redyx_comments.comments_by_post SET body = ?, is_edited = true, edited_at = ? WHERE post_id = ? AND path = ?`,
		body, now, c.PostID, c.Path,
	).WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("update comments_by_post: %w", err)
	}

	c.Body = body
	c.IsEdited = true
	c.EditedAt = now
	return c, nil
}

// DeleteComment performs a soft delete: sets body to "[deleted]" and preserves thread structure.
func (s *Store) DeleteComment(ctx context.Context, commentID string) error {
	c, err := s.GetComment(ctx, commentID)
	if err != nil {
		return err
	}

	emptyUUID := gocql.UUID{}

	// Soft delete in comments_by_id
	if err := s.session.Query(
		`UPDATE redyx_comments.comments_by_id SET is_deleted = true, body = ?, author_username = ?, author_id = ? WHERE comment_id = ?`,
		"[deleted]", "[deleted]", emptyUUID, c.CommentID,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("delete comments_by_id: %w", err)
	}

	// Soft delete in comments_by_post
	if err := s.session.Query(
		`UPDATE redyx_comments.comments_by_post SET is_deleted = true, body = ?, author_username = ?, author_id = ? WHERE post_id = ? AND path = ?`,
		"[deleted]", "[deleted]", emptyUUID, c.PostID, c.Path,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("delete comments_by_post: %w", err)
	}

	// Soft delete in comments_by_author
	if c.AuthorUsername != "" && c.AuthorUsername != "[deleted]" {
		if err := s.session.Query(
			`UPDATE redyx_comments.comments_by_author SET is_deleted = true, body = ? WHERE author_username = ? AND created_at = ? AND comment_id = ?`,
			"[deleted]", c.AuthorUsername, c.CreatedAt, c.CommentID,
		).WithContext(ctx).Exec(); err != nil {
			s.logger.Warn("failed to soft-delete in comments_by_author", zap.Error(err))
		}
	}

	return nil
}

// ListComments retrieves comments for a post with sorting and pagination.
// Returns top-level comments sorted by the given order, plus child comments
// up to 3 levels deep in tree-display order.
func (s *Store) ListComments(ctx context.Context, postID string, sort CommentSortOrder, limit int, cursor string) ([]*Comment, string, int, error) {
	postUUID, err := gocql.ParseUUID(postID)
	if err != nil {
		return nil, "", 0, fmt.Errorf("invalid post_id: %w", err)
	}

	// Fetch all comments for this post
	iter := s.session.Query(
		`SELECT comment_id, post_id, parent_id, author_id, author_username, body, path, depth,
		        vote_score, upvotes, downvotes, reply_count, is_edited, is_deleted, created_at, edited_at
		 FROM redyx_comments.comments_by_post WHERE post_id = ?`, postUUID,
	).WithContext(ctx).Iter()

	var allComments []*Comment
	for {
		c := &Comment{}
		if !iter.Scan(
			&c.CommentID, &c.PostID, &c.ParentID, &c.AuthorID, &c.AuthorUsername, &c.Body,
			&c.Path, &c.DepthVal, &c.VoteScore, &c.Upvotes, &c.Downvotes, &c.ReplyCount,
			&c.IsEdited, &c.IsDeleted, &c.CreatedAt, &c.EditedAt,
		) {
			break
		}
		allComments = append(allComments, c)
	}
	if err := iter.Close(); err != nil {
		return nil, "", 0, fmt.Errorf("list comments iter: %w", err)
	}

	// Separate top-level comments (depth == 1)
	var topLevel []*Comment
	childMap := make(map[string][]*Comment) // parentPath -> children
	for _, c := range allComments {
		if c.DepthVal == 1 {
			topLevel = append(topLevel, c)
		} else {
			pp := ParentPath(c.Path)
			childMap[pp] = append(childMap[pp], c)
		}
	}

	// Sort top-level comments
	sortComments(topLevel, sort)

	totalCount := len(topLevel)

	// Apply cursor-based pagination on top-level
	startIdx := 0
	if cursor != "" {
		for i, c := range topLevel {
			if c.CommentID.String() == cursor {
				startIdx = i + 1
				break
			}
		}
	}

	endIdx := startIdx + limit
	if endIdx > len(topLevel) {
		endIdx = len(topLevel)
	}

	page := topLevel[startIdx:endIdx]

	// Build result with nested children (up to depth 3)
	var result []*Comment
	for _, tlc := range page {
		result = append(result, tlc)
		// Add children up to depth 3
		result = appendChildren(result, tlc.Path, allComments, 3)
	}

	// Generate next cursor
	var nextCursor string
	if endIdx < len(topLevel) {
		nextCursor = topLevel[endIdx-1].CommentID.String()
	}

	return result, nextCursor, totalCount, nil
}

// appendChildren appends child comments of the given parentPath up to maxDepth.
func appendChildren(result []*Comment, parentPath string, all []*Comment, maxDepth int) []*Comment {
	for _, c := range all {
		if IsDescendant(c.Path, parentPath) && c.DepthVal <= maxDepth {
			result = append(result, c)
		}
	}
	return result
}

// ListReplies retrieves replies to a specific comment with pagination.
func (s *Store) ListReplies(ctx context.Context, commentID string, limit int, cursor string) ([]*Comment, string, error) {
	parent, err := s.GetComment(ctx, commentID)
	if err != nil {
		return nil, "", err
	}

	// Query descendants using path range scan
	// path > '{parentPath}.' AND path < '{parentPath}/' captures all descendants
	// because '/' is the next ASCII character after '.'
	pathPrefix := parent.Path + "."
	pathEnd := parent.Path + "/"

	iter := s.session.Query(
		`SELECT comment_id, post_id, parent_id, author_id, author_username, body, path, depth,
		        vote_score, upvotes, downvotes, reply_count, is_edited, is_deleted, created_at, edited_at
		 FROM redyx_comments.comments_by_post WHERE post_id = ? AND path >= ? AND path < ?`,
		parent.PostID, pathPrefix, pathEnd,
	).WithContext(ctx).Iter()

	var allReplies []*Comment
	parentDepth := parent.DepthVal
	for {
		c := &Comment{}
		if !iter.Scan(
			&c.CommentID, &c.PostID, &c.ParentID, &c.AuthorID, &c.AuthorUsername, &c.Body,
			&c.Path, &c.DepthVal, &c.VoteScore, &c.Upvotes, &c.Downvotes, &c.ReplyCount,
			&c.IsEdited, &c.IsDeleted, &c.CreatedAt, &c.EditedAt,
		) {
			break
		}
		// Filter to relative depth <= 3 from parent
		if c.DepthVal-parentDepth <= 3 {
			allReplies = append(allReplies, c)
		}
	}
	if err := iter.Close(); err != nil {
		return nil, "", fmt.Errorf("list replies iter: %w", err)
	}

	// Apply cursor-based pagination
	startIdx := 0
	if cursor != "" {
		for i, c := range allReplies {
			if c.CommentID.String() == cursor {
				startIdx = i + 1
				break
			}
		}
	}

	endIdx := startIdx + limit
	if endIdx > len(allReplies) {
		endIdx = len(allReplies)
	}

	page := allReplies[startIdx:endIdx]

	var nextCursor string
	if endIdx < len(allReplies) && len(page) > 0 {
		nextCursor = page[len(page)-1].CommentID.String()
	}

	return page, nextCursor, nil
}

// UpdateVoteScore updates the vote scores for a comment in both tables.
func (s *Store) UpdateVoteScore(ctx context.Context, commentID string, voteScore, upvotes, downvotes int) error {
	c, err := s.GetComment(ctx, commentID)
	if err != nil {
		return err
	}

	// Update comments_by_id
	if err := s.session.Query(
		`UPDATE redyx_comments.comments_by_id SET vote_score = ?, upvotes = ?, downvotes = ? WHERE comment_id = ?`,
		voteScore, upvotes, downvotes, c.CommentID,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("update vote score comments_by_id: %w", err)
	}

	// Update comments_by_post
	if err := s.session.Query(
		`UPDATE redyx_comments.comments_by_post SET vote_score = ?, upvotes = ?, downvotes = ? WHERE post_id = ? AND path = ?`,
		voteScore, upvotes, downvotes, c.PostID, c.Path,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("update vote score comments_by_post: %w", err)
	}

	return nil
}

// CommentSortOrder defines how comments are sorted.
type CommentSortOrder int

const (
	SortBest CommentSortOrder = iota + 1
	SortTop
	SortNew
	SortControversial
)

// sortComments sorts a slice of comments in-place by the given order.
func sortComments(comments []*Comment, order CommentSortOrder) {
	switch order {
	case SortBest:
		sortByWilson(comments)
	case SortTop:
		sortByVoteScore(comments)
	case SortNew:
		sortByCreatedAt(comments)
	default:
		sortByWilson(comments)
	}
}

func sortByWilson(comments []*Comment) {
	for i := 1; i < len(comments); i++ {
		for j := i; j > 0; j-- {
			a := WilsonScore(comments[j].Upvotes, comments[j].Downvotes)
			b := WilsonScore(comments[j-1].Upvotes, comments[j-1].Downvotes)
			if a > b {
				comments[j], comments[j-1] = comments[j-1], comments[j]
			}
		}
	}
}

func sortByVoteScore(comments []*Comment) {
	for i := 1; i < len(comments); i++ {
		for j := i; j > 0; j-- {
			if comments[j].VoteScore > comments[j-1].VoteScore {
				comments[j], comments[j-1] = comments[j-1], comments[j]
			}
		}
	}
}

func sortByCreatedAt(comments []*Comment) {
	for i := 1; i < len(comments); i++ {
		for j := i; j > 0; j-- {
			if comments[j].CreatedAt.After(comments[j-1].CreatedAt) {
				comments[j], comments[j-1] = comments[j-1], comments[j]
			}
		}
	}
}
