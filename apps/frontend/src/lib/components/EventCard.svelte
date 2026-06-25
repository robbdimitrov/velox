<script lang="ts">
  import { Clock, Gauge, MapPin } from 'lucide-svelte';
  import { formatMoney } from '$lib/api/client';
  import type { EventSummary } from '$lib/api/types';

  let { event }: { event: EventSummary } = $props();

  const scarcityTone = $derived(
    event.remaining_bucket === 'LOW'
      ? 'text-urgency'
      : event.remaining_bucket === 'MEDIUM'
        ? 'text-warn'
        : 'text-ok'
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

<a class="grid grid-cols-[96px_1fr_auto] gap-3 border-b border-line py-3 transition hover:bg-panel/70" href={`/events/${event.id}`}>
  <img class="h-20 w-24 border border-line object-cover" src={event.image_url} alt="" />
  <div class="min-w-0">
    <p class="truncate text-lg font-black uppercase tracking-normal text-ink">{event.title}</p>
    <p class="mt-1 flex items-center gap-1 text-sm text-ink/70"><MapPin size={15} /> {event.venue}, {event.city}</p>
    <div class="mt-2 flex flex-wrap gap-3 text-xs uppercase text-ink/60">
      <span class="flex items-center gap-1"><Clock size={14} /> {saleTime}</span>
      <span class={`font-mono ${scarcityTone}`}>{event.remaining_bucket.replace('_', ' ')}</span>
      <span>{formatMoney(event.min_price_cents)} floor</span>
    </div>
  </div>
  <div class="flex min-w-16 flex-col items-end justify-between text-right font-mono">
    <Gauge class="text-signal" size={18} />
    <span class="text-2xl font-black">{event.demand_score}</span>
    <span class="text-[10px] uppercase text-ink/50">{event.projection_lag_ms}ms lag</span>
  </div>
</a>
