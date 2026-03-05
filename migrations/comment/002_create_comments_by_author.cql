-- Query table: find comments by author for profile page
-- Partitioned by author_username, clustered by created_at DESC for chronological listing
CREATE TABLE IF NOT EXISTS redyx_comments.comments_by_author (
    author_username TEXT,
    created_at      TIMESTAMP,
    comment_id      UUID,
    post_id         UUID,
    body            TEXT,
    vote_score      INT,
    is_deleted      BOOLEAN,
    PRIMARY KEY (author_username, created_at, comment_id)
) WITH CLUSTERING ORDER BY (created_at DESC, comment_id ASC);
