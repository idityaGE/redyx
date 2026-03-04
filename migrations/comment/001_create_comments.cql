-- Keyspace
CREATE KEYSPACE IF NOT EXISTS redyx_comments
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

-- Primary query table: all comments for a post in tree order
CREATE TABLE IF NOT EXISTS redyx_comments.comments_by_post (
    post_id       UUID,
    comment_id    UUID,
    parent_id     UUID,
    author_id     UUID,
    author_username TEXT,
    body          TEXT,
    path          TEXT,
    depth         INT,
    vote_score    INT,
    upvotes       INT,
    downvotes     INT,
    reply_count   INT,
    is_edited     BOOLEAN,
    is_deleted    BOOLEAN,
    created_at    TIMESTAMP,
    edited_at     TIMESTAMP,
    PRIMARY KEY (post_id, path)
) WITH CLUSTERING ORDER BY (path ASC);

-- Lookup table: find comment by ID (for Get/Update/Delete without knowing post_id)
CREATE TABLE IF NOT EXISTS redyx_comments.comments_by_id (
    comment_id    UUID,
    post_id       UUID,
    parent_id     UUID,
    author_id     UUID,
    author_username TEXT,
    body          TEXT,
    path          TEXT,
    depth         INT,
    vote_score    INT,
    upvotes       INT,
    downvotes     INT,
    reply_count   INT,
    is_edited     BOOLEAN,
    is_deleted    BOOLEAN,
    created_at    TIMESTAMP,
    edited_at     TIMESTAMP,
    PRIMARY KEY (comment_id)
);

-- Counter table: atomic path segment generation per parent
CREATE TABLE IF NOT EXISTS redyx_comments.comment_path_counters (
    post_id     UUID,
    parent_path TEXT,
    counter     COUNTER,
    PRIMARY KEY (post_id, parent_path)
);
