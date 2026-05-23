# User Service — Test Case Specification

| | |
|---|---|
| **Project Name** | Redyx |
| **Module Name** | User |
| **File** | `internal/user/server_test.go` |
| **Total Tests** | 11 |
| **Package** | `user` (same-package test) |
| **Created By** | idityaGE |
| **Date of Creation** | 2026-05-10 |

---

## Test Infrastructure

| Component | Implementation |
|---|---|
| Logger | `zap.NewNop()` |
| DB Pool | `nil` — tests short-circuit before DB access |
| Post / Comment / Community Clients | `nil` — graceful nil-client behaviour tested explicitly |
| Auth Context | `auth.WithClaims(context.Background(), &auth.Claims{…})` |
| Helper convention | `t.Helper()` in `testUserServer`, `authedUserCtx` |

---

## 1. `profileToProto` Conversion Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_USER_001 | `TestProfileToProto_ActiveUser` | — | Construct active `profile` struct; call `profileToProto` | userId = `"user-abc"`, username = `"alice"`, displayName = `"Alice Wonderland"`, bio = `"Curiouser and curiouser"`, karma = 1234 | All fields mapped correctly: `UserId`, `Username`, `DisplayName`, `Bio`, `Karma` all match source | PASS |
| TC_USER_002 | `TestProfileToProto_DeletedUser_Sanitized` | Profile with `DeletedAt.Valid=true` | Set `DeletedAt.Valid = true`; call `profileToProto` | username = `"deleted_user"`, displayName = `"Secret Name"`, bio = `"Private bio"`, karma = 9999 | `Username="[deleted]"`, `DisplayName=""`, `Bio=""`, `Karma=0` (all PII stripped for soft-deleted accounts) | PASS |

---

## 2. GetProfile Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_USER_003 | `TestGetProfile_EmptyUsername` | User server initialized | Call `GetProfile` with empty username | username = `""` | Returns an error | PASS |

---

## 3. UpdateProfile Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_USER_004 | `TestUpdateProfile_Unauthenticated` | User server initialized | Call `UpdateProfile` with unauthenticated context | displayName = `"New Name"` | Returns an error (missing auth claims) | PASS |
| TC_USER_005 | `TestUpdateProfile_BioTooLong` | Authenticated context | Call `UpdateProfile` with 501-character bio | bio = 501×`"a"` | Returns an error (max 500 chars) | PASS |
| TC_USER_006 | `TestUpdateProfile_DisplayNameTooLong` | Authenticated context | Call `UpdateProfile` with 51-character display name | displayName = 51×`"a"` | Returns an error (max 50 chars) | PASS |

---

## 4. DeleteAccount Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_USER_007 | `TestDeleteAccount_Unauthenticated` | User server initialized | Call `DeleteAccount` with unauthenticated context | — | Returns an error | PASS |

---

## 5. GetUserPosts Tests (Nil-Client Graceful Behaviour)

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_USER_008 | `TestGetUserPosts_EmptyUsername` | User server initialized | Call `GetUserPosts` with empty username | username = `""` | Returns an error | PASS |
| TC_USER_009 | `TestGetUserPosts_NoPostClient_ReturnsEmpty` | User server with nil post client | Call `GetUserPosts` with valid username | username = `"someuser"` | Returns success with empty posts list (graceful nil-client fallback) | PASS |

---

## 6. GetUserComments Tests (Nil-Client Graceful Behaviour)

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_USER_010 | `TestGetUserComments_EmptyUsername` | User server initialized | Call `GetUserComments` with empty username | username = `""` | Returns an error | PASS |
| TC_USER_011 | `TestGetUserComments_NoCommentClient_ReturnsEmpty` | User server with nil comment client | Call `GetUserComments` with valid username | username = `"someuser"` | Returns success with empty comments list (graceful nil-client fallback) | PASS |

---

## 7. GetUserCommunities Tests (Nil-Client Graceful Behaviour)

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_USER_012 | `TestGetUserCommunities_EmptyUserID` | User server initialized | Call `GetUserCommunities` with empty user_id | userId = `""` | Returns an error | PASS |
| TC_USER_013 | `TestGetUserCommunities_NoCommunityClient_ReturnsEmpty` | User server with nil community client | Call `GetUserCommunities` with valid user_id and pagination | userId = `"user-123"`, limit = 10 | Returns success with empty communities list (graceful nil-client fallback) | PASS |

---

## Patterns & Conventions

| Pattern | Implementation |
|---|---|
| Package scope | `package user` (white-box / same-package) |
| Logger | `zap.NewNop()` (silent) |
| Nil DB | `NewServer(nil, zap.NewNop())` — validation-only tests |
| Nil client graceful fallback | `GetUserPosts / GetUserComments / GetUserCommunities` return empty response when downstream client is nil (no panic) |
| Soft-delete sanitization | `profileToProto` clears PII fields when `DeletedAt.Valid=true` |
| Auth context | `auth.WithClaims(context.Background(), &auth.Claims{UserID, Username})` |
| Helper tagging | `t.Helper()` in `testUserServer` and `authedUserCtx` |
