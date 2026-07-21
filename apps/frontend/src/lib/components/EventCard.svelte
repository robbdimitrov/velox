<script lang="ts">
  import { CalendarDays, Gauge, MapPin } from '@lucide/svelte';
  import type { EventSummary } from '$lib/api/types';

  let { event }: { event: EventSummary } = $props();

  const eventTimeFormatter = new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  });
  const eventTime = $derived(formatEventTime(event.starts_at));

  function formatEventTime(value: string | undefined) {
    if (!value) return 'Start TBA';
    const timestamp = new Date(value).getTime();
    if (!Number.isFinite(timestamp)) return 'Start TBA';
    return eventTimeFormatter.format(new Date(timestamp));
  }
</script>

<a
  class="group relative grid gap-3 rounded-sm border border-line bg-panelSoft p-4 transition-all duration-300 hover:-translate-y-1 hover:border-signal sm:grid-cols-[1fr_auto] sm:gap-4"
  href={`/events/${event.id}`}
>
  <div class="flex flex-col justify-center min-w-0">
    <p
      class="truncate text-lg font-black uppercase tracking-tight text-ink transition-colors group-hover:text-signal sm:text-xl"
    >
      {event.title}
    </p>
    <p class="mt-1 flex items-center gap-1.5 text-sm text-inkMuted font-medium">
      <MapPin size={14} class="text-signal/80" />
      {event.venue}, <span class="text-ink/60">{event.city}</span>
    </p>
    <div
      class="mt-3 flex flex-wrap items-center gap-4 text-xs font-semibold uppercase text-inkMuted"
    >
      <span
        class="flex items-center gap-1.5 rounded-sm border border-line bg-panel/70 px-2 py-1 text-inkMuted"
        ><CalendarDays size={13} class="text-signal" /> {eventTime}</span
      >
      <span
        class="rounded-sm border border-line bg-panel/70 px-2 py-1 font-mono tabular-nums"
        class:text-urgency={event.remaining_bucket === 'FULL' ||
          event.remaining_bucket === 'LOW'}
        class:text-warn={event.remaining_bucket === 'MEDIUM'}
        class:text-ok={event.remaining_bucket === 'HIGH'}
      >
        {event.remaining_bucket}
      </span>
    </div>
  </div>
  <div
    class="font-mono tabular-nums flex items-center justify-between border-t border-line pt-3 sm:flex-col sm:items-end sm:border-t-0 sm:pb-1 sm:pt-1"
  >
    <div
      class="flex flex-col items-center gap-1 rounded-sm border border-line bg-panel p-2 transition-colors group-hover:border-signal"
    >
      <Gauge class="text-signal" size={20} />
      <span class="text-2xl font-black text-ink">{event.demand_score}</span>
    </div>
    <span
      class="text-[10px] font-medium uppercase tracking-widest text-inkMuted"
      >{event.projection_lag_ms}ms</span
    >
  </div>
</a>
