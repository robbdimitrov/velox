<script lang="ts">
  import { onMount } from 'svelte';
  import { Activity, RadioTower, Ticket, Users } from '@lucide/svelte';
  import Panel from '$lib/components/Panel.svelte';
  import SurfaceStat from '$lib/components/SurfaceStat.svelte';

  let { data } = $props();

  let metrics = $state({
    totalReservations: 0,
    activeHolds: 0,
    seatsRemaining: 0,
    demandScore: 0,
    projectionLagMs: 0,
    sectionAvailability: {} as Record<string, number>
  });
  let streamState = $state<'connecting' | 'live' | 'degraded'>('connecting');

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
      detail: 'Currently held for review',
      icon: Users,
      tone: 'signal'
    },
    {
      label: 'Seats Remaining',
      value: metrics.seatsRemaining,
      detail: 'Available to reserve',
      icon: Ticket,
      tone: 'ok'
    }
  ] as const);

  onMount(() => {
    if (typeof EventSource === 'undefined') return;

    const source = new EventSource(
      `${data.gatewayBaseURL}/organizer/metrics/stream`
    );

    source.onmessage = (event) => {
      streamState = 'live';
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
      streamState = 'degraded';
      source.close();
    };

    return () => {
      source.close();
    };
  });
</script>

<svelte:head>
  <title>Organizer Analytics — Velox</title>
</svelte:head>

<main>
  <Panel padding="lg">
    <div
      class="border-line flex flex-col justify-between gap-4 border-b pb-6 sm:flex-row sm:items-end"
    >
      <div>
        <h1 class="text-ink text-3xl font-black tracking-tight uppercase">
          Organizer Analytics
        </h1>
        <p class="text-inkMuted mt-1 text-sm">
          Live operational read model for reservation health and inventory
          movement.
        </p>
      </div>
      <div
        class="border-line bg-panelSoft/70 flex items-center gap-3 rounded-sm border px-4 py-2"
      >
        <span class="relative flex h-3 w-3">
          {#if streamState === 'live'}
            <span
              class="bg-signal absolute inline-flex h-full w-full animate-ping rounded-full opacity-75"
            ></span>
          {/if}
          <span
            class="relative inline-flex h-3 w-3 rounded-full"
            class:bg-signal={streamState === 'live'}
            class:bg-warn={streamState === 'connecting'}
            class:bg-urgency={streamState === 'degraded'}
          ></span>
        </span>
        <span
          class="font-mono text-xs font-bold tracking-widest uppercase"
          class:text-signal={streamState === 'live'}
          class:text-warn={streamState === 'connecting'}
          class:text-urgency={streamState === 'degraded'}
          >{streamState === 'live'
            ? 'Live Updates'
            : streamState === 'connecting'
              ? 'Connecting'
              : 'Metrics Degraded'}</span
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
          class="border-line mb-6 flex items-center justify-between border-b pb-4"
        >
          <h2
            class="text-ink flex items-center gap-2 text-sm font-black tracking-wider uppercase"
          >
            <Activity size={18} class="text-signal" /> Section Availability
          </h2>
        </div>
        <div class="space-y-4">
          {#each Object.entries(metrics.sectionAvailability) as [section, percentage]}
            <div
              class="border-line bg-panelSoft/70 hover:border-signal/40 grid grid-cols-[90px_1fr_80px] items-center gap-4 rounded-sm border px-4 py-3 transition-colors"
            >
              <span class="text-ink font-mono text-sm font-bold"
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
              <span class="text-ink text-right font-mono text-sm font-black"
                >{percentage}%</span
              >
            </div>
          {/each}
          {#if Object.keys(metrics.sectionAvailability).length === 0}
            <div
              class="border-line bg-panelSoft/70 text-inkMuted rounded-sm border px-4 py-8 text-center text-sm font-bold tracking-widest uppercase"
            >
              Section metrics unavailable
            </div>
          {/if}
        </div>
      </Panel>

      <Panel padding="lg" flexColumn>
        <div
          class="border-line mb-6 flex items-center justify-between border-b pb-4"
        >
          <h2
            class="text-ink flex items-center gap-2 text-sm font-black tracking-wider uppercase"
          >
            <RadioTower size={18} class="text-signal" /> System Health
          </h2>
        </div>

        <div class="mb-8 grid grid-cols-2 gap-4">
          <div
            class="border-line bg-panelSoft/70 rounded-sm border p-4 shadow-inner"
          >
            <p
              class="text-inkMuted mb-1 text-[10px] font-bold tracking-widest uppercase"
            >
              Projection Lag
            </p>
            <p class="text-warn font-mono text-2xl font-black">
              {metrics.projectionLagMs}ms
            </p>
          </div>
          <div
            class="border-line bg-panelSoft/70 rounded-sm border p-4 shadow-inner"
          >
            <p
              class="text-inkMuted mb-1 text-[10px] font-bold tracking-widest uppercase"
            >
              Demand Score
            </p>
            <p class="text-ok font-mono text-2xl font-black">
              {metrics.demandScore}<span class="text-ink/40 text-sm">/100</span>
            </p>
          </div>
        </div>

        <div class="flex-1 space-y-4 font-mono text-sm">
          <div class="border-ok flex items-start gap-3 border-l-2 py-1 pl-4">
            <span
              class="mt-0.5"
              class:text-ok={streamState === 'live'}
              class:text-warn={streamState === 'connecting'}
              class:text-urgency={streamState === 'degraded'}>●</span
            >
            <div>
              <p class="text-ink font-bold">Metrics Stream</p>
              <p class="text-ink/60 mt-0.5 text-xs">{streamState}</p>
            </div>
          </div>
        </div>
      </Panel>
    </div>
  </Panel>
</main>
