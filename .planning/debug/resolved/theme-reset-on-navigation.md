---
status: resolved
trigger: "Theme resets to dark mode on every page navigation, even when light mode was selected"
created: 2026-03-05T00:00:00Z
updated: 2026-03-05T00:00:00Z
---

## Current Focus

hypothesis: CONFIRMED - Two-part root cause: (1) `<html class="dark">` hardcoded in template, and (2) Astro ClientRouter swaps the new page's `<html>` tag into the DOM on navigation, which carries the hardcoded `class="dark"`, overriding any runtime class changes. The inline script does check localStorage correctly, BUT after Astro swaps in the new document, the `<html>` element gets replaced with the hardcoded `class="dark"` from the template before the script runs on the new page.
test: Trace the Astro ClientRouter page swap lifecycle
expecting: `<html class="dark">` from new page replaces current `<html>` class state
next_action: Apply fix using astro:after-swap event to reapply theme after navigation swap

## Symptoms

expected: Theme preference (light/dark) persists across page navigations. If user selects light mode, it stays light on all pages.
actual: Theme resets to dark mode on every page navigation, even when light mode was selected.
errors: No error messages reported.
reproduction: 1) Set theme to light mode via ThemeToggle component. 2) Click any link to navigate to another page. 3) Theme reverts to dark mode.
started: Likely since Astro ClientRouter was added for SPA navigation (Phase 3, plan 07)

## Eliminated

## Evidence

- timestamp: 2026-03-05T00:01:00Z
  checked: BaseLayout.astro line 14
  found: `<html lang="en" class="dark">` — hardcoded dark class in template
  implication: Every new page document has class="dark" regardless of user preference

- timestamp: 2026-03-05T00:02:00Z
  checked: BaseLayout.astro lines 23-31 (inline theme script)
  found: The is:inline script DOES check localStorage correctly, but Astro's swap-functions.js `scriptsAlreadyRan` Set prevents it from re-executing on subsequent navigations because the script textContent is identical
  implication: Script only runs on initial page load, not on SPA navigations

- timestamp: 2026-03-05T00:03:00Z
  checked: astro/dist/transitions/swap-functions.js lines 21-29 (swapRootAttributes)
  found: During navigation, ALL attributes are removed from current `<html>` and replaced with attributes from new document's `<html>` — which always has `class="dark"` from template
  implication: swapRootAttributes resets theme to dark on every navigation

- timestamp: 2026-03-05T00:04:00Z
  checked: astro/dist/transitions/swap-functions.js lines 3-19 (deselectScripts + scriptsAlreadyRan)
  found: `scriptsAlreadyRan` is a Set keyed by script textContent. Once the theme script runs on first page load, it's marked as already-ran. On navigation, `detectScriptExecuted` returns true, script gets `data-astro-exec=""` attribute, and does NOT re-execute.
  implication: The is:inline theme script never re-runs to correct the class="dark" override

## Resolution

root_cause: Two-part mechanism: (1) `swapRootAttributes` in Astro ClientRouter replaces all `<html>` attributes with new document's attributes on each navigation — new document always has hardcoded `class="dark"` from BaseLayout.astro template. (2) The `is:inline` theme script that checks localStorage is deduplicated by Astro's `scriptsAlreadyRan` Set (keyed by textContent) so it only runs on first page load, never re-executing to correct the overridden class.
fix: Add `data-astro-rerun` attribute to the inline theme script so it re-executes on every navigation, correctly applying the user's stored preference after swapRootAttributes resets the class.
verification: |
  - Build succeeds with no errors
  - `data-astro-rerun` present in built output (dist/server/chunks/BaseLayout_D6B_iKyF.mjs)
  - Code path verified: deselectScripts() in swap-functions.js skips dedup when `data-astro-rerun` is present (line 14)
  - Sequence verified: swapRootAttributes sets class="dark" → script re-executes → checks localStorage → corrects to light if needed
  - Human verified: confirmed fixed — theme persists across navigations
files_changed:
  - web/src/layouts/BaseLayout.astro

