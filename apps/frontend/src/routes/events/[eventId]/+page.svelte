<script lang="ts">
  import { goto } from '$app/navigation';
  import {
    createIdempotencyKey,
    createGatewayClient,
    formatMoney
  } from '$lib/api/client';
  import type { SeatDelta } from '$lib/api/types';
  import SeatCanvas from '$lib/components/SeatCanvas.svelte';
  import VirtualWaitingRoom from '$lib/components/VirtualWaitingRoom.svelte';
  import { checkoutState } from '$lib/state/checkout-state.svelte';
  import { SeatSelectionState } from '$lib/state/seat-state.svelte';
  import {
    Accessibility,
    Minus,
    Plus,
    RotateCcw,
    TicketCheck,
    Layers
  } from '@lucide/svelte';
  import PrimaryButton from '$lib/components/PrimaryButton.svelte';
  import { onMount } from 'svelte';
  import { slide } from 'svelte/transition';

  let { data } = $props();
  const seatState = new SeatSelectionState();
  let reservingStatus = $state('');
  let reserving = $derived(reservingStatus !== '');
  let error = $state('');
  let eventLog: string[] = $state([]);
  let sectionID = $state('');
  let zoomLevel = $state(1);
  let accessibleOnly = $state(false);

  $effect(() => {
    if (data.isRateLimited) return;
    sectionID = data.snapshot.section_id;
    seatState.load(data.snapshot.seats, data.snapshot.server_time_ms);
    eventLog = ['Snapshot loaded from projection read model'];
    // Reset zoom when section changes
    zoomLevel = 1;
  });

  $effect(() => {
    if (data.isRateLimited) return;
    if (typeof EventSource === 'undefined') return;

    const source = new EventSource(data.seatSseURL);
    source.onmessage = (event) => {
      try {
        const delta = JSON.parse(event.data) as SeatDelta;
        seatState.applyDelta(delta);
        eventLog = [
          `${delta.seat_id} ${delta.status} v${delta.version}`,
          ...eventLog
        ].slice(0, 6);
      } catch {
        eventLog = ['Ignored malformed seat update', ...eventLog].slice(0, 6);
      }
    };
    source.onerror = () => {
      eventLog = ['Connection lost, reconnecting...', ...eventLog].slice(0, 6);
    };

    return () => {
      source.close();
    };
  });

  async function reserve() {
    if (!seatState.selectedSeats.length) return;
    reservingStatus = 'Holding Seat...';
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

      reservingStatus = 'Confirming...';
      await new Promise((r) => setTimeout(r, 800));
      reservingStatus = 'Reserved!';
      await new Promise((r) => setTimeout(r, 400));

      checkoutState.load(reservation);
      await goto('/checkout');
    } catch (requestError) {
      error =
        'Reservation rejected by gateway. Refresh seat state and try again.';
      reservingStatus = '';
    }
  }
</script>

{#if data.isRateLimited}
  <VirtualWaitingRoom />
{:else}
  <main
    class="mx-auto grid max-w-7xl gap-6 px-4 py-8 lg:grid-cols-[260px_1fr_320px]"
  >
    <aside class="glass-panel h-max p-6 sticky top-28">
      <div class="flex items-center gap-3 border-b border-white/10 pb-4 mb-6">
        <div
          class="p-2 bg-gradient-to-br from-signal to-primary rounded-lg shadow-md"
        >
          <Layers class="text-white" size={18} />
        </div>
        <h2 class="text-sm font-black uppercase tracking-wider text-white">
          Section Tools
        </h2>
      </div>

      <label class="form-control mb-6">
        <span class="label-text text-inkMuted font-medium mb-1.5">Section</span>
        <select
          bind:value={sectionID}
          onchange={() => goto(`?section_id=${sectionID}`)}
          class="select select-bordered select-sm border-white/10 bg-black/40 text-ink rounded-lg focus:border-signal"
        >
          <option>A</option>
          <option>B</option>
          <option>C</option>
        </select>
      </label>

      <div class="grid grid-cols-2 gap-3 mb-6">
        <button
          class="btn btn-sm border-white/10 bg-black/40 text-ink hover:bg-black/60 rounded-lg shadow-inner"
          onclick={() => (zoomLevel = Math.min(3, zoomLevel * 1.2))}
          ><Plus size={16} /> Zoom</button
        >
        <button
          class="btn btn-sm border-white/10 bg-black/40 text-ink hover:bg-black/60 rounded-lg shadow-inner"
          onclick={() => (zoomLevel = Math.max(0.5, zoomLevel / 1.2))}
          ><Minus size={16} /> Zoom</button
        >
      </div>

      <label
        class="flex items-center gap-3 text-sm cursor-pointer hover:text-white transition-colors group bg-black/20 p-3 rounded-lg border border-white/5 mb-6"
      >
        <input
          bind:checked={accessibleOnly}
          class="toggle toggle-primary toggle-sm"
          type="checkbox"
        />
        <span
          class="flex items-center gap-2 group-hover:text-signal transition-colors"
          ><Accessibility size={16} /> Accessible</span
        >
      </label>

      <div
        class="space-y-3 border-t border-white/10 pt-6 text-xs uppercase font-bold tracking-wider text-inkMuted"
      >
        <p class="flex items-center gap-3">
          <span class="inline-block h-3 w-3 rounded-full bg-[#9CA3AF]"></span> Available
        </p>
        <p class="flex items-center gap-3">
          <span
            class="inline-block h-3 w-3 rounded-full bg-signal shadow-[0_0_8px_rgba(124,58,237,0.8)]"
          ></span> Selected
        </p>
        <p class="flex items-center gap-3">
          <span
            class="inline-block h-3 w-3 rounded-full bg-accent shadow-[0_0_8px_rgba(255,42,95,0.8)]"
          ></span> Held
        </p>
        <p class="flex items-center gap-3">
          <span
            class="inline-block h-3 w-3 rounded-full bg-[#15151A] border border-white/20"
          ></span> Sold
        </p>
      </div>
    </aside>

    <section class="min-w-0 flex flex-col gap-4">
      <div
        class="glass-panel p-6 flex flex-col sm:flex-row sm:items-end justify-between gap-4"
      >
        <div>
          <h1
            class="text-3xl font-black uppercase tracking-tight text-white drop-shadow-md"
          >
            {data.event?.title}
          </h1>
          <p
            class="text-sm font-medium text-signal mt-1 uppercase tracking-wide"
          >
            {data.event?.venue} <span class="text-inkMuted mx-2">|</span>
            Section {sectionID}
          </p>
        </div>
        <div
          class="bg-black/40 px-3 py-1.5 rounded-full border border-white/5 flex flex-col items-end"
        >
          <p class="font-mono text-xs text-ink/60 uppercase tracking-widest">
            Snapshot <span class="text-white"
              >{data.snapshot.snapshot_age_ms}ms</span
            >
          </p>
          <p
            class="font-mono text-xs text-ink/60 uppercase tracking-widest mt-0.5"
          >
            Lag <span class="text-ok">{data.snapshot.projection_lag_ms}ms</span>
          </p>
        </div>
      </div>

      <SeatCanvas
        seats={seatState.seats}
        selectedSeatIDs={seatState.selectedSeatIDs}
        onToggle={(seat) => seatState.toggleSeat(seat)}
        {sectionID}
        {zoomLevel}
        {accessibleOnly}
      />

      <div class="glass-panel h-32 overflow-hidden p-4 relative">
        <p
          class="mb-3 text-xs font-black uppercase tracking-widest text-inkMuted border-b border-white/10 pb-2"
        >
          Live inventory log
        </p>
        <div class="space-y-1.5">
          {#each eventLog as item, index (item)}
            <p
              class="font-mono text-xs text-ink/80 truncate hover:text-white transition-colors"
              in:slide={{ duration: 200 }}
            >
              <span class="text-signal mr-2">›</span>{item}
            </p>
          {/each}
        </div>
      </div>
    </section>

    <aside
      class="glass-panel h-max p-6 sticky top-28 flex flex-col justify-between"
    >
      <div>
        <div
          class="flex items-center justify-between border-b border-white/10 pb-4 mb-6"
        >
          <h2 class="text-sm font-black uppercase tracking-wider text-white">
            Selected Seats
          </h2>
          <button
            class="btn btn-ghost btn-xs bg-black/20 hover:bg-signal hover:text-white rounded-full h-8 w-8 p-0 flex items-center justify-center transition-all"
            onclick={() =>
              seatState.load(data.snapshot.seats, data.snapshot.server_time_ms)}
          >
            <RotateCcw size={15} />
          </button>
        </div>

        <div class="min-h-[144px] space-y-3">
          {#if seatState.selectedSeats.length}
            <div class="grid grid-cols-2 gap-3">
              {#each seatState.selectedSeats as seat}
                <div
                  class="flex items-center justify-center rounded-lg border border-white/5 bg-black/40 p-3 font-mono text-sm shadow-sm"
                  in:slide
                >
                  <span class="text-white font-bold">{seat.seat_id}</span>
                </div>
              {/each}
            </div>
          {:else}
            <div
              class="h-full flex items-center justify-center border-2 border-dashed border-white/5 rounded-xl p-4"
            >
              <p class="text-sm text-inkMuted text-center">
                Choose available seats<br />from the map.
              </p>
            </div>
          {/if}
        </div>
      </div>

      <div class="mt-6 border-t border-white/10 pt-6">
        <div class="flex justify-between items-center font-mono">
          <span class="text-inkMuted uppercase tracking-widest text-xs"
            >Total Tickets</span
          >
          <strong
            class="text-2xl text-ok drop-shadow-[0_0_8px_rgba(16,185,129,0.5)]"
            >{seatState.selectedSeats.length}</strong
          >
        </div>

        {#if error}
          <p
            class="mt-4 rounded-lg border border-urgency/50 bg-urgency/10 p-3 text-xs font-medium text-urgency leading-tight"
          >
            {error}
          </p>
        {/if}

        <PrimaryButton
          disabled={!seatState.selectedSeats.length || reserving}
          onclick={reserve}
        >
          <TicketCheck size={18} />
          {reserving ? reservingStatus : 'Confirm Reservation'}
        </PrimaryButton>
      </div>
    </aside>
  </main>
{/if}
