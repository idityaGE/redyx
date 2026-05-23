# Spam Service — Test Case Specification

| | |
|---|---|
| **Project Name** | Redyx |
| **Module Name** | Spam |
| **File** | `internal/spam/spam_test.go` |
| **Total Tests** | 22 |
| **Package** | `spam` (same-package test) |
| **Created By** | idityaGE |
| **Date of Creation** | 2026-05-10 |

---

## Test Infrastructure

| Component | Implementation |
|---|---|
| Logger | `zap.NewNop()` |
| Redis | `miniredis.RunT(t)` (in-process, via `testRedis` helper) |
| Blocklist File | `data/blocklist.json` loaded via `LoadBlocklist` |
| Server Helper | `testServer(t)` — composes `testBlocklist + testRedis + NewDedupChecker + NewServer` |
| Helper convention | `t.Helper()` in all setup helpers |
| Skip guard | `t.Skipf` if `data/blocklist.json` not found on disk |

---

## 1. Blocklist — `LoadBlocklist` Test

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_SPAM_001 | `TestLoadBlocklist` | `data/blocklist.json` exists on disk | Call `LoadBlocklist("data/blocklist.json")` | path = `"data/blocklist.json"` | Returns non-nil `Blocklist`; `keywords` list non-empty; `domains` list non-empty | PASS |

---

## 2. Blocklist — `CheckKeywords` Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_SPAM_002 | `TestCheckKeywords_Clean` | Blocklist loaded | Call `bl.CheckKeywords("This is a normal post about programming")` | content = clean text | `matched=false` | PASS |
| TC_SPAM_003 | `TestCheckKeywords_Match` | Blocklist loaded | Call `bl.CheckKeywords("Hey everyone, BUY NOW before it's too late!")` | content contains `"buy now"` keyword | `matched=true` | PASS |
| TC_SPAM_004 | `TestCheckKeywords_CaseInsensitive` | Blocklist loaded | Call `bl.CheckKeywords("CLICK HERE for amazing deals")` | content contains `"click here"` in uppercase | `matched=true` (case-insensitive matching) | PASS |

---

## 3. Blocklist — `CheckURLs` Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_SPAM_005 | `TestCheckURLs_Clean` | Blocklist loaded | Call `bl.CheckURLs(["https://golang.org", "https://github.com"])` | urls = legitimate domains | `matched=false` | PASS |
| TC_SPAM_006 | `TestCheckURLs_BlockedDomain` | Blocklist loaded | Call `bl.CheckURLs(["https://malware-site.com/payload"])` | urls = known blocked domain | `matched=true` | PASS |
| TC_SPAM_007 | `TestCheckURLs_BitLy` | Blocklist loaded | Call `bl.CheckURLs(["https://bit.ly/abc123"])` | urls = `bit.ly` shortened URL | `matched=true` (URL shortener is blocklisted) | PASS |

---

## 4. Blocklist — `ExtractURLs` Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_SPAM_008 | `TestExtractURLs_BareURLs` | — | Call `ExtractURLs("Check out https://example.com and http://test.org/page")` | content with 2 bare URLs | Returns slice of length 2 | PASS |
| TC_SPAM_009 | `TestExtractURLs_MarkdownLinks` | — | Call `ExtractURLs("Visit [my site](https://example.com) for more")` | content with 1 markdown link URL | Returns slice of length ≥ 1 | PASS |
| TC_SPAM_010 | `TestExtractURLs_NoURLs` | — | Call `ExtractURLs("This is a plain text post with no links")` | content with no URLs | Returns slice of length 0 | PASS |

---

## 5. Dedup Checker — `DedupChecker.Check` Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_SPAM_011 | `TestCheckDuplicate_FirstTime` | miniredis running; fresh dedup store | Call `dc.Check(ctx, "user1", "Hello world")` | userId = `"user1"`, content = `"Hello world"` | `isDup=false`, `hash` is non-empty string | PASS |
| TC_SPAM_012 | `TestCheckDuplicate_SecondTime` | miniredis running; same content already submitted by user1 | Submit same content twice for same user | userId = `"user1"`, content = `"Hello world"` (twice) | 1st call: `isDup=false`; 2nd call: `isDup=true` | PASS |
| TC_SPAM_013 | `TestCheckDuplicate_DifferentUser` | miniredis running; user1 already submitted content | Submit same content for user2 | userId = `"user2"`, content = `"Hello world"` | `isDup=false` (dedup is per-user scoped) | PASS |
| TC_SPAM_014 | `TestCheckDuplicate_NormalizesContent` | miniredis running; fresh dedup store | Submit `"  Hello   World  "` then `"hello world"` for same user | user1 with whitespace-variant content | `isDup=true`; both calls return same hash (content normalized before hashing) | PASS |

---

## 6. Server — `CheckContent` Tests

| Test Case ID | Test Name | Pre-Condition | Test Steps | Test Data | Expected Result | Status |
|:---------|:------------------|:-------------|:--------------------|:----------------|:------------------|:------|
| TC_SPAM_015 | `TestCheckContent_Clean` | Server initialized with blocklist + dedup | Call `srv.CheckContent` with clean content | userId = `"user1"`, contentType = `"post_body"`, content = `"This is a normal discussion about Go programming"` | `result=SPAM_CHECK_RESULT_CLEAN`, `reasons=[]` (empty) | PASS |
| TC_SPAM_016 | `TestCheckContent_BlockedKeyword` | Server initialized | Call `srv.CheckContent` with content containing blocked keyword | content = `"Hey! Buy now before it's too late!"` | `result=SPAM_CHECK_RESULT_SPAM`, `len(reasons)>0` | PASS |
| TC_SPAM_017 | `TestCheckContent_BlockedURL` | Server initialized | Call `srv.CheckContent` with blocked URL in `urls` field | urls = `["https://malware-site.com/payload"]` | `result=SPAM_CHECK_RESULT_SPAM` | PASS |
| TC_SPAM_018 | `TestCheckContent_BlockedURL_InContent` | Server initialized | Call `srv.CheckContent` with blocked URL embedded in content text | content = `"Visit https://bit.ly/scam for free stuff"` | `result=SPAM_CHECK_RESULT_SPAM` (URL extracted from content text) | PASS |
| TC_SPAM_019 | `TestCheckContent_Duplicate` | Server initialized | Submit same request twice | userId = `"user1"`, content = `"Some unique post content here"` | 1st resp: `isDuplicate=false`, `contentHash≠""`; 2nd resp: `isDuplicate=true` | PASS |
| TC_SPAM_020 | `TestCheckContent_VagueReasons` | Server initialized | Submit content with multiple spam keywords; inspect reasons | content = `"Buy now and get free money!"` | `reasons` must NOT contain exact keyword strings like `"buy now"` or `"free money"` (reasons are deliberately vague for privacy) | PASS |

---

## Patterns & Conventions

| Pattern | Implementation |
|---|---|
| Package scope | `package spam` (white-box / same-package) |
| Logger | `zap.NewNop()` (silent) |
| In-process Redis | `miniredis.RunT(t)` — auto-cleaned on `t.Cleanup` |
| Blocklist skip guard | `t.Skipf` when `data/blocklist.json` not present — safe in CI without data file |
| Content normalization | Dedup normalizes whitespace + lowercases before hashing (SHA-based) |
| Reason vagueness | Spam reasons returned to clients are deliberately obfuscated (no exact keyword exposure) |
| Helper tagging | `t.Helper()` in `testBlocklist`, `testRedis`, `testServer` |
| Reference pattern | This file is the **canonical pattern** for all other service test files in the codebase |
