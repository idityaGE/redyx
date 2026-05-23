# Community Service — Test Case Specification

| | |
|---|---|
| **Project Name** | Redyx |
| **Module Name** | Community |
| **File** | `internal/community/server_test.go` |
| **Total Tests** | 16 |
| **Package** | `community` (same-package test) |
| **Created By** | idityaGE |
| **Date of Creation** | 2026-05-10 |

---

## Test Infrastructure

| Component | Implementation |
|---|---|
| Logger | `zap.NewNop()` |
| DB Pool | `nil` — tests short-circuit before DB access |
| Cache | `nil` — not required for validation tests |
| Auth Context | `auth.WithClaims(context.Background(), &auth.Claims{…})` |
| Helper convention | `t.Helper()` in `testCommunityServer`, `authedCommCtx` |
| Nil recovery | `defer recover()` for tests that pass validation and reach nil DB |

---

## 1. `nameRegex` Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_COMM_001 | `TestNameRegex_ValidNames` | `nameRegex` compiled | Call `nameRegex.MatchString` for 6 valid names | `"abc"`, `"Go_lang"`, `"community123"`, `"a_b_c_d_e_f"`, `"ALLCAPS"`, `"Mix3dC4se"` | All 6 names match the regex | PASS |
| TC_COMM_002 | `TestNameRegex_InvalidNames` | `nameRegex` compiled | Call `nameRegex.MatchString` for 6 invalid names | `"ab"` (too short), `""` (empty), `"has space"`, `"has-dash"`, `"has.dot"`, `"toolongname123456789012"` (22 chars, over limit) | None of the 6 names match the regex | PASS |

---

## 2. CreateCommunity Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_COMM_003 | `TestCreateCommunity_Unauthenticated` | Community server initialized | Call `CreateCommunity` with unauthenticated context | name = `"validname"` | Returns an error (missing auth claims) | PASS |
| TC_COMM_004 | `TestCreateCommunity_InvalidName_TooShort` | Authenticated context | Call `CreateCommunity` with 2-character name | name = `"ab"` (min is 3) | Returns an error | PASS |
| TC_COMM_005 | `TestCreateCommunity_InvalidName_WithSpace` | Authenticated context | Call `CreateCommunity` with name containing space | name = `"my community"` | Returns an error | PASS |
| TC_COMM_006 | `TestCreateCommunity_InvalidName_WithDash` | Authenticated context | Call `CreateCommunity` with name containing dash | name = `"my-community"` | Returns an error | PASS |
| TC_COMM_007 | `TestCreateCommunity_InvalidName_TooLong` | Authenticated context | Call `CreateCommunity` with 23-character name | name = `"toolongname123456789012"` (23 chars, max is 21) | Returns an error | PASS |
| TC_COMM_008 | `TestCreateCommunity_ValidName_FailsAtDB` | Authenticated context; nil DB pool | Call `CreateCommunity` with valid name; recover panic | name = `"validname"`, description = `"A test community"` | Validation passes; panics at nil DB (recovered) or returns DB error | PASS |

---

## 3. UpdateCommunity Validation Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_COMM_009 | `TestUpdateCommunity_Unauthenticated` | Community server initialized | Call `UpdateCommunity` with unauthenticated context | name = `"validname"`, description = `"New description"` | Returns an error | PASS |

---

## 4. Auth Guard Tests (JoinCommunity / LeaveCommunity / AssignModerator / RevokeModerator)

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_COMM_010 | `TestJoinCommunity_Unauthenticated` | Community server initialized | Call `JoinCommunity` with unauthenticated context | name = `"testcommunity"` | Returns an error | PASS |
| TC_COMM_011 | `TestLeaveCommunity_Unauthenticated` | Community server initialized | Call `LeaveCommunity` with unauthenticated context | name = `"testcommunity"` | Returns an error | PASS |
| TC_COMM_012 | `TestAssignModerator_Unauthenticated` | Community server initialized | Call `AssignModerator` with unauthenticated context | name = `"testcommunity"`, userId = `"user-2"`, username = `"otheruser"` | Returns an error | PASS |
| TC_COMM_013 | `TestRevokeModerator_Unauthenticated` | Community server initialized | Call `RevokeModerator` with unauthenticated context | name = `"testcommunity"`, userId = `"user-2"` | Returns an error | PASS |

---

## 5. `communityRulesToSlice` Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_COMM_014 | `TestCommunityRulesToSlice_ConvertsCorrectly` | — | Call `communityRulesToSlice` with 2 rules | rules = `[{title:"Be Kind", desc:"Treat others well"}, {title:"No Spam", desc:"No promotional content"}]` | Returns slice of 2 maps; `result[0]["title"]="Be Kind"`, `result[1]["title"]="No Spam"` | PASS |
| TC_COMM_015 | `TestCommunityRulesToSlice_Empty` | — | Call `communityRulesToSlice` with empty slice | rules = `[]` | Returns empty slice (length 0) | PASS |
| TC_COMM_016 | `TestCommunityRulesToSlice_Nil` | — | Call `communityRulesToSlice` with nil | rules = `nil` | Returns empty slice (length 0) | PASS |

---

## 6. `buildCommunityProto` Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_COMM_017 | `TestBuildCommunityProto_NoRules` | — | Call `buildCommunityProto` with nil rules | communityId = `"comm-id-1"`, name = `"testcomm"`, memberCount = 42, visibility = `int16(1)` | No error; `CommunityId="comm-id-1"`, `Name="testcomm"`, `MemberCount=42`, `Rules` is empty | PASS |
| TC_COMM_018 | `TestBuildCommunityProto_WithRules` | Valid JSON rules | Call `buildCommunityProto` with valid JSON rules | rulesJSON = `[{"title":"Rule 1","description":"Desc 1"},{"title":"Rule 2","description":"Desc 2"}]` | No error; `len(Rules)=2`, `Rules[0].Title="Rule 1"` | PASS |
| TC_COMM_019 | `TestBuildCommunityProto_InvalidRulesJSON` | — | Call `buildCommunityProto` with invalid JSON bytes | rulesJSON = `"not valid json"` | Returns an error (JSON unmarshal fails) | PASS |

---

## Patterns & Conventions

| Pattern | Implementation |
|---|---|
| Package scope | `package community` (white-box / same-package) |
| Logger | `zap.NewNop()` (silent) |
| Nil DB/cache | `NewServer(nil, nil, zap.NewNop())` — validation and helper tests only |
| Panic recovery | `defer recover()` guards for tests that reach nil DB |
| Auth context | `auth.WithClaims(context.Background(), &auth.Claims{UserID, Username})` |
| Helper tagging | `t.Helper()` in `testCommunityServer` and `authedCommCtx` |
| Name rules | Regex enforces 3–21 chars, alphanumeric + underscore only |
