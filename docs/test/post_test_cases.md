# Post Service — Test Case Specification

| | |
|---|---|
| **Project Name** | Redyx |
| **Module Name** | Post |
| **File** | `internal/post/server_test.go` |
| **Total Tests** | 16 |
| **Package** | `post` (same-package test) |
| **Created By** | idityaGE |
| **Date of Creation** | 2026-05-10 |

---

## Test Infrastructure

| Component | Implementation |
|---|---|
| Logger | `zap.NewNop()` |
| DB (ShardRouter) | `&ShardRouter{}` (empty struct — no live DSNs) |
| Auth Context | `auth.WithClaims(context.Background(), &auth.Claims{…})` |
| Helper convention | `t.Helper()` in `testPostServer` |
| Nil tolerance | Tests short-circuit before any DB/shard access |

---

## 1. HotScore / RisingScore Ranking Algorithm Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_POST_001 | `TestHotScore_NewPost_HighScore` | — | Call `HotScore(10, time.Now())` | votes = 10, postedAt = now | Returned score > 0 | PASS |
| TC_POST_002 | `TestHotScore_OlderPost_LowerScore` | — | Compute `HotScore` for same vote count at two different times; compare | votes = 10, recent vs 24h old | Recent post score > old post score | PASS |
| TC_POST_003 | `TestHotScore_ZeroScore_StillPositive` | — | Call `HotScore(0, time.Now())` | votes = 0, postedAt = now | Returned score > 0 (age-based floor prevents zero) | PASS |
| TC_POST_004 | `TestRisingScore_PositiveVotes` | — | Call `RisingScore(100, time.Now().Add(-1h))` | votes = 100, postedAt = 1h ago | Returned score > 0 | PASS |
| TC_POST_005 | `TestRisingScore_ZeroVotes` | — | Call `RisingScore(0, time.Now())` | votes = 0 | Returned score = 0 | PASS |

---

## 2. CreatePost Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_POST_006 | `TestCreatePost_Unauthenticated` | Post server initialized | Call `CreatePost` with unauthenticated `context.Background()` | title = `"Valid Title"`, community = `"testcomm"` | Returns an error (missing auth claims) | PASS |
| TC_POST_007 | `TestCreatePost_EmptyTitle` | Authenticated context | Call `CreatePost` with empty title | title = `""`, community = `"testcomm"` | Returns an error | PASS |
| TC_POST_008 | `TestCreatePost_TitleTooLong` | Authenticated context | Call `CreatePost` with 301-character title | title = 301×`"a"` | Returns an error (max 300 chars) | PASS |
| TC_POST_009 | `TestCreatePost_EmptyCommunityName` | Authenticated context | Call `CreatePost` with empty community_name | title = `"Valid Title"`, community = `""` | Returns an error | PASS |
| TC_POST_010 | `TestCreatePost_LinkType_MissingURL` | Authenticated context | Call `CreatePost` with `POST_TYPE_LINK` but empty URL | postType = LINK, url = `""` | Returns an error | PASS |
| TC_POST_011 | `TestCreatePost_LinkType_InvalidURL` | Authenticated context | Call `CreatePost` with `POST_TYPE_LINK` and malformed URL | postType = LINK, url = `"not a valid url"` | Returns an error | PASS |
| TC_POST_012 | `TestCreatePost_MediaType_NoMediaIDs` | Authenticated context | Call `CreatePost` with `POST_TYPE_MEDIA` and nil `media_ids` | postType = MEDIA, mediaIds = nil | Returns an error | PASS |
| TC_POST_013 | `TestCreatePost_TextBody_TooLong` | Authenticated context | Call `CreatePost` with text body of 40 001 chars | body = 40 001×`"x"` | Returns an error (max 40 000 chars) | PASS |

---

## 3. GetPost Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_POST_014 | `TestGetPost_EmptyPostID` | Post server initialized | Call `GetPost` with empty post_id | postId = `""` | Returns an error | PASS |

---

## 4. UpdatePost Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_POST_015 | `TestUpdatePost_Unauthenticated` | Post server initialized | Call `UpdatePost` with unauthenticated context | postId = `"post-123"`, title = `"New Title"` | Returns an error | PASS |
| TC_POST_016 | `TestUpdatePost_EmptyPostID` | Authenticated context | Call `UpdatePost` with empty post_id | postId = `""`, title = `"New Title"` | Returns an error | PASS |
| TC_POST_017 | `TestUpdatePost_TitleTooLong` | Authenticated context | Call `UpdatePost` with 301-character title | title = 301×`"a"` | Returns an error (max 300 chars) | PASS |

---

## 5. DeletePost Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_POST_018 | `TestDeletePost_Unauthenticated` | Post server initialized | Call `DeletePost` with unauthenticated context | postId = `"post-123"` | Returns an error | PASS |
| TC_POST_019 | `TestDeletePost_EmptyPostID` | Authenticated context | Call `DeletePost` with empty post_id | postId = `""` | Returns an error | PASS |

---

## 6. ListPosts Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_POST_020 | `TestListPosts_EmptyCommunityName` | Post server initialized | Call `ListPosts` with empty community_name | community = `""` | Returns an error | PASS |

---

## Patterns & Conventions

| Pattern | Implementation |
|---|---|
| Package scope | `package post` (white-box / same-package) |
| Logger | `zap.NewNop()` (silent) |
| ShardRouter stub | `&ShardRouter{}` — bypasses `NewShardRouter` which needs live DSNs |
| Nil DB tolerance | Validation logic short-circuits before shard/DB access |
| Auth context | `auth.WithClaims(context.Background(), &auth.Claims{UserID, Username})` |
| Helper tagging | `t.Helper()` in `testPostServer` and `authenticatedCtx` |
