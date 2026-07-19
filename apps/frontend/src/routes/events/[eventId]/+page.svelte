<script lang="ts">
  import { goto } from '$app/navigation';
  import { createIdempotencyKey, createGatewayClient } from '$lib/api/client';
  import type { SeatDelta } from '$lib/api/types';
  import AnnouncementCard from '$lib/components/AnnouncementCard.svelte';
  import SeatCanvas from '$lib/components/SeatCanvas.svelte';
  import VirtualWaitingRoom from '$lib/components/VirtualWaitingRoom.svelte';
  import { checkoutState } from '$lib/state/checkout-state.svelte';
  import { SeatSelectionState } from '$lib/state/seat-state.svelte';
  import {
    Accessibility,
    Megaphone,
    Minus,
    OctagonAlert,
    Plus,
    RotateCcw,
    TicketCheck,
    Layers
  } from '@lucide/svelte';
  import Panel from '$lib/components/Panel.svelte';
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
  let isCancelled = $derived(data.event?.status === 'CANCELLED');

  const ANNOUNCEMENT_PREVIEW_COUNT = 5;
  let showAllAnnouncements = $state(false);
  let visibleAnnouncements = $derived(
    showAllAnnouncements
      ? data.announcements
      : data.announcements.slice(0, ANNOUNCEMENT_PREVIEW_COUNT)
  );
  let hiddenAnnouncementCount = $derived(
    data.announcements.length - ANNOUNCEMENT_PREVIEW_COUNT
  );

  $effect(() => {
    if (data.isRateLimited) return;
    sectionID = data.snapshot.section_id;
    seatState.load(data.snapshot.seats, data.snapshot.server_time_ms);
    eventLog = ['Snapshot loaded from projection read model'];
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
    if (!seatState.selectedSeats.length || isCancelled) return;
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
  <main class="grid w-full gap-6 lg:grid-cols-[260px_1fr_320px]">
    {#if isCancelled}
      <div
        class="lg:col-span-3 flex items-center gap-3 rounded border border-urgency/50 bg-urgency/10 p-4 text-urgency"
      >
        <OctagonAlert size={20} />
        <p class="text-sm font-bold uppercase tracking-wide">
          This event has been cancelled by the organizer. Seat reservation is
          disabled.
        </p>
      </div>
    {/if}

    <Panel padding="lg" sticky hMax>
      <div class="mb-6 flex items-center gap-3 border-b border-line pb-4">
        <div class="rounded bg-signal p-2 shadow-md shadow-signal/20">
          <Layers class="text-carbon" size={18} />
        </div>
        <h2 class="text-sm font-black uppercase tracking-wider text-ink">
          Section Tools
        </h2>
      </div>

      <label class="form-control mb-6">
        <span class="label-text text-inkMuted font-medium mb-1.5">Section</span>
        <select
          bind:value={sectionID}
          onchange={() => goto(`?section_id=${sectionID}`)}
          class="select select-bordered select-sm w-full rounded-sm border-line bg-carbon/60 text-ink focus:border-signal focus:outline-none focus:ring-1 focus:ring-signal/50"
        >
          <option>A</option>
          <option>B</option>
          <option>C</option>
        </select>
      </label>

      <div class="mb-6 grid grid-cols-2 gap-3">
        <button
          class="btn btn-sm rounded-sm border-line bg-panelSoft text-ink shadow-inner hover:bg-panel"
          onclick={() => (zoomLevel = Math.min(3, zoomLevel * 1.2))}
          ><Plus size={16} /> Zoom</button
        >
        <button
          class="btn btn-sm rounded-sm border-line bg-panelSoft text-ink shadow-inner hover:bg-panel"
          onclick={() => (zoomLevel = Math.max(0.5, zoomLevel / 1.2))}
          ><Minus size={16} /> Zoom</button
        >
      </div>

      <label
        class="group mb-6 flex cursor-pointer items-center gap-3 rounded-sm border border-line bg-panelSoft/60 p-3 text-sm transition-colors hover:text-ink"
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
        class="space-y-3 border-t border-line pt-6 text-xs font-bold uppercase tracking-wider text-inkMuted"
      >
        <p class="flex items-center gap-3">
          <span class="inline-block h-3 w-3 rounded-full bg-inkMuted"></span> Available
        </p>
        <p class="flex items-center gap-3">
          <span
            class="inline-block h-3 w-3 rounded-full bg-signal shadow-[0_0_8px_rgba(250,204,21,0.8)]"
          ></span> Selected
        </p>
        <p class="flex items-center gap-3">
          <span
            class="inline-block h-3 w-3 rounded-full bg-urgency shadow-[0_0_8px_rgba(239,68,68,0.8)]"
          ></span> Held
        </p>
        <p class="flex items-center gap-3">
          <span
            class="inline-block h-3 w-3 rounded-full border border-white/20 bg-panel"
          ></span> Sold
        </p>
      </div>
    </Panel>

    <section class="min-w-0 flex flex-col gap-4">
      <Panel padding="lg">
        <div
          class="flex flex-col justify-between gap-4 sm:flex-row sm:items-end"
        >
          <div>
            <h1
              class="text-3xl font-black uppercase tracking-tight text-ink drop-shadow-md"
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
            class="flex flex-col items-end rounded-sm border border-line bg-panelSoft/70 px-3 py-1.5"
          >
            <p
              class="font-mono tabular-nums text-xs uppercase tracking-widest text-ink/60"
            >
              Snapshot <span class="text-ink"
                >{data.snapshot.snapshot_age_ms}ms</span
              >
            </p>
            <p
              class="font-mono tabular-nums mt-0.5 text-xs uppercase tracking-widest text-ink/60"
            >
              Lag <span class="text-ok"
                >{data.snapshot.projection_lag_ms}ms</span
              >
            </p>
          </div>
        </div>
      </Panel>

      <SeatCanvas
        seats={seatState.seats}
        selectedSeatIDs={seatState.selectedSeatIDs}
        onToggle={(seat) => seatState.toggleSeat(seat)}
        {sectionID}
        {zoomLevel}
        {accessibleOnly}
      />

      <Panel padding="sm" overflowHidden>
        <div class="h-24">
          <p
            class="mb-3 border-b border-line pb-2 text-xs font-black uppercase tracking-widest text-inkMuted"
          >
            Live inventory log
          </p>
          <div class="space-y-1.5">
            {#each eventLog as item, index (item)}
              <p
                class="font-mono tabular-nums truncate text-xs text-ink/80 transition-colors hover:text-ink"
                in:slide={{ duration: 200 }}
              >
                <span class="text-signal mr-2">›</span>{item}
              </p>
            {/each}
          </div>
        </div>
      </Panel>

      <Panel padding="sm">
        <p
          class="mb-3 flex items-center gap-2 border-b border-line pb-2 text-xs font-black uppercase tracking-widest text-inkMuted"
        >
          <Megaphone size={14} /> Event Updates
        </p>
        {#if data.announcements.length}
          <div class="space-y-3">
            {#each visibleAnnouncements as announcement (announcement.id)}
              <AnnouncementCard {announcement} />
            {/each}
          </div>
          {#if !showAllAnnouncements && hiddenAnnouncementCount > 0}
            <button
              class="btn btn-sm btn-block mt-3 rounded-sm border-line bg-panelSoft text-ink shadow-inner hover:bg-panel"
              onclick={() => (showAllAnnouncements = true)}
            >
              Show {hiddenAnnouncementCount} more update{hiddenAnnouncementCount ===
              1
                ? ''
                : 's'}
            </button>
          {/if}
        {:else}
          <p class="text-xs text-inkMuted p-2">No updates yet.</p>
        {/if}
      </Panel>
    </section>

    <Panel padding="lg" sticky hMax flexColumn>
      <div>
        <div
          class="mb-6 flex items-center justify-between border-b border-line pb-4"
        >
          <h2 class="text-sm font-black uppercase tracking-wider text-ink">
            Selected Seats
          </h2>
          <button
            class="btn btn-ghost btn-xs flex h-8 w-8 items-center justify-center rounded-sm bg-panelSoft p-0 transition-all hover:bg-signal hover:text-carbon"
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
                  class="flex items-center justify-center rounded-sm border border-line bg-panelSoft/70 p-3 font-mono text-sm tabular-nums shadow-sm"
                  in:slide
                >
                  <span class="font-bold text-ink">{seat.seat_id}</span>
                </div>
              {/each}
            </div>
          {:else}
            <div
              class="flex h-full items-center justify-center rounded-sm border-2 border-dashed border-line p-4"
            >
              <p class="text-sm text-inkMuted text-center">
                Choose available seats<br />from the map.
              </p>
            </div>
          {/if}
        </div>
      </div>

      <div class="mt-6 border-t border-line pt-6">
        <div class="font-mono tabular-nums flex items-center justify-between">
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
            class="mt-4 rounded border border-urgency/50 bg-urgency/10 p-3 text-xs font-medium text-urgency leading-tight"
          >
            {error}
          </p>
        {/if}

        <PrimaryButton
          disabled={!seatState.selectedSeats.length || reserving || isCancelled}
          onclick={reserve}
        >
          <TicketCheck size={18} />
          {reserving ? reservingStatus : 'Confirm Reservation'}
        </PrimaryButton>
      </div>
    </Panel>
  </main>
{/if}
