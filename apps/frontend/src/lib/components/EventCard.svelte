<script lang="ts">
  import { Clock, Gauge, MapPin } from '@lucide/svelte';
  import { formatMoney } from '$lib/api/client';
  import type { EventSummary } from '$lib/api/types';

  let { event }: { event: EventSummary } = $props();

  const scarcityTone = $derived(
    event.remaining_bucket === 'LOW'
      ? 'text-accent drop-shadow-[0_0_8px_rgba(255,42,95,0.8)]'
      : event.remaining_bucket === 'MEDIUM'
        ? 'text-warn drop-shadow-[0_0_8px_rgba(245,158,11,0.8)]'
        : 'text-ok drop-shadow-[0_0_8px_rgba(16,185,129,0.8)]'
  );
  
  const saleTime = $derived(
    new Intl.DateTimeFormat('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    }).format(new Date(event.sale_starts_at))
  );
</script>

<a
  class="group relative grid grid-cols-[110px_1fr_auto] gap-4 rounded-xl border border-white/5 bg-black/40 p-3 transition-all duration-300 hover:-translate-y-1 hover:border-signal/40 hover:bg-black/60 hover:shadow-glow"
  href={`/events/${event.id}`}
>
  <div class="overflow-hidden rounded-lg shadow-md bg-carbon">
    <img
      class="h-24 w-full object-cover transition-transform duration-500 group-hover:scale-110"
      src={event.image_url}
      alt=""
    />
  </div>
  <div class="flex flex-col justify-center min-w-0">
    <p class="truncate text-xl font-black uppercase tracking-tight text-white group-hover:text-signal transition-colors">
      {event.title}
    </p>
    <p class="mt-1 flex items-center gap-1.5 text-sm text-inkMuted font-medium">
      <MapPin size={14} class="text-signal/80" />
      {event.venue}, <span class="text-ink/60">{event.city}</span>
    </p>
    <div class="mt-3 flex flex-wrap items-center gap-4 text-xs font-semibold uppercase text-inkMuted">
      <span class="flex items-center gap-1.5 bg-white/5 px-2 py-1 rounded-md shadow-sm"><Clock size={13} class="text-info" /> {saleTime}</span>
      <span class={`font-mono px-2 py-1 bg-white/5 rounded-md shadow-sm ${scarcityTone}`}>
        {event.remaining_bucket.replace('_', ' ')}
      </span>

    </div>
  </div>
  <div class="flex flex-col items-end justify-between py-1 font-mono">
    <div class="bg-black/50 p-2 rounded-lg border border-white/5 flex flex-col items-center gap-1 shadow-inner group-hover:border-signal/30 transition-colors">
      <Gauge class="text-signal group-hover:animate-spin-slow" size={20} />
      <span class="text-2xl font-black text-white">{event.demand_score}</span>
    </div>
    <span class="text-[10px] font-medium uppercase text-ink/40 tracking-widest">{event.projection_lag_ms}ms</span>
  </div>
</a>

<style>
  :global(.animate-spin-slow) {
    animation: spin 3s linear infinite;
  }
</style>
