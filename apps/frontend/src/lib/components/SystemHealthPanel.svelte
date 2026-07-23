<script lang="ts">
  import { Activity, Layers, Ticket, Timer } from '@lucide/svelte';
  import Panel from '$lib/components/Panel.svelte';
  import { formatDurationMs, lagToneClass } from '$lib/format';
  import type { OrganizerMetrics } from '$lib/api/types';

  let {
    metrics = null,
    unavailable = false
  }: { metrics?: OrganizerMetrics | null; unavailable?: boolean } = $props();

  const tiles = $derived(
    metrics
      ? [
          {
            icon: Ticket,
            label: 'Reservations',
            value: metrics.totalReservations
          },
          { icon: Activity, label: 'Active Holds', value: metrics.activeHolds },
          {
            icon: Layers,
            label: 'Seats Remaining',
            value: metrics.seatsRemaining
          },
          {
            icon: Timer,
            label: 'Projection Lag',
            value: formatDurationMs(metrics.projectionLagMs),
            toneClass: lagToneClass(metrics.projectionLagMs)
          }
        ]
      : []
  );
  const sectionAvailability = $derived(
    Object.entries(metrics?.sectionAvailability ?? {})
  );
</script>

<Panel padding="lg">
  <div class="border-line mb-6 flex items-center gap-2 border-b pb-4">
    <Activity class="text-signal" size={20} />
    <h3 class="text-ink text-sm font-black tracking-wider uppercase">
      System Health
    </h3>
  </div>

  {#if unavailable}
    <p class="text-warn mb-4 text-xs font-bold tracking-widest uppercase">
      Live metrics unavailable
    </p>
  {/if}

  {#if !metrics}
    {#if !unavailable}
      <p class="text-inkMuted text-sm">Waiting for live metrics&hellip;</p>
    {/if}
  {:else}
    <div class="grid grid-cols-2 gap-4">
      {#each tiles as metric}
        {@const Icon = metric.icon}
        <div class="border-line bg-panelSoft/70 rounded-sm border p-4">
          <div class="text-inkMuted mb-2 flex items-center gap-2">
            <Icon size={14} />
            <span class="text-xs font-bold tracking-widest uppercase">
              {metric.label}
            </span>
          </div>
          <div
            class="font-mono text-2xl tabular-nums {metric.toneClass ??
              'text-ink'}"
          >
            {metric.value}
          </div>
        </div>
      {/each}
    </div>

    {#if sectionAvailability.length}
      <div class="border-line mt-6 space-y-2 border-t pt-4">
        <span class="text-inkMuted text-xs font-bold tracking-widest uppercase">
          Section Availability
        </span>
        {#each sectionAvailability as [section, pct]}
          <div class="flex items-center gap-3">
            <span
              class="text-inkMuted w-16 shrink-0 font-mono text-xs uppercase"
            >
              {section}
            </span>
            <div class="bg-carbon/70 h-1.5 w-full overflow-hidden rounded-full">
              <div
                class="bg-signal h-full rounded-full"
                style="width: {pct}%"
              ></div>
            </div>
            <span
              class="text-inkMuted w-10 shrink-0 text-right font-mono text-xs tabular-nums"
            >
              {pct}%
            </span>
          </div>
        {/each}
      </div>
    {/if}
  {/if}
</Panel>
