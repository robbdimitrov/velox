<script lang="ts">
  import type { EventAnnouncement } from '$lib/api/types';

  let { announcement }: { announcement: EventAnnouncement } = $props();

  function formatUpdateTime(isoTimestamp: string) {
    const parsed = new Date(isoTimestamp);
    if (Number.isNaN(parsed.getTime())) return isoTimestamp;
    return parsed.toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit'
    });
  }
</script>

{#if announcement.severity === 'CANCELLATION'}
  <div class="rounded-sm border border-urgency/50 bg-urgency/10 p-3">
    <div class="flex items-center justify-between gap-3">
      <h3 class="text-sm font-bold text-urgency">{announcement.title}</h3>
      <span
        class="shrink-0 font-mono text-[10px] uppercase tracking-widest tabular-nums text-inkMuted"
      >
        {formatUpdateTime(announcement.created_at)}
      </span>
    </div>
    <p class="mt-1 text-xs leading-relaxed text-ink/80">
      {announcement.body}
    </p>
  </div>
{:else if announcement.severity === 'SCHEDULE_CHANGE'}
  <div class="rounded-sm border border-warning/30 bg-warning/10 p-3">
    <div class="flex items-center justify-between gap-3">
      <h3 class="text-sm font-bold text-warning">{announcement.title}</h3>
      <span
        class="shrink-0 font-mono text-[10px] uppercase tracking-widest tabular-nums text-inkMuted"
      >
        {formatUpdateTime(announcement.created_at)}
      </span>
    </div>
    <p class="mt-1 text-xs leading-relaxed text-ink/80">
      {announcement.body}
    </p>
  </div>
{:else}
  <div class="rounded-sm border border-line bg-panelSoft/70 p-3">
    <div class="flex items-center justify-between gap-3">
      <h3 class="text-sm font-bold text-ink">{announcement.title}</h3>
      <span
        class="shrink-0 font-mono text-[10px] uppercase tracking-widest tabular-nums text-inkMuted"
      >
        {formatUpdateTime(announcement.created_at)}
      </span>
    </div>
    <p class="mt-1 text-xs leading-relaxed text-ink/80">
      {announcement.body}
    </p>
  </div>
{/if}
