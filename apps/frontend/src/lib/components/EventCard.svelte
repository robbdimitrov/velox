<script lang="ts">
  import { Clock, Gauge, MapPin } from '@lucide/svelte';
  import type { EventSummary } from '$lib/api/types';

  let { event }: { event: EventSummary } = $props();

  const saleTimeFormatter = new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  });
  const saleTime = $derived(formatSaleTime(event.sale_starts_at));

  function formatSaleTime(value: string) {
    const timestamp = new Date(value).getTime();
    if (!Number.isFinite(timestamp)) return 'Sale TBA';
    return saleTimeFormatter.format(new Date(timestamp));
  }
</script>

<a
  class="group relative grid grid-cols-[92px_1fr] gap-3 rounded-sm border border-line bg-panelSoft p-3 transition-all duration-300 hover:-translate-y-1 hover:border-signal sm:grid-cols-[118px_1fr_auto] sm:gap-4"
  href={`/events/${event.id}`}
>
  <div class="overflow-hidden rounded-sm bg-carbon">
    <img
      class="h-20 w-full object-cover transition-transform duration-500 group-hover:scale-110 sm:h-24"
      src={event.image_url}
      alt=""
    />
  </div>
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
        ><Clock size={13} class="text-signal" /> {saleTime}</span
      >
      <span
        class="rounded-sm border border-line bg-panel/70 px-2 py-1 font-mono tabular-nums"
        class:text-urgency={event.remaining_bucket === 'SOLD_OUT' ||
          event.remaining_bucket === 'LOW'}
        class:text-warn={event.remaining_bucket === 'MEDIUM'}
        class:text-ok={event.remaining_bucket === 'HIGH'}
      >
        {event.remaining_bucket.replace('_', ' ')}
      </span>
    </div>
  </div>
  <div
    class="font-mono tabular-nums col-span-2 flex items-center justify-between border-t border-line pt-3 sm:col-span-1 sm:flex-col sm:items-end sm:border-t-0 sm:pb-1 sm:pt-1"
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
