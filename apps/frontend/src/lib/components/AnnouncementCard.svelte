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
  <div class="border-urgency/50 bg-urgency/10 rounded-sm border p-3">
    <div class="flex items-center justify-between gap-3">
      <h3 class="text-urgency text-sm font-bold">{announcement.title}</h3>
      <span
        class="text-inkMuted shrink-0 font-mono text-[10px] tracking-widest uppercase tabular-nums"
      >
        {formatUpdateTime(announcement.created_at)}
      </span>
    </div>
    <p class="text-ink/80 mt-1 text-xs leading-relaxed">
      {announcement.body}
    </p>
  </div>
{:else if announcement.severity === 'SCHEDULE_CHANGE'}
  <div class="border-warning/30 bg-warning/10 rounded-sm border p-3">
    <div class="flex items-center justify-between gap-3">
      <h3 class="text-warning text-sm font-bold">{announcement.title}</h3>
      <span
        class="text-inkMuted shrink-0 font-mono text-[10px] tracking-widest uppercase tabular-nums"
      >
        {formatUpdateTime(announcement.created_at)}
      </span>
    </div>
    <p class="text-ink/80 mt-1 text-xs leading-relaxed">
      {announcement.body}
    </p>
  </div>
{:else}
  <div class="border-line bg-panelSoft/70 rounded-sm border p-3">
    <div class="flex items-center justify-between gap-3">
      <h3 class="text-ink text-sm font-bold">{announcement.title}</h3>
      <span
        class="text-inkMuted shrink-0 font-mono text-[10px] tracking-widest uppercase tabular-nums"
      >
        {formatUpdateTime(announcement.created_at)}
      </span>
    </div>
    <p class="text-ink/80 mt-1 text-xs leading-relaxed">
      {announcement.body}
    </p>
  </div>
{/if}
