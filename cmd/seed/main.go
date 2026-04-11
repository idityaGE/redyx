// Seed script: populates users, profiles, communities, posts, and comments
// directly into PostgreSQL and ScyllaDB.
//
// Usage:
//
//	go run ./cmd/seed [flags]
//
// Flags:
//
//	-users        number of users to create (default 20)
//	-communities  number of communities to create (default 6)
//	-posts        posts per community (default 15)
//	-comments     top-level comments per post (default 8)
//	-replies      replies per top-level comment (default 3)
//	-pg           postgres host:port (default localhost:5432)
//	-scylla       scylladb host (default localhost:9042)
//	-pg-user      postgres username (default redyx)
//	-pg-pass      postgres password (default redyx)

// Two terminals:
//   kubectl port-forward -n redyx-data postgresql-0 5432:5432
//   kubectl port-forward -n redyx-data scylladb-0 9042:9042
//
// Then seed:
//   go run ./cmd/seed/
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	numUsers := flag.Int("users", 20, "number of users")
	numCommunities := flag.Int("communities", 6, "number of communities")
	numPosts := flag.Int("posts", 15, "posts per community")
	numComments := flag.Int("comments", 8, "top-level comments per post")
	numReplies := flag.Int("replies", 3, "replies per top-level comment")
	pgHost := flag.String("pg", "localhost:5432", "postgres host:port")
	scyllaHost := flag.String("scylla", "localhost:9042", "scylladb host")
	pgUser := flag.String("pg-user", "redyx", "postgres user")
	pgPass := flag.String("pg-pass", "redyx", "postgres password")
	flag.Parse()

	fake := gofakeit.New(0)
	ctx := context.Background()

	// ── PostgreSQL connections ────────────────────────────────────────────────
	connect := func(dbName string) *pgxpool.Pool {
		dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", *pgUser, *pgPass, *pgHost, dbName)
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			log.Fatalf("connect %s: %v", dbName, err)
		}
		return pool
	}

	authDB := connect("auth")
	defer authDB.Close()
	userDB := connect("user_profiles")
	defer userDB.Close()
	communityDB := connect("community")
	defer communityDB.Close()
	shard0 := connect("posts_shard_0")
	defer shard0.Close()
	shard1 := connect("posts_shard_1")
	defer shard1.Close()

	// ── ScyllaDB connection ───────────────────────────────────────────────────
	cluster := gocql.NewCluster(*scyllaHost)
	cluster.Keyspace = "redyx_comments"
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = 10 * time.Second
	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("scylladb connect: %v", err)
	}
	defer session.Close()

	// ─────────────────────────────────────────────────────────────────────────
	// 1. USERS
	// ─────────────────────────────────────────────────────────────────────────
	type user struct {
		id       string
		username string
	}
	users := make([]user, 0, *numUsers)

	defaultPass, _ := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)

	log.Printf("seeding %d users…", *numUsers)
	for i := 0; i < *numUsers; i++ {
		id := uuid.New().String()
		username := fake.Username() + fmt.Sprintf("%d", fake.IntRange(10, 9999))
		email := fake.Email()
		displayName := fake.Name()
		bio := fake.Sentence(fake.IntRange(5, 20))
		now := time.Now()

		if _, err := authDB.Exec(ctx,
			`INSERT INTO users (id, email, username, password_hash, auth_method, is_verified, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, 'email', true, $5, $5)
			 ON CONFLICT DO NOTHING`,
			id, email, username, string(defaultPass), now,
		); err != nil {
			log.Printf("  skip user %s: %v", username, err)
			continue
		}

		if _, err := userDB.Exec(ctx,
			`INSERT INTO profiles (user_id, username, display_name, bio, avatar_url, karma, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, '', 0, $5, $5)
			 ON CONFLICT DO NOTHING`,
			id, username, displayName, bio, now,
		); err != nil {
			log.Printf("  skip profile %s: %v", username, err)
			continue
		}

		users = append(users, user{id: id, username: username})
	}
	log.Printf("  created %d users", len(users))

	if len(users) == 0 {
		log.Fatal("no users created — aborting")
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. COMMUNITIES
	// ─────────────────────────────────────────────────────────────────────────
	type community struct {
		id   string
		name string
	}
	communities := make([]community, 0, *numCommunities)

	log.Printf("seeding %d communities…", *numCommunities)
	for i := 0; i < *numCommunities; i++ {
		cid := uuid.New().String()
		// community names: alphanumeric + underscore, 3-25 chars
		name := sanitizeName(fake.Word()) + fmt.Sprintf("_%d", fake.IntRange(100, 9999))
		if len(name) > 25 {
			name = name[:25]
		}
		desc := fake.Sentence(fake.IntRange(8, 25))
		owner := users[i%len(users)]
		now := time.Now()

		if _, err := communityDB.Exec(ctx,
			`INSERT INTO communities (id, name, description, visibility, member_count, owner_id, created_at, updated_at)
			 VALUES ($1, $2, $3, 1, 1, $4, $5, $5)
			 ON CONFLICT DO NOTHING`,
			cid, name, desc, owner.id, now,
		); err != nil {
			log.Printf("  skip community %s: %v", name, err)
			continue
		}

		// owner as admin
		if _, err := communityDB.Exec(ctx,
			`INSERT INTO community_members (community_id, user_id, role, joined_at)
			 VALUES ($1, $2, 'admin', $3) ON CONFLICT DO NOTHING`,
			cid, owner.id, now,
		); err != nil {
			log.Printf("  skip owner membership %s: %v", name, err)
		}

		communities = append(communities, community{id: cid, name: name})
	}
	log.Printf("  created %d communities", len(communities))

	if len(communities) == 0 {
		log.Fatal("no communities created — aborting")
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 3. MEMBERSHIPS — every user joins every community
	// ─────────────────────────────────────────────────────────────────────────
	log.Printf("seeding memberships…")
	totalMembers := 0
	for _, c := range communities {
		for _, u := range users {
			if _, err := communityDB.Exec(ctx,
				`INSERT INTO community_members (community_id, user_id, role, joined_at)
				 VALUES ($1, $2, 'member', $3) ON CONFLICT DO NOTHING`,
				c.id, u.id, time.Now(),
			); err != nil {
				continue
			}
			totalMembers++
		}
		if _, err := communityDB.Exec(ctx,
			`UPDATE communities SET member_count = (
				SELECT COUNT(*) FROM community_members WHERE community_id = $1
			) WHERE id = $1`, c.id,
		); err != nil {
			log.Printf("  member_count update failed for %s: %v", c.name, err)
		}
	}
	log.Printf("  created %d memberships", totalMembers)

	// ─────────────────────────────────────────────────────────────────────────
	// 4. POSTS
	// ─────────────────────────────────────────────────────────────────────────
	type post struct {
		id            string
		authorID      string
		authorUsername string
	}
	var allPosts []post

	log.Printf("seeding %d posts per community…", *numPosts)
	for ci, c := range communities {
		db := shard0
		if ci%2 == 1 {
			db = shard1
		}

		for pi := 0; pi < *numPosts; pi++ {
			pid := uuid.New().String()
			author := users[fake.IntRange(0, len(users)-1)]
			title := fake.Sentence(fake.IntRange(4, 14))
			body := fake.LoremIpsumParagraph(fake.IntRange(1, 3), fake.IntRange(3, 6), fake.IntRange(6, 15), "\n\n")
			voteScore := fake.IntRange(-5, 200)
			upvotes := max(0, voteScore+fake.IntRange(0, 20))
			downvotes := max(0, upvotes-voteScore)
			createdAt := fake.DateRange(time.Now().AddDate(0, -6, 0), time.Now())

			if _, err := db.Exec(ctx,
				`INSERT INTO posts
				 (id, title, body, url, post_type, author_id, author_username,
				  community_id, community_name, vote_score, upvotes, downvotes,
				  comment_count, hot_score, is_edited, is_deleted, is_pinned,
				  is_anonymous, media_urls, thumbnail_url, created_at)
				 VALUES ($1,$2,$3,'',$4,$5,$6,$7,$8,$9,$10,$11,0,0,false,false,false,false,'{}','', $12)
				 ON CONFLICT DO NOTHING`,
				pid, title, body, 1, // post_type=1 (text)
				author.id, author.username,
				c.id, c.name,
				voteScore, upvotes, downvotes,
				createdAt,
			); err != nil {
				log.Printf("  skip post: %v", err)
				continue
			}
			allPosts = append(allPosts, post{id: pid, authorID: author.id, authorUsername: author.username})
		}
	}
	log.Printf("  created %d posts", len(allPosts))

	// ─────────────────────────────────────────────────────────────────────────
	// 5. COMMENTS (ScyllaDB)
	// ─────────────────────────────────────────────────────────────────────────
	// path counters tracked in-memory: key = "postID:parentPath"
	pathCounters := map[string]int64{}
	nextPath := func(postID, parentPath string) string {
		key := postID + ":" + parentPath
		pathCounters[key]++
		seg := fmt.Sprintf("%03d", pathCounters[key])
		if parentPath == "" {
			return seg
		}
		return parentPath + "." + seg
	}

	insertComment := func(commentID, postID, parentID, path, authorID, authorUsername, body string, depth, voteScore int, createdAt time.Time) {
		parentUUID := gocql.UUID{}
		if parentID != "" {
			if u, err := gocql.ParseUUID(parentID); err == nil {
				parentUUID = u
			}
		}
		cUUID, _ := gocql.ParseUUID(commentID)
		pUUID, _ := gocql.ParseUUID(postID)
		aUUID, _ := gocql.ParseUUID(authorID)

		// comments_by_id
		if err := session.Query(`
			INSERT INTO comments_by_id
			(comment_id, post_id, parent_id, path, depth, author_id, author_username,
			 body, vote_score, upvotes, downvotes, reply_count,
			 is_edited, is_deleted, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,false,false,?)`,
			cUUID, pUUID, parentUUID, path, depth, aUUID, authorUsername,
			body, voteScore, max(0, voteScore), 0, 0,
			createdAt,
		).Exec(); err != nil {
			log.Printf("  comments_by_id insert failed: %v", err)
		}

		// comments_by_post
		if err := session.Query(`
			INSERT INTO comments_by_post
			(post_id, path, comment_id, parent_id, depth, author_id, author_username,
			 body, vote_score, upvotes, downvotes, reply_count,
			 is_edited, is_deleted, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,false,false,?)`,
			pUUID, path, cUUID, parentUUID, depth, aUUID, authorUsername,
			body, voteScore, max(0, voteScore), 0, 0,
			createdAt,
		).Exec(); err != nil {
			log.Printf("  comments_by_post insert failed: %v", err)
		}

		// comments_by_author
		if err := session.Query(`
			INSERT INTO comments_by_author
			(author_username, created_at, comment_id, post_id, body, vote_score, is_deleted)
			VALUES (?,?,?,?,?,?,false)`,
			authorUsername, createdAt, cUUID, pUUID, body, voteScore,
		).Exec(); err != nil {
			log.Printf("  comments_by_author insert failed: %v", err)
		}
	}

	log.Printf("seeding comments (%d top-level, %d replies each)…", *numComments, *numReplies)
	totalComments := 0

	for _, p := range allPosts {
		for ci := 0; ci < *numComments; ci++ {
			author := users[fake.IntRange(0, len(users)-1)]
			body := fake.Sentence(fake.IntRange(5, 40))
			voteScore := fake.IntRange(0, 50)
			createdAt := fake.DateRange(time.Now().AddDate(0, -3, 0), time.Now())

			path := nextPath(p.id, "")
			cid := uuid.New().String()
			insertComment(cid, p.id, "", path, author.id, author.username, body, 1, voteScore, createdAt)
			totalComments++

			for ri := 0; ri < *numReplies; ri++ {
				replyAuthor := users[fake.IntRange(0, len(users)-1)]
				replyBody := fake.Sentence(fake.IntRange(4, 25))
				replyScore := fake.IntRange(0, 20)
				replyCreatedAt := createdAt.Add(time.Duration(fake.IntRange(1, 3600)) * time.Second)

				replyPath := nextPath(p.id, path)
				rid := uuid.New().String()
				insertComment(rid, p.id, cid, replyPath, replyAuthor.id, replyAuthor.username, replyBody, 2, replyScore, replyCreatedAt)
				totalComments++
			}
		}
	}
	log.Printf("  created %d comments", totalComments)

	log.Println("seed complete ✓")
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if len(result) < 3 {
		result = result + "xyz"
	}
	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
