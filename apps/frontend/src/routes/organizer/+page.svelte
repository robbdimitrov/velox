<script lang="ts">
  import { onMount } from 'svelte';
  import { Activity, RadioTower, Ticket, Users } from '@lucide/svelte';
  import Panel from '$lib/components/Panel.svelte';
  import SurfaceStat from '$lib/components/SurfaceStat.svelte';

  let { data } = $props();

  let metrics = $state({
    totalReservations: 14800,
    activeHolds: 342,
    seatsRemaining: 4105,
    demandScore: 98,
    projectionLagMs: 81,
    sectionAvailability: {
      A: 82,
      B: 71,
      C: 60,
      Floor: 49,
      Balcony: 38
    } as Record<string, number>
  });

  let summaryCards = $derived([
    {
      label: 'Total Reservations',
      value: metrics.totalReservations,
      detail: 'Total confirmed reservations',
      icon: Ticket,
      tone: 'signal'
    },
    {
      label: 'Active Holds',
      value: metrics.activeHolds,
      detail: 'Currently in checkout flow',
      icon: Users,
      tone: 'signal'
    },
    {
      label: 'Seats Remaining',
      value: metrics.seatsRemaining,
      detail: 'Available for purchase',
      icon: Ticket,
      tone: 'ok'
    }
  ] as const);

  onMount(() => {
    if (typeof EventSource === 'undefined') return;
    let fallbackInterval: ReturnType<typeof setInterval> | undefined;

    const source = new EventSource(
      `${data.gatewayBaseURL}/organizer/metrics/stream`
    );

    source.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data);
        if (payload.totalReservations !== undefined)
          metrics.totalReservations = payload.totalReservations;
        if (payload.activeHolds !== undefined)
          metrics.activeHolds = payload.activeHolds;
        if (payload.seatsRemaining !== undefined)
          metrics.seatsRemaining = payload.seatsRemaining;
        if (payload.demandScore !== undefined)
          metrics.demandScore = payload.demandScore;
        if (payload.projectionLagMs !== undefined)
          metrics.projectionLagMs = payload.projectionLagMs;
        if (payload.sectionAvailability !== undefined)
          metrics.sectionAvailability = payload.sectionAvailability;
      } catch (e) {
        console.error('Failed to parse metric payload', e);
      }
    };

    source.onerror = () => {
      source.close();
      fallbackInterval ??= setInterval(() => {
        metrics.totalReservations += Math.floor(Math.random() * 5);
        metrics.activeHolds = Math.max(
          0,
          metrics.activeHolds + Math.floor(Math.random() * 10) - 5
        );
        metrics.seatsRemaining = Math.max(
          0,
          metrics.seatsRemaining - Math.floor(Math.random() * 5)
        );
        metrics.projectionLagMs = 70 + Math.floor(Math.random() * 20);
      }, 2000);
    };

    return () => {
      source.close();
      if (fallbackInterval) clearInterval(fallbackInterval);
    };
  });
</script>

<main>
  <Panel padding="lg">
    <div
      class="flex flex-col justify-between gap-4 border-b border-line pb-6 sm:flex-row sm:items-end"
    >
      <div>
        <h1 class="text-3xl font-black uppercase tracking-tight text-ink">
          Organizer Analytics
        </h1>
        <p class="text-sm text-inkMuted mt-1">
          Live operational read model for flash sale health and inventory
          movement.
        </p>
      </div>
      <div
        class="flex items-center gap-3 rounded-sm border border-line bg-panelSoft/70 px-4 py-2"
      >
        <span class="relative flex h-3 w-3">
          <span
            class="animate-ping absolute inline-flex h-full w-full rounded-full bg-signal opacity-75"
          ></span>
          <span class="relative inline-flex rounded-full h-3 w-3 bg-signal"
          ></span>
        </span>
        <span
          class="font-mono text-xs font-bold uppercase tracking-widest text-signal"
          >Live Updates</span
        >
      </div>
    </div>

    <div class="mt-8 grid grid-cols-1 gap-4 md:grid-cols-3">
      {#each summaryCards as card}
        <SurfaceStat {...card} />
      {/each}
    </div>

    <div class="mt-8 grid gap-6 lg:grid-cols-[1fr_400px]">
      <Panel padding="lg">
        <div
          class="mb-6 flex items-center justify-between border-b border-line pb-4"
        >
          <h2
            class="flex items-center gap-2 text-sm font-black uppercase tracking-wider text-ink"
          >
            <Activity size={18} class="text-signal" /> Section Availability
          </h2>
        </div>
        <div class="space-y-4">
          {#each Object.entries(metrics.sectionAvailability) as [section, percentage]}
            <div
              class="grid grid-cols-[90px_1fr_80px] items-center gap-4 rounded-sm border border-line bg-panelSoft/70 px-4 py-3 transition-colors hover:border-signal/40"
            >
              <span class="font-mono text-sm font-bold text-ink"
                >SEC {section}</span
              >
              <progress
                class="progress h-2.5 w-full bg-black/50"
                class:progress-error={percentage < 40}
                class:progress-warning={percentage >= 40 && percentage < 70}
                class:progress-success={percentage >= 70}
                value={percentage}
                max="100"
              ></progress>
              <span class="text-right font-mono text-sm font-black text-ink"
                >{percentage}%</span
              >
            </div>
          {/each}
        </div>
      </Panel>

      <Panel padding="lg" flexColumn>
        <div
          class="mb-6 flex items-center justify-between border-b border-line pb-4"
        >
          <h2
            class="flex items-center gap-2 text-sm font-black uppercase tracking-wider text-ink"
          >
            <RadioTower size={18} class="text-signal" /> System Health
          </h2>
        </div>

        <div class="grid grid-cols-2 gap-4 mb-8">
          <div
            class="rounded-sm border border-line bg-panelSoft/70 p-4 shadow-inner"
          >
            <p
              class="text-[10px] uppercase font-bold tracking-widest text-inkMuted mb-1"
            >
              Projection Lag
            </p>
            <p class="font-mono text-2xl font-black text-warn">
              {metrics.projectionLagMs}ms
            </p>
          </div>
          <div
            class="rounded-sm border border-line bg-panelSoft/70 p-4 shadow-inner"
          >
            <p
              class="text-[10px] uppercase font-bold tracking-widest text-inkMuted mb-1"
            >
              Demand Score
            </p>
            <p class="font-mono text-2xl font-black text-ok">
              {metrics.demandScore}<span class="text-sm text-ink/40">/100</span>
            </p>
          </div>
        </div>

        <div class="space-y-4 font-mono text-sm flex-1">
          <div class="flex items-start gap-3 border-l-2 border-ok pl-4 py-1">
            <span class="text-ok mt-0.5 animate-pulse">●</span>
            <div>
              <p class="font-bold text-ink">CDN Cache Status</p>
              <p class="text-ink/60 text-xs mt-0.5">stale-while-revalidate</p>
            </div>
          </div>
          <div
            class="flex items-start gap-3 border-l-2 border-warn pl-4 py-2 bg-warn/10 rounded-r"
          >
            <span class="text-warn mt-0.5">▲</span>
            <div>
              <p class="text-warn font-bold">Queue Pressure</p>
              <p class="text-inkMuted text-xs mt-0.5">Elevated on section A</p>
            </div>
          </div>
          <div
            class="flex items-start gap-3 border-l-2 border-white/10 pl-4 py-1 opacity-60"
          >
            <span class="text-ink/40 mt-0.5">●</span>
            <div>
              <p class="font-bold text-ink">DLQ Rate</p>
              <p class="text-ink/60 text-xs mt-0.5">0 payloads/min</p>
            </div>
          </div>
          <div
            class="flex items-start gap-3 border-l-2 border-white/10 pl-4 py-1 opacity-60"
          >
            <span class="text-ink/40 mt-0.5">●</span>
            <div>
              <p class="font-bold text-ink">Reservation 429 Rate</p>
              <p class="text-ink/60 text-xs mt-0.5">0.7% throttled</p>
            </div>
          </div>
        </div>
      </Panel>
    </div>
  </Panel>
</main>
