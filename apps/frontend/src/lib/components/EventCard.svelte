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
  class="group border-line bg-panelSoft hover:border-signal relative grid gap-3 rounded-sm border p-4 transition-all duration-300 hover:-translate-y-1 sm:grid-cols-[1fr_auto] sm:gap-4"
  href={`/events/${event.id}`}
>
  <div class="flex min-w-0 flex-col justify-center">
    <p
      class="text-ink group-hover:text-signal truncate text-lg font-black tracking-tight uppercase transition-colors sm:text-xl"
    >
      {event.title}
    </p>
    <p class="text-inkMuted mt-1 flex items-center gap-1.5 text-sm font-medium">
      <MapPin size={14} class="text-signal/80" />
      {event.venue}, <span class="text-ink/60">{event.city}</span>
    </p>
    <div
      class="text-inkMuted mt-3 flex flex-wrap items-center gap-4 text-xs font-semibold uppercase"
    >
      <span
        class="border-line bg-panel/70 text-inkMuted flex items-center gap-1.5 rounded-sm border px-2 py-1"
        ><CalendarDays size={13} class="text-signal" /> {eventTime}</span
      >
      <span
        class="border-line bg-panel/70 rounded-sm border px-2 py-1 font-mono tabular-nums"
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
    class="border-line flex items-center justify-between border-t pt-3 font-mono tabular-nums sm:flex-col sm:items-end sm:border-t-0 sm:pt-1 sm:pb-1"
  >
    <div
      class="border-line bg-panel group-hover:border-signal flex flex-col items-center gap-1 rounded-sm border p-2 transition-colors"
    >
      <Gauge class="text-signal" size={20} />
      <span class="text-ink text-2xl font-black">{event.demand_score}</span>
    </div>
    <span
      class="text-inkMuted text-[10px] font-medium tracking-widest uppercase"
      >{event.projection_lag_ms}ms</span
    >
  </div>
</a>
