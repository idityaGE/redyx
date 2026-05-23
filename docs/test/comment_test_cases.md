# Comment Service — Test Case Specification

| | |
|---|---|
| **Project Name** | Redyx |
| **Module Name** | Comment |
| **File** | `internal/comment/server_test.go` |
| **Total Tests** | 18 |
| **Package** | `comment` (same-package test) |
| **Created By** | idityaGE |
| **Date of Creation** | 2026-05-10 |

---

## Test Infrastructure

| Component | Implementation |
|---|---|
| Logger | `zap.NewNop()` |
| Store | `nil` — tests short-circuit before store access |
| External Clients | `nil` — not required for validation tests |
| Auth Context | `auth.WithClaims(context.Background(), &auth.Claims{…})` |
| Helper convention | `t.Helper()` in `testCommentServer`, `authedCtx` |
| Nil recovery | `defer recover()` for tests that validate pass-through to nil store |

---

## 1. `mapSortOrder` Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_CMNT_001 | `TestMapSortOrder_AllCases` | — | Call `mapSortOrder` for each proto enum value | `BEST`, `TOP`, `NEW`, `CONTROVERSIAL`, `UNSPECIFIED` (5 sub-cases) | `BEST→SortBest`, `TOP→SortTop`, `NEW→SortNew`, `CONTROVERSIAL→SortControversial`, `UNSPECIFIED→SortBest` (default fallback) | PASS |

---

## 2. CreateComment Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_CMNT_002 | `TestCreateComment_Unauthenticated` | Comment server initialized | Call `CreateComment` with unauthenticated context | postId = `"post-1"`, body = `"A valid comment body"` | Returns an error (missing auth claims) | PASS |
| TC_CMNT_003 | `TestCreateComment_EmptyPostID` | Authenticated context | Call `CreateComment` with empty post_id | postId = `""`, body = `"A valid comment body"` | Returns an error | PASS |
| TC_CMNT_004 | `TestCreateComment_EmptyBody` | Authenticated context | Call `CreateComment` with empty body | postId = `"post-1"`, body = `""` | Returns an error | PASS |
| TC_CMNT_005 | `TestCreateComment_BodyTooLong` | Authenticated context | Call `CreateComment` with 10 001-character body | body = 10 001×`"a"` | Returns an error (max 10 000 chars) | PASS |

---

## 3. GetComment Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_CMNT_006 | `TestGetComment_EmptyCommentID` | Comment server initialized | Call `GetComment` with empty comment_id | commentId = `""` | Returns an error | PASS |

---

## 4. UpdateComment Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_CMNT_007 | `TestUpdateComment_Unauthenticated` | Comment server initialized | Call `UpdateComment` with unauthenticated context | commentId = `"comment-1"`, body = `"Updated body"` | Returns an error | PASS |
| TC_CMNT_008 | `TestUpdateComment_EmptyCommentID` | Authenticated context | Call `UpdateComment` with empty comment_id | commentId = `""`, body = `"Updated body"` | Returns an error | PASS |
| TC_CMNT_009 | `TestUpdateComment_EmptyBody` | Authenticated context | Call `UpdateComment` with empty body | commentId = `"comment-1"`, body = `""` | Returns an error | PASS |
| TC_CMNT_010 | `TestUpdateComment_BodyTooLong` | Authenticated context | Call `UpdateComment` with 10 001-character body | body = 10 001×`"x"` | Returns an error (max 10 000 chars) | PASS |

---

## 5. DeleteComment Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_CMNT_011 | `TestDeleteComment_Unauthenticated` | Comment server initialized | Call `DeleteComment` with unauthenticated context | commentId = `"comment-1"` | Returns an error | PASS |
| TC_CMNT_012 | `TestDeleteComment_EmptyCommentID` | Authenticated context | Call `DeleteComment` with empty comment_id | commentId = `""` | Returns an error | PASS |

---

## 6. ListComments Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_CMNT_013 | `TestListComments_EmptyPostID` | Comment server initialized | Call `ListComments` with empty post_id | postId = `""` | Returns an error | PASS |
| TC_CMNT_014 | `TestListComments_ValidRequest_FailsAtStore` | Nil store | Call `ListComments` with valid post_id and limit > 50 | postId = `"some-post-id"`, limit = 200 | Validation passes (limit clamped to 50); panics at nil store (recovered) or returns store error | PASS |

---

## 7. ListReplies Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_CMNT_015 | `TestListReplies_EmptyCommentID` | Comment server initialized | Call `ListReplies` with empty comment_id | commentId = `""` | Returns an error | PASS |

---

## 8. ListCommentsByAuthor Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_CMNT_016 | `TestListCommentsByAuthor_EmptyUsername` | Comment server initialized | Call `ListCommentsByAuthor` with empty username | username = `""` | Returns an error | PASS |

---

## 9. `commentToProto` Conversion Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_CMNT_017 | `TestCommentToProto_NonZeroParentID_SetInProto` | Valid UUIDs for all IDs | Construct `Comment` with non-zero `ParentID`; call `commentToProto` | parentId = `"cccc…"`, commentId = `"aaaa…"`, body = `"Hello world"`, voteScore = 5, authorUsername = `"alice"` | `proto.ParentId` is non-empty; `CommentId`, `Body`, `VoteScore`, `AuthorUsername` all match source | PASS |
| TC_CMNT_018 | `TestCommentToProto_ZeroParentID_NotSetInProto` | Valid comment/post/author UUIDs; ParentID = zero UUID | Construct `Comment` without ParentID; call `commentToProto` | parentId = zero UUID (top-level comment) | `proto.ParentId` is empty string (top-level has no parent) | PASS |

---

## Patterns & Conventions

| Pattern | Implementation |
|---|---|
| Package scope | `package comment` (white-box / same-package) |
| Logger | `zap.NewNop()` (silent) |
| Nil store | `NewServer(nil, nil, nil, zap.NewNop())` — validation-only |
| Panic recovery | `defer recover()` guards in tests that let execution reach nil store |
| Auth context | `auth.WithClaims(context.Background(), &auth.Claims{UserID, Username})` |
| Helper tagging | `t.Helper()` in `testCommentServer` and `authedCtx` |
