// Package moderation implements the moderation gRPC service for community
// content moderation, user bans, mod log, and report queue management.
package moderation

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReportRecord represents a report row from the database.
type ReportRecord struct {
	ID             string
	CommunityID    string
	CommunityName  string
	ContentID      string
	ContentType    int16
	ReporterID     string
	Reason         string
	Source         string
	Status         string
	ResolvedAction *string
	ResolvedBy     *string
	ResolvedAt     *time.Time
	CreatedAt      time.Time
	// Aggregated field (from GROUP BY queries).
	ReportCount int32
}

// BanRecord represents a ban row from the database.
type BanRecord struct {
	ID               string
	CommunityID      string
	CommunityName    string
	UserID           string
	Username         string
	Reason           string
	BannedBy         string
	BannedByUsername string
	DurationSeconds  int64
	ExpiresAt        *time.Time
	CreatedAt        time.Time
}

// ModLogRecord represents a mod_log row from the database.
type ModLogRecord struct {
	ID                string
	CommunityID       string
	CommunityName     string
	ModeratorID       string
	ModeratorUsername string
	Action            string
	TargetID          string
	TargetType        string
	Reason            string
	CreatedAt         time.Time
}

// Store provides PostgreSQL storage for moderation data.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a new moderation store backed by PostgreSQL.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateReport inserts a new report and returns its ID.
func (s *Store) CreateReport(ctx context.Context, r *ReportRecord) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO reports (community_id, community_name, content_id, content_type, reporter_id, reason, source)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		r.CommunityID, r.CommunityName, r.ContentID, r.ContentType,
		r.ReporterID, r.Reason, r.Source,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert report: %w", err)
	}
	return id, nil
}

// ListReports returns aggregated reports for a community, grouped by content_id and content_type.
// Returns one entry per unique content item with report_count, sorted by report_count DESC.
func (s *Store) ListReports(ctx context.Context, communityID, status, source string, limit, offset int) ([]ReportRecord, int, error) {
	// Build count query
	countQuery := `SELECT COUNT(DISTINCT (content_id, content_type)) FROM reports WHERE community_id = $1`
	countArgs := []any{communityID}
	argIdx := 2

	if status != "" {
		countQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
		countArgs = append(countArgs, status)
		argIdx++
	}
	if source != "" {
		countQuery += fmt.Sprintf(` AND source = $%d`, argIdx)
		countArgs = append(countArgs, source)
	}

	var totalCount int
	if err := s.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count reports: %w", err)
	}

	// Build main query with GROUP BY
	query := `SELECT
		MIN(id::text) as report_id,
		content_id,
		content_type,
		MIN(reporter_id::text) as reporter_id,
		MIN(reason) as reason,
		COUNT(*) as report_count,
		MAX(created_at) as latest_report,
		MIN(source) as source,
		MIN(status) as status,
		MIN(resolved_action) as resolved_action
	FROM reports
	WHERE community_id = $1`
	args := []any{communityID}
	argIdx = 2

	if status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}
	if source != "" {
		query += fmt.Sprintf(` AND source = $%d`, argIdx)
		args = append(args, source)
		argIdx++
	}

	query += ` GROUP BY content_id, content_type
		ORDER BY COUNT(*) DESC, MAX(created_at) DESC
		LIMIT $` + fmt.Sprintf("%d", argIdx) + ` OFFSET $` + fmt.Sprintf("%d", argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	var reports []ReportRecord
	for rows.Next() {
		var r ReportRecord
		if err := rows.Scan(
			&r.ID, &r.ContentID, &r.ContentType, &r.ReporterID,
			&r.Reason, &r.ReportCount, &r.CreatedAt, &r.Source,
			&r.Status, &r.ResolvedAction,
		); err != nil {
			return nil, 0, fmt.Errorf("scan report: %w", err)
		}
		reports = append(reports, r)
	}

	return reports, totalCount, nil
}

// UpdateReportStatus marks all reports for a content item as resolved.
func (s *Store) UpdateReportStatus(ctx context.Context, contentID string, contentType int16, action string, resolvedBy string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE reports
		 SET status = 'resolved', resolved_action = $1, resolved_by = $2, resolved_at = now()
		 WHERE content_id = $3 AND content_type = $4 AND status = 'active'`,
		action, resolvedBy, contentID, contentType,
	)
	if err != nil {
		return fmt.Errorf("update report status: %w", err)
	}
	return nil
}

// CreateBan inserts or replaces a ban record. Uses ON CONFLICT to handle re-bans.
func (s *Store) CreateBan(ctx context.Context, b *BanRecord) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO bans (community_id, community_name, user_id, username, reason, banned_by, banned_by_username, duration_seconds, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (community_id, user_id) DO UPDATE SET
		   reason = EXCLUDED.reason,
		   banned_by = EXCLUDED.banned_by,
		   banned_by_username = EXCLUDED.banned_by_username,
		   duration_seconds = EXCLUDED.duration_seconds,
		   expires_at = EXCLUDED.expires_at,
		   created_at = now()
		 RETURNING id`,
		b.CommunityID, b.CommunityName, b.UserID, b.Username,
		b.Reason, b.BannedBy, b.BannedByUsername, b.DurationSeconds, b.ExpiresAt,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create ban: %w", err)
	}
	return id, nil
}

// DeleteBan removes a ban record for a user in a community.
func (s *Store) DeleteBan(ctx context.Context, communityID, userID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM bans WHERE community_id = $1 AND user_id = $2`,
		communityID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete ban: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("ban not found")
	}
	return nil
}

// GetBan returns the active ban for a user in a community, or nil if not banned.
func (s *Store) GetBan(ctx context.Context, communityID, userID string) (*BanRecord, error) {
	var b BanRecord
	err := s.pool.QueryRow(ctx,
		`SELECT id, community_id, community_name, user_id, username, reason, banned_by, banned_by_username,
		        duration_seconds, expires_at, created_at
		 FROM bans
		 WHERE community_id = $1 AND user_id = $2
		   AND (expires_at IS NULL OR expires_at > now())`,
		communityID, userID,
	).Scan(&b.ID, &b.CommunityID, &b.CommunityName, &b.UserID, &b.Username,
		&b.Reason, &b.BannedBy, &b.BannedByUsername, &b.DurationSeconds, &b.ExpiresAt, &b.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ban: %w", err)
	}
	return &b, nil
}

// ListBans returns active bans for a community (expired bans auto-filtered).
func (s *Store) ListBans(ctx context.Context, communityID string, limit, offset int) ([]BanRecord, int, error) {
	var totalCount int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM bans
		 WHERE community_id = $1 AND (expires_at IS NULL OR expires_at > now())`,
		communityID,
	).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("count bans: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, community_id, community_name, user_id, username, reason, banned_by, banned_by_username,
		        duration_seconds, expires_at, created_at
		 FROM bans
		 WHERE community_id = $1 AND (expires_at IS NULL OR expires_at > now())
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		communityID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list bans: %w", err)
	}
	defer rows.Close()

	var bans []BanRecord
	for rows.Next() {
		var b BanRecord
		if err := rows.Scan(&b.ID, &b.CommunityID, &b.CommunityName, &b.UserID, &b.Username,
			&b.Reason, &b.BannedBy, &b.BannedByUsername, &b.DurationSeconds, &b.ExpiresAt, &b.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan ban: %w", err)
		}
		bans = append(bans, b)
	}

	return bans, totalCount, nil
}

// CreateModLogEntry inserts a moderation action log entry.
func (s *Store) CreateModLogEntry(ctx context.Context, entry *ModLogRecord) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO mod_log (community_id, community_name, moderator_id, moderator_username, action, target_id, target_type, reason)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id`,
		entry.CommunityID, entry.CommunityName, entry.ModeratorID, entry.ModeratorUsername,
		entry.Action, entry.TargetID, entry.TargetType, entry.Reason,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create mod log entry: %w", err)
	}
	return id, nil
}

// ListModLog returns paginated mod log entries for a community, optionally filtered by action.
func (s *Store) ListModLog(ctx context.Context, communityID, actionFilter string, limit, offset int) ([]ModLogRecord, int, error) {
	// Count
	countQuery := `SELECT COUNT(*) FROM mod_log WHERE community_id = $1`
	countArgs := []any{communityID}
	if actionFilter != "" {
		countQuery += ` AND action = $2`
		countArgs = append(countArgs, actionFilter)
	}

	var totalCount int
	if err := s.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count mod log: %w", err)
	}

	// Query
	query := `SELECT id, community_id, community_name, moderator_id, moderator_username, action, target_id, target_type, reason, created_at
		FROM mod_log WHERE community_id = $1`
	args := []any{communityID}
	argIdx := 2

	if actionFilter != "" {
		query += fmt.Sprintf(` AND action = $%d`, argIdx)
		args = append(args, actionFilter)
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list mod log: %w", err)
	}
	defer rows.Close()

	var entries []ModLogRecord
	for rows.Next() {
		var e ModLogRecord
		if err := rows.Scan(&e.ID, &e.CommunityID, &e.CommunityName, &e.ModeratorID, &e.ModeratorUsername,
			&e.Action, &e.TargetID, &e.TargetType, &e.Reason, &e.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan mod log: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, totalCount, nil
}
