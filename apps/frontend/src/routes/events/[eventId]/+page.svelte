<script lang="ts">
  import { goto } from '$app/navigation';
  import { createIdempotencyKey, createGatewayClient, formatMoney } from '$lib/api/client';
  import { makeMockReservation } from '$lib/api/mock';
  import type { SeatDelta } from '$lib/api/types';
  import SeatCanvas from '$lib/components/SeatCanvas.svelte';
  import { checkoutState } from '$lib/state/checkout-state.svelte';
  import { SeatSelectionState } from '$lib/state/seat-state.svelte';
  import { Accessibility, Minus, Plus, RotateCcw, TicketCheck } from 'lucide-svelte';
  import { onMount } from 'svelte';

  let { data } = $props();
  const seatState = new SeatSelectionState();
  let reserving = $state(false);
  let error = $state('');
  let sectionID = $state(data.snapshot.section_id);
  let eventLog = $state<string[]>(['Snapshot loaded from projection read model']);

  seatState.load(data.snapshot.seats, data.snapshot.server_time_ms);

  onMount(() => {
    if (typeof WebSocket === 'undefined') return;
    const socket = new WebSocket(data.seatSocketURL);
    socket.onmessage = (message) => {
      const delta = JSON.parse(message.data) as SeatDelta;
      seatState.applyDelta(delta);
      eventLog = [`${delta.seat_id} ${delta.status} v${delta.version}`, ...eventLog].slice(0, 6);
    };
    socket.onerror = () => socket.close();
    return () => socket.close();
  });

  async function reserve() {
    if (!seatState.selectedSeats.length) return;
    reserving = true;
    error = '';

    const client = createGatewayClient(fetch, data.gatewayBaseURL);
    try {
      const reservation = await client.reserveSeats(
        {
          event_id: data.snapshot.event_id,
          section_id: sectionID,
          seat_ids: seatState.selectedSeats.map((seat) => seat.seat_id),
          expected_versions: seatState.expectedVersions()
        },
        createIdempotencyKey()
      );
      checkoutState.load(reservation);
      await goto('/checkout');
    } catch (requestError) {
      if (requestError instanceof Error && data.snapshot.event_id.startsWith('evt_')) {
        const reservation = makeMockReservation(seatState.selectedSeats.map((seat) => seat.seat_id));
        checkoutState.load(reservation);
        await goto('/checkout');
        return;
      }
      error = 'Reservation rejected by gateway. Refresh seat state and try again.';
    } finally {
      reserving = false;
    }
  }
</script>

<main class="mx-auto grid max-w-7xl gap-4 px-4 py-5 lg:grid-cols-[210px_1fr_320px]">
  <aside class="thin-panel h-max p-4">
    <h2 class="border-b border-line pb-3 text-sm font-black uppercase">Section Tools</h2>
    <label class="form-control mt-4">
      <span class="label-text text-ink/70">Section</span>
      <select bind:value={sectionID} class="select select-bordered select-sm border-line bg-carbon">
        <option>A</option>
        <option>B</option>
        <option>C</option>
      </select>
    </label>
    <div class="mt-4 grid grid-cols-2 gap-2">
      <button class="btn btn-sm border-line bg-panel"><Plus size={16} /> Zoom</button>
      <button class="btn btn-sm border-line bg-panel"><Minus size={16} /> Zoom</button>
    </div>
    <label class="mt-4 flex items-center gap-2 text-sm">
      <input class="toggle toggle-primary toggle-sm" type="checkbox" />
      <Accessibility size={16} /> Accessible
    </label>
    <div class="mt-4 space-y-2 border-t border-line pt-4 text-xs uppercase text-ink/60">
      <p><span class="inline-block h-2 w-2 bg-[#7A7A86]"></span> Available</p>
      <p><span class="inline-block h-2 w-2 bg-signal"></span> Selected</p>
      <p><span class="inline-block h-2 w-2 bg-urgency"></span> Held</p>
      <p><span class="inline-block h-2 w-2 bg-carbon ring-1 ring-line"></span> Sold</p>
    </div>
  </aside>

  <section class="min-w-0">
    <div class="mb-3 flex items-end justify-between">
      <div>
        <h1 class="text-2xl font-black uppercase">{data.event.title}</h1>
        <p class="text-sm text-ink/60">{data.event.venue} · section {sectionID}</p>
      </div>
      <p class="font-mono text-xs text-ink/50">
        snapshot {data.snapshot.snapshot_age_ms}ms · lag {data.snapshot.projection_lag_ms}ms
      </p>
    </div>
    <SeatCanvas seats={seatState.seats} selectedSeatIDs={seatState.selectedSeatIDs} onToggle={(seat) => seatState.toggleSeat(seat)} />
    <div class="mt-3 thin-panel h-28 overflow-hidden p-3">
      <p class="mb-2 text-xs font-black uppercase text-ink/50">Live inventory log</p>
      {#each eventLog as item}
        <p class="font-mono text-xs text-ink/70">{item}</p>
      {/each}
    </div>
  </section>

  <aside class="thin-panel h-max p-4">
    <div class="flex items-center justify-between border-b border-line pb-3">
      <h2 class="text-sm font-black uppercase">Selected Seats</h2>
      <button class="btn btn-ghost btn-xs" onclick={() => seatState.load(data.snapshot.seats, data.snapshot.server_time_ms)}>
        <RotateCcw size={15} />
      </button>
    </div>
    <div class="mt-4 min-h-36 space-y-2">
      {#if seatState.selectedSeats.length}
        {#each seatState.selectedSeats as seat}
          <div class="flex items-center justify-between border border-line bg-carbon p-2 font-mono text-sm">
            <span>{seat.seat_id}</span>
            <span>{formatMoney(seat.price_cents)}</span>
          </div>
        {/each}
      {:else}
        <p class="text-sm text-ink/50">Choose available seats from the map.</p>
      {/if}
    </div>
    <div class="mt-4 border-t border-line pt-4">
      <div class="flex justify-between font-mono">
        <span>Total</span>
        <strong>{formatMoney(seatState.selectedTotalCents)}</strong>
      </div>
      {#if error}
        <p class="mt-3 border border-urgency px-2 py-1 text-sm text-urgency">{error}</p>
      {/if}
      <button class="btn btn-primary mt-4 w-full" disabled={!seatState.selectedSeats.length || reserving} onclick={reserve}>
        <TicketCheck size={17} />
        {reserving ? 'Holding' : 'Reserve'}
      </button>
    </div>
  </aside>
</main>
