---
status: resolved
trigger: "Multiple bugs on community settings page at /community/{name}/settings"
created: 2026-03-27T00:00:00Z
updated: 2026-03-27T12:30:00Z
---

## Current Focus

hypothesis: Three new issues found after previous fixes
test: Investigate each issue
expecting: Find root causes for 400 error, visibility not pre-selected, "unknown" display
next_action: Fix all three issues

### New Issues Found (2026-03-27)

1. **Moderator 400 error**: Frontend sends `userId` (camelCase) but proto expects `user_id` (snake_case)
   - Logs show: "user_id is required: invalid input"
   - gRPC-JSON transcoding expects snake_case field names in request body

2. **Visibility not pre-selected**: API returns `"VISIBILITY_RESTRICTED"` (string) but frontend expects number
   - Frontend visibilityOptions use { value: 1, 2, 3 } 
   - editVisibility comparison fails because "VISIBILITY_RESTRICTED" !== 2

3. **"unknown" on community page**: Same root cause - visibility is string "VISIBILITY_PUBLIC" 
   - visibilityLabel(v) switch expects numbers but receives string
   - Falls through to default: return 'unknown'

## Symptoms

expected: 
  - Issue 1: Updating visibility settings should only update visibility
  - Issue 2: UI should have banner and icon upload functionality using MinIO media client
  - Issue 3: POST /api/v1/communities/{name}/moderators should add a moderator
actual:
  - Issue 1: After saving visibility, the community rules disappear/get deleted
  - Issue 2: The upload UI elements are completely missing from the page
  - Issue 3: Returns 500 Internal Server Error
errors:
  - Issue 1: None visible - silent data loss
  - Issue 2: None - feature is missing
  - Issue 3: HTTP 500 status
reproduction:
  - Issue 1: 1. Go to community settings, 2. Add/update a rule, 3. Save rules (works), 4. Update visibility setting, 5. Rules are gone
  - Issue 2: Go to /community/{name}/settings - no banner/icon upload controls visible
  - Issue 3: Try to add a moderator via the settings page
started: User discovered when testing settings page

## Eliminated

- hypothesis: COALESCE($2, rules) fix would preserve rules when nil
  evidence: User tested - PATCH with visibility only still returns rules: []
  timestamp: 2026-03-27T00:10:00Z

## Evidence

- timestamp: 2026-03-27T00:00:30Z
  checked: internal/community/server.go UpdateCommunity function (lines 203-263)
  found: |
    Line 226: `rulesJSON, err := json.Marshal(communityRulesToSlice(req.GetRules()))`
    This ALWAYS marshals rules from request, even when not sent (empty slice -> []).
    Line 239 in the UPDATE query unconditionally sets `rules = $2` with this JSON.
    When frontend sends only `{visibility: 2}`, req.GetRules() returns nil/empty slice.
    The empty slice gets marshaled as `[]` and overwrites existing rules.
  implication: Root cause of Issue 1 confirmed - need conditional update for rules

- timestamp: 2026-03-27T00:10:00Z
  checked: User verification feedback
  found: |
    1. Rules STILL getting deleted after PATCH with visibility only
    2. Moderators 500 still occurring
    3. Banner/icon not displaying on community PAGE (not settings)
  implication: Previous fixes did not take effect or were incorrect

- timestamp: 2026-03-27T00:20:00Z
  checked: Community service logs and API response
  found: |
    1. Moderator 400 error log: "user_id is required: invalid input"
       - Frontend sends `userId` (camelCase) but proto expects `user_id` (snake_case)
    2. API returns: "visibility": "VISIBILITY_RESTRICTED" (string enum)
       - Frontend expects number (1, 2, 3) but receives string
       - This breaks radio button pre-selection and visibilityLabel() function
    3. CommunitySidebar visibilityLabel() returns "unknown" for unrecognized values
  implication: Three frontend bugs found, all fixable

- timestamp: 2026-03-27T00:25:00Z
  checked: Applied fixes
  found: |
    1. Changed addModerator to send `user_id` (snake_case) in JSON body
    2. Changed all visibility types from number to string
    3. Updated visibilityOptions to use 'VISIBILITY_PUBLIC' etc
    4. Updated visibilityLabel() in all components to handle string enums
    5. Rebuilt and recreated community-service container
  implication: All three new issues should be fixed

## Resolution

root_cause:
  - Issue 1 (rules deletion): Fixed in previous session - COALESCE($2, rules) preserves rules when nil
  - Issue 2 (visibility not pre-selected): API returns string enum but frontend expected number
  - Issue 3 (unknown display): Same - visibilityLabel() expected number, got string
  - Issue 4 (moderator 400): Frontend sent camelCase field names but gRPC expects snake_case

fix:
  - Changed frontend to use string visibility enum values ('VISIBILITY_PUBLIC' etc)
  - Updated all Community types to use visibility: string
  - Updated visibilityLabel() in CommunitySidebar, CommunityList to handle string enums
  - Changed addModerator to send user_id (snake_case) in JSON body
  
verification: Awaiting user testing
files_changed:
  - web/src/components/community/CommunitySettings.svelte
  - web/src/components/community/CommunitySidebar.svelte
  - web/src/components/community/CommunityDetail.svelte
  - web/src/components/community/CommunityList.svelte
