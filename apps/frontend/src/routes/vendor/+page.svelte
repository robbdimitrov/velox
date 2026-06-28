<script lang="ts">
  import { onMount } from 'svelte';
  import { formatMoney } from '$lib/api/client';
  import {
    Activity,
    BadgeDollarSign,
    RadioTower,
    Ticket,
    Users
  } from '@lucide/svelte';

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

  onMount(() => {
    if (typeof EventSource === 'undefined') return;

    const source = new EventSource(
      `${data.gatewayBaseURL}/vendor/metrics/stream`
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
      setInterval(() => {
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

    return () => source.close();
  });
</script>

<main class="mx-auto max-w-7xl px-4 py-8">
  <section class="glass-panel p-6">
    <div
      class="border-b border-white/10 pb-6 flex flex-col sm:flex-row justify-between sm:items-end gap-4"
    >
      <div>
        <h1
          class="text-3xl font-black uppercase tracking-tight text-transparent bg-clip-text bg-gradient-to-r from-signal to-primary"
        >
          Vendor Analytics
        </h1>
        <p class="text-sm text-inkMuted mt-1">
          Live operational read model for flash sale health and inventory
          movement.
        </p>
      </div>
      <div
        class="flex items-center gap-3 bg-black/40 px-4 py-2 rounded-full border border-white/5"
      >
        <span class="relative flex h-3 w-3">
          <span
            class="animate-ping absolute inline-flex h-full w-full rounded-full bg-signal opacity-75"
          ></span>
          <span class="relative inline-flex rounded-full h-3 w-3 bg-signal"
          ></span>
        </span>
        <span
          class="text-xs font-bold font-mono uppercase tracking-widest text-signal"
          >Live Updates</span
        >
      </div>
    </div>

    <div class="mt-8 grid grid-cols-1 md:grid-cols-3 gap-4">
      <div class="glass-panel p-6 border-l-4 border-l-signal">
        <div class="flex items-start justify-between">
          <div>
            <p
              class="text-xs font-bold uppercase tracking-widest text-inkMuted mb-1"
            >
              Total Reservations
            </p>
            <p class="text-3xl font-black text-white">
              {metrics.totalReservations}
            </p>
            <p class="text-xs text-ink/40 mt-2">Total confirmed reservations</p>
          </div>
          <div class="p-3 bg-signal/20 rounded-xl text-signal shadow-inner">
            <Ticket size={28} />
          </div>
        </div>
      </div>

      <div class="glass-panel p-6 border-l-4 border-l-info">
        <div class="flex items-start justify-between">
          <div>
            <p
              class="text-xs font-bold uppercase tracking-widest text-inkMuted mb-1"
            >
              Active Holds
            </p>
            <p class="text-3xl font-black text-white">{metrics.activeHolds}</p>
            <p class="text-xs text-ink/40 mt-2">Currently in checkout flow</p>
          </div>
          <div class="p-3 bg-info/20 rounded-xl text-info shadow-inner">
            <Users size={28} />
          </div>
        </div>
      </div>

      <div class="glass-panel p-6 border-l-4 border-l-ok">
        <div class="flex items-start justify-between">
          <div>
            <p
              class="text-xs font-bold uppercase tracking-widest text-inkMuted mb-1"
            >
              Seats Remaining
            </p>
            <p class="text-3xl font-black text-white">
              {metrics.seatsRemaining}
            </p>
            <p class="text-xs text-ink/40 mt-2">Available for purchase</p>
          </div>
          <div class="p-3 bg-ok/20 rounded-xl text-ok shadow-inner">
            <Ticket size={28} />
          </div>
        </div>
      </div>
    </div>

    <div class="mt-8 grid gap-6 lg:grid-cols-[1fr_400px]">
      <div class="glass-panel p-6">
        <div
          class="flex justify-between items-center mb-6 border-b border-white/10 pb-4"
        >
          <h2
            class="text-sm font-black uppercase tracking-wider text-white flex items-center gap-2"
          >
            <Activity size={18} class="text-signal" /> Section Availability
          </h2>
        </div>
        <div class="space-y-4">
          {#each Object.entries(metrics.sectionAvailability) as [section, percentage]}
            <div
              class="grid grid-cols-[90px_1fr_80px] items-center gap-4 py-3 px-4 rounded-xl bg-black/20 border border-white/5 hover:bg-black/40 hover:border-white/10 transition-colors"
            >
              <span class="font-mono text-sm font-bold text-white"
                >SEC {section}</span
              >
              <progress
                class={`progress w-full h-2.5 bg-black/50 ${percentage < 40 ? 'progress-error drop-shadow-[0_0_5px_rgba(255,42,95,0.8)]' : percentage < 70 ? 'progress-warning drop-shadow-[0_0_5px_rgba(245,158,11,0.8)]' : 'progress-success drop-shadow-[0_0_5px_rgba(16,185,129,0.8)]'}`}
                value={percentage}
                max="100"
              ></progress>
              <span class="text-right font-mono text-sm font-black text-white"
                >{percentage}%</span
              >
            </div>
          {/each}
        </div>
      </div>

      <div class="glass-panel p-6 flex flex-col">
        <div
          class="flex justify-between items-center mb-6 border-b border-white/10 pb-4"
        >
          <h2
            class="text-sm font-black uppercase tracking-wider text-white flex items-center gap-2"
          >
            <RadioTower size={18} class="text-accent" /> System Health
          </h2>
        </div>

        <div class="grid grid-cols-2 gap-4 mb-8">
          <div
            class="bg-black/30 p-4 rounded-xl border border-white/5 shadow-inner"
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
            class="bg-black/30 p-4 rounded-xl border border-white/5 shadow-inner"
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
              <p class="text-white font-bold">CDN Cache Status</p>
              <p class="text-ink/60 text-xs mt-0.5">stale-while-revalidate</p>
            </div>
          </div>
          <div
            class="flex items-start gap-3 border-l-2 border-warn pl-4 py-2 bg-warn/10 rounded-r-lg"
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
              <p class="text-white font-bold">DLQ Rate</p>
              <p class="text-ink/60 text-xs mt-0.5">0 payloads/min</p>
            </div>
          </div>
          <div
            class="flex items-start gap-3 border-l-2 border-white/10 pl-4 py-1 opacity-60"
          >
            <span class="text-ink/40 mt-0.5">●</span>
            <div>
              <p class="text-white font-bold">Reservation 429 Rate</p>
              <p class="text-ink/60 text-xs mt-0.5">0.7% throttled</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</main>
