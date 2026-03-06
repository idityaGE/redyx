<script lang="ts">
  import { api, ApiError } from '../../lib/api';
  import { relativeTime } from '../../lib/time';
  import BanDialog from './BanDialog.svelte';

  interface Props {
    communityName: string;
  }

  let { communityName }: Props = $props();

  type Report = {
    contentId: string;
    contentType: number; // 1=post, 2=comment
    contentTitle: string;
    contentPreview: string;
    contentAuthorId: string;
    contentAuthorUsername: string;
    reportCount: number;
    topReason: string;
    latestReportAt: string;
    source: string;
    status: string;
    resolvedAction: string;
  };

  type ReportResponse = {
    reports: Report[];
    nextCursor: string;
  };

  let activeTab = $state<'active' | 'resolved'>('active');
  let sourceFilter = $state<'all' | 'user-report' | 'spam-detection'>('all');
  let reports = $state<Report[]>([]);
  let loading = $state(true);
  let nextCursor = $state('');
  let loadingMore = $state(false);

  // Inline confirmation state
  let confirmingRemove = $state<string | null>(null);
  let confirmingDismiss = $state<string | null>(null);
  let confirmingUndo = $state<string | null>(null);

  // Ban dialog state
  let banDialogTarget = $state<{ userId: string; username: string } | null>(null);

  // Action in-progress tracking
  let actionInProgress = $state<string | null>(null);

  async function fetchReports(append = false) {
    if (!append) {
      loading = true;
      nextCursor = '';
    } else {
      loadingMore = true;
    }

    try {
      const params = new URLSearchParams();
      params.set('status', activeTab);
      if (sourceFilter !== 'all') params.set('source', sourceFilter);
      if (append && nextCursor) params.set('cursor', nextCursor);

      const data = await api<ReportResponse>(
        `/communities/${encodeURIComponent(communityName)}/moderation/reports?${params.toString()}`
      );
      if (append) {
        reports = [...reports, ...(data.reports ?? [])];
      } else {
        reports = data.reports ?? [];
      }
      nextCursor = data.nextCursor ?? '';
    } catch {
      if (!append) reports = [];
    } finally {
      loading = false;
      loadingMore = false;
    }
  }

  // Fetch on mount and when tab/filter changes
  $effect(() => {
    // Reference reactive vars so $effect tracks them
    const _tab = activeTab;
    const _source = sourceFilter;
    fetchReports();
  });

  function contentTypeLabel(ct: number): string {
    return ct === 1 ? 'post' : 'comment';
  }

  function truncate(text: string, max: number): string {
    if (!text) return '';
    return text.length > max ? text.slice(0, max) + '...' : text;
  }

  // ── Active tab actions ──────────────────────────

  async function removeContent(report: Report) {
    actionInProgress = report.contentId;
    try {
      await api(`/communities/${encodeURIComponent(communityName)}/moderation/remove`, {
        method: 'POST',
        body: JSON.stringify({
          contentId: report.contentId,
          contentType: report.contentType,
        }),
      });
      confirmingRemove = null;
      await fetchReports();
    } catch {
      // Silently fail, user can retry
    } finally {
      actionInProgress = null;
    }
  }

  async function dismissReport(report: Report) {
    actionInProgress = report.contentId;
    try {
      await api(`/communities/${encodeURIComponent(communityName)}/moderation/reports/dismiss`, {
        method: 'POST',
        body: JSON.stringify({
          contentId: report.contentId,
          contentType: report.contentType,
        }),
      });
      confirmingDismiss = null;
      await fetchReports();
    } catch {
      // Silently fail
    } finally {
      actionInProgress = null;
    }
  }

  function openBanDialog(report: Report) {
    banDialogTarget = {
      userId: report.contentAuthorId,
      username: report.contentAuthorUsername,
    };
  }

  function handleBanned() {
    banDialogTarget = null;
    fetchReports();
  }

  // ── Resolved tab undo actions ───────────────────

  async function undoAction(report: Report) {
    actionInProgress = report.contentId;
    try {
      if (report.resolvedAction === 'removed') {
        await api(`/communities/${encodeURIComponent(communityName)}/moderation/restore`, {
          method: 'POST',
          body: JSON.stringify({
            contentId: report.contentId,
            contentType: report.contentType,
          }),
        });
      } else if (report.resolvedAction === 'banned') {
        await api(`/communities/${encodeURIComponent(communityName)}/moderation/unban`, {
          method: 'POST',
          body: JSON.stringify({
            userId: report.contentAuthorId,
          }),
        });
      } else {
        // Dismissed — re-submit as active (server handles re-opening)
        await api(`/communities/${encodeURIComponent(communityName)}/moderation/restore`, {
          method: 'POST',
          body: JSON.stringify({
            contentId: report.contentId,
            contentType: report.contentType,
          }),
        });
      }
      confirmingUndo = null;
      await fetchReports();
    } catch {
      // Silently fail
    } finally {
      actionInProgress = null;
    }
  }
</script>

<div class="font-mono">
  <!-- Tab bar: active / resolved -->
  <div class="flex gap-2 text-xs mb-3">
    <button
      onclick={() => { activeTab = 'active'; }}
      class="border border-terminal-border px-2 py-0.5 transition-colors {activeTab === 'active' ? 'text-accent-500 border-accent-500' : 'text-terminal-dim hover:text-terminal-fg'}"
    >
      [active]
    </button>
    <button
      onclick={() => { activeTab = 'resolved'; }}
      class="border border-terminal-border px-2 py-0.5 transition-colors {activeTab === 'resolved' ? 'text-accent-500 border-accent-500' : 'text-terminal-dim hover:text-terminal-fg'}"
    >
      [resolved]
    </button>
  </div>

  <!-- Source filter -->
  <div class="flex gap-2 text-xs mb-4">
    <button
      onclick={() => { sourceFilter = 'all'; }}
      class="transition-colors {sourceFilter === 'all' ? 'text-accent-500' : 'text-terminal-dim hover:text-terminal-fg'}"
    >
      [all]
    </button>
    <button
      onclick={() => { sourceFilter = 'user-report'; }}
      class="transition-colors {sourceFilter === 'user-report' ? 'text-accent-500' : 'text-terminal-dim hover:text-terminal-fg'}"
    >
      [user-report]
    </button>
    <button
      onclick={() => { sourceFilter = 'spam-detection'; }}
      class="transition-colors {sourceFilter === 'spam-detection' ? 'text-accent-500' : 'text-terminal-dim hover:text-terminal-fg'}"
    >
      [spam-detection]
    </button>
  </div>

  <!-- Loading -->
  {#if loading}
    <div class="text-xs text-terminal-dim animate-pulse">[loading reports...]</div>
  {:else if reports.length === 0}
    <div class="text-xs text-terminal-dim italic">no {activeTab} reports</div>
  {:else}
    <!-- Report items -->
    <div class="space-y-2">
      {#each reports as report (report.contentId)}
        <div class="border border-terminal-border bg-terminal-surface p-3">
          <!-- Header row -->
          <div class="flex items-center gap-2 text-xs mb-1 flex-wrap">
            <span class="px-1 border {report.source === 'spam-detection' ? 'border-accent-500 text-accent-500' : 'border-terminal-border text-terminal-dim'}">
              [{report.source ?? 'user-report'}]
            </span>
            <span class="text-terminal-dim">[{contentTypeLabel(report.contentType)}]</span>
            <span class="text-terminal-fg">u/{report.contentAuthorUsername}</span>
            <span class="text-terminal-dim">&middot;</span>
            <span class="text-terminal-dim">{report.reportCount} report{report.reportCount !== 1 ? 's' : ''}</span>
            <span class="text-terminal-dim">&middot;</span>
            <span class="text-terminal-dim">{relativeTime(report.latestReportAt)}</span>
          </div>

          <!-- Content preview -->
          <div class="text-xs text-terminal-fg mb-2">
            {truncate(report.contentTitle || report.contentPreview || '[no content]', 80)}
          </div>

          <!-- Reason -->
          <div class="text-xs text-terminal-dim mb-2">
            reason: {report.topReason}
          </div>

          <!-- Actions -->
          {#if activeTab === 'active'}
            <div class="flex items-center gap-2 text-xs">
              {#if confirmingRemove === report.contentId}
                <span class="text-red-400">
                  remove?
                  <button
                    onclick={() => removeContent(report)}
                    disabled={actionInProgress === report.contentId}
                    class="text-red-500 hover:text-red-400 underline cursor-pointer disabled:opacity-50"
                  >
                    [confirm]
                  </button>
                  <button
                    onclick={() => { confirmingRemove = null; }}
                    class="text-terminal-dim hover:text-terminal-fg underline cursor-pointer"
                  >
                    [cancel]
                  </button>
                </span>
              {:else if confirmingDismiss === report.contentId}
                <span class="text-yellow-400">
                  dismiss?
                  <button
                    onclick={() => dismissReport(report)}
                    disabled={actionInProgress === report.contentId}
                    class="text-yellow-500 hover:text-yellow-400 underline cursor-pointer disabled:opacity-50"
                  >
                    [confirm]
                  </button>
                  <button
                    onclick={() => { confirmingDismiss = null; }}
                    class="text-terminal-dim hover:text-terminal-fg underline cursor-pointer"
                  >
                    [cancel]
                  </button>
                </span>
              {:else}
                <button
                  onclick={() => { confirmingRemove = report.contentId; confirmingDismiss = null; }}
                  class="text-red-500 hover:text-red-400 transition-colors border border-terminal-border px-1.5 py-0.5"
                >
                  [remove]
                </button>
                <button
                  onclick={() => { confirmingDismiss = report.contentId; confirmingRemove = null; }}
                  class="text-terminal-dim hover:text-terminal-fg transition-colors border border-terminal-border px-1.5 py-0.5"
                >
                  [dismiss]
                </button>
                <button
                  onclick={() => openBanDialog(report)}
                  class="text-red-500 hover:text-red-400 transition-colors border border-terminal-border px-1.5 py-0.5"
                >
                  [ban user]
                </button>
              {/if}
            </div>
          {:else}
            <!-- Resolved tab -->
            <div class="flex items-center gap-2 text-xs">
              <span class="text-terminal-dim">action: {report.resolvedAction ?? 'resolved'}</span>
              {#if confirmingUndo === report.contentId}
                <span class="text-yellow-400">
                  undo?
                  <button
                    onclick={() => undoAction(report)}
                    disabled={actionInProgress === report.contentId}
                    class="text-yellow-500 hover:text-yellow-400 underline cursor-pointer disabled:opacity-50"
                  >
                    [confirm]
                  </button>
                  <button
                    onclick={() => { confirmingUndo = null; }}
                    class="text-terminal-dim hover:text-terminal-fg underline cursor-pointer"
                  >
                    [cancel]
                  </button>
                </span>
              {:else}
                <button
                  onclick={() => { confirmingUndo = report.contentId; }}
                  class="text-terminal-dim hover:text-terminal-fg transition-colors border border-terminal-border px-1.5 py-0.5"
                >
                  [undo]
                </button>
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    </div>

    <!-- Load more -->
    {#if nextCursor}
      <div class="mt-3">
        <button
          onclick={() => fetchReports(true)}
          disabled={loadingMore}
          class="text-xs text-terminal-dim hover:text-accent-500 transition-colors"
        >
          {loadingMore ? '[loading...]' : '[load more]'}
        </button>
      </div>
    {/if}
  {/if}
</div>

<!-- Ban dialog overlay -->
{#if banDialogTarget}
  <BanDialog
    {communityName}
    userId={banDialogTarget.userId}
    username={banDialogTarget.username}
    onClose={() => { banDialogTarget = null; }}
    onBanned={handleBanned}
  />
{/if}
