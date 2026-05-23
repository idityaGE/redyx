# Redyx — Test Case Index

| | |
|---|---|
| **Project Name** | Redyx |
| **Document Type** | Test Case Specification Index |
| **Author** | idityaGE |
| **Date** | 2026-05-10 |
| **Total Tests** | 134 across 7 services |

---

## Service Test Summary

| # | Service | File | Tests | Document |
|---|---|---|---|---|
| 1 | Auth | `internal/auth/server_test.go` | 23 | [auth_test_cases.md](./auth_test_cases.md) |
| 2 | Post | `internal/post/server_test.go` | 16 | [post_test_cases.md](./post_test_cases.md) |
| 3 | Comment | `internal/comment/server_test.go` | 18 | [comment_test_cases.md](./comment_test_cases.md) |
| 4 | Community | `internal/community/server_test.go` | 16 | [community_test_cases.md](./community_test_cases.md) |
| 5 | User | `internal/user/server_test.go` | 11 | [user_test_cases.md](./user_test_cases.md) |
| 6 | Vote | `internal/vote/server_test.go` | 30 | [vote_test_cases.md](./vote_test_cases.md) |
| 7 | Spam *(reference)* | `internal/spam/spam_test.go` | 22 | [spam_test_cases.md](./spam_test_cases.md) |

---

## Test Focus by Service

| Service | Primary Focus Areas |
|---|---|
| **Auth** | Argon2id hash/verify, JWT generation & expiry, OTP generate/verify/single-use, `hashRefreshToken`, server input validation (email regex, username regex, password length, empty fields) |
| **Post** | `HotScore`/`RisingScore` ranking algorithms, `CreatePost`/`UpdatePost`/`DeletePost`/`ListPosts` validation (auth, title length, URL format, media IDs, body length, community name) |
| **Comment** | `mapSortOrder` all enum cases, CRUD validation (auth, empty IDs, body length limits), `commentToProto` (zero vs non-zero parent UUID), `ListCommentsByAuthor` username validation |
| **Community** | `nameRegex` valid/invalid names, auth guards on all RPCs, `communityRulesToSlice` conversion, `buildCommunityProto` (no rules, with rules, invalid JSON) |
| **User** | `profileToProto` (active user & soft-deleted sanitization), `GetProfile`/`UpdateProfile`/`DeleteAccount` validation, graceful nil-client behaviour for `GetUserPosts`/`GetUserComments`/`GetUserCommunities` |
| **Vote** | `VoteStore` Lua script: all 9 vote transitions (up/down/none, idempotency, flips), `BatchGetVoteStates`, `GetScore`; server auth guards; `GetVoteState` direction mapping; `toInt` helper |
| **Spam** | `LoadBlocklist`, `CheckKeywords`/`CheckURLs` (clean, matched, case-insensitive), `ExtractURLs` (bare & markdown URLs), `DedupChecker` (first/second/cross-user/normalization), `CheckContent` end-to-end (clean, keyword, URL, duplicate, vague reasons) |

---

## Shared Testing Patterns

All test files follow the same conventions established in `spam_test.go`:

| Pattern | Value |
|---|---|
| Test scope | Same-package (`package <service>`) — white-box access |
| Logger | `zap.NewNop()` — silent, no output noise |
| Redis | `miniredis.RunT(t)` — in-process, no external dependency |
| Helper tagging | `t.Helper()` in every setup helper |
| Nil external clients | Server constructed with `nil` DB/clients for validation-only tests |
| Auth context | `auth.WithClaims(context.Background(), &auth.Claims{…})` |
| Panic recovery | `defer recover()` where tests pass validation and reach nil downstream dependency |

---

## Running Tests

```bash
# Run all tests across all services
go test ./internal/...

# Run tests for a specific service
go test ./internal/auth/...
go test ./internal/post/...
go test ./internal/comment/...
go test ./internal/community/...
go test ./internal/user/...
go test ./internal/vote/...
go test ./internal/spam/...

# Run with verbose output
go test -v ./internal/...

# Run with race detector
go test -race ./internal/...
```
