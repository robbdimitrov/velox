<script lang="ts">
  import { goto } from '$app/navigation';
  import { createIdempotencyKey, createGatewayClient } from '$lib/api/client';
  import type { SeatDelta } from '$lib/api/types';
  import AnnouncementCard from '$lib/components/AnnouncementCard.svelte';
  import SeatCanvas from '$lib/components/SeatCanvas.svelte';
  import VirtualWaitingRoom from '$lib/components/VirtualWaitingRoom.svelte';
  import { reservationState } from '$lib/state/reservation-state.svelte';
  import { SeatSelectionState } from '$lib/state/seat-state.svelte';
  import {
    Accessibility,
    CalendarDays,
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
  let sectionOptions = $derived(
    data.sections?.length
      ? data.sections
      : [{ id: data.snapshot.section_id, name: data.snapshot.section_id }]
  );

  const eventTimeFormatter = new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  });

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
        const delta = JSON.parse(event.data) as unknown;
        if (!isSeatDelta(delta)) {
          return;
        }
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
    reservingStatus = 'Holding seats...';
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

      reservingStatus = 'Preparing review...';
      await new Promise((r) => setTimeout(r, 800));
      reservingStatus = 'Hold ready';
      await new Promise((r) => setTimeout(r, 400));

      reservationState.load(reservation);
      await goto('/reservation');
    } catch (requestError) {
      error =
        'Reservation rejected by gateway. Refresh seat state and try again.';
      reservingStatus = '';
    }
  }

  function formatEventTime(value: string | undefined) {
    if (!value) return 'TBA';
    const timestamp = new Date(value).getTime();
    if (!Number.isFinite(timestamp)) return 'TBA';
    return eventTimeFormatter.format(new Date(timestamp));
  }

  function isSeatDelta(value: unknown): value is SeatDelta {
    if (!value || typeof value !== 'object') return false;
    const candidate = value as Partial<SeatDelta>;
    return (
      typeof candidate.seat_id === 'string' &&
      candidate.seat_id.length > 0 &&
      typeof candidate.status === 'string' &&
      typeof candidate.version === 'number'
    );
  }
</script>

{#if data.isRateLimited}
  <VirtualWaitingRoom />
{:else}
  <main class="grid w-full gap-6 lg:grid-cols-[260px_1fr_320px]">
    {#if isCancelled}
      <div
        class="border-urgency/50 bg-urgency/10 text-urgency flex items-center gap-3 rounded border p-4 lg:col-span-3"
      >
        <OctagonAlert size={20} />
        <p class="text-sm font-bold tracking-wide uppercase">
          This event has been cancelled by the organizer. Seat reservation is
          disabled.
        </p>
      </div>
    {/if}

    {#if data.loadErrors?.length}
      <div
        class="border-urgency/50 bg-urgency/10 text-urgency rounded-sm border p-4 text-sm font-semibold lg:col-span-3"
      >
        {#each data.loadErrors as loadError}
          <p>{loadError}</p>
        {/each}
      </div>
    {/if}

    <Panel padding="lg" sticky hMax>
      <div class="border-line mb-6 flex items-center gap-3 border-b pb-4">
        <div class="bg-signal shadow-signal/20 rounded p-2 shadow-md">
          <Layers class="text-primary-content" size={18} />
        </div>
        <h2 class="text-ink text-sm font-black tracking-wider uppercase">
          Section Tools
        </h2>
      </div>

      <label class="form-control mb-6">
        <span class="label-text text-inkMuted mb-1.5 font-medium">Section</span>
        <select
          bind:value={sectionID}
          onchange={() => goto(`?section_id=${sectionID}`)}
          class="select select-bordered select-sm border-line bg-carbon/60 text-ink focus:border-signal focus:ring-signal/50 w-full rounded-sm focus:ring-1 focus:outline-none"
        >
          {#each sectionOptions as section}
            <option value={section.id}>{section.name}</option>
          {/each}
        </select>
      </label>

      <div class="mb-6 grid grid-cols-2 gap-3">
        <button
          class="btn btn-sm border-line bg-panelSoft text-ink hover:bg-panel rounded-sm shadow-inner"
          onclick={() => (zoomLevel = Math.min(3, zoomLevel * 1.2))}
          ><Plus size={16} /> Zoom</button
        >
        <button
          class="btn btn-sm border-line bg-panelSoft text-ink hover:bg-panel rounded-sm shadow-inner"
          onclick={() => (zoomLevel = Math.max(0.5, zoomLevel / 1.2))}
          ><Minus size={16} /> Zoom</button
        >
      </div>

      <label
        class="group border-line bg-panelSoft/60 hover:text-ink mb-6 flex cursor-pointer items-center gap-3 rounded-sm border p-3 text-sm transition-colors"
      >
        <input
          bind:checked={accessibleOnly}
          class="toggle toggle-primary toggle-sm"
          type="checkbox"
        />
        <span
          class="group-hover:text-signal flex items-center gap-2 transition-colors"
          ><Accessibility size={16} /> Accessible</span
        >
      </label>

      <div
        class="border-line text-inkMuted space-y-3 border-t pt-6 text-xs font-bold tracking-wider uppercase"
      >
        <p class="flex items-center gap-3">
          <span class="bg-inkMuted inline-block h-3 w-3 rounded-full"></span> Available
        </p>
        <p class="flex items-center gap-3">
          <span
            class="bg-signal inline-block h-3 w-3 rounded-full shadow-[0_0_8px_rgba(159,29,47,0.8)]"
          ></span> Selected
        </p>
        <p class="flex items-center gap-3">
          <span
            class="bg-urgency inline-block h-3 w-3 rounded-full shadow-[0_0_8px_rgba(239,68,68,0.8)]"
          ></span> Held
        </p>
        <p class="flex items-center gap-3">
          <span
            class="bg-panel inline-block h-3 w-3 rounded-full border border-white/20"
          ></span> Reserved
        </p>
      </div>
    </Panel>

    <section class="flex min-w-0 flex-col gap-4">
      <Panel padding="lg">
        <div class="grid gap-4 md:grid-cols-[1fr_auto] md:items-end">
          <div class="min-w-0">
            <div class="mb-2 flex flex-wrap items-center gap-2">
              <span
                class="border-line bg-panelSoft text-signal rounded-sm border px-2 py-1 text-xs font-black uppercase"
              >
                {data.event.category}
              </span>
              <span
                class="border-line bg-panelSoft text-inkMuted flex items-center gap-1 rounded-sm border px-2 py-1 text-xs font-semibold uppercase"
              >
                <CalendarDays size={13} />
                Starts {formatEventTime(data.event.starts_at)}
              </span>
            </div>
            <h1
              class="text-ink text-3xl font-black tracking-tight uppercase drop-shadow-md"
            >
              {data.event.title}
            </h1>
            <p
              class="text-signal mt-1 text-sm font-medium tracking-wide uppercase"
            >
              {data.event.venue}
              {#if data.event.city}
                <span class="text-inkMuted mx-2">|</span>{data.event.city}
              {/if}
              <span class="text-inkMuted mx-2">|</span>Section {sectionID}
            </p>
            {#if data.event.description}
              <p class="text-inkMuted mt-3 max-w-2xl text-sm leading-6">
                {data.event.description}
              </p>
            {/if}
          </div>
          <div
            class="border-line bg-panelSoft/70 flex flex-col items-end rounded-sm border px-3 py-1.5"
          >
            <p
              class="text-ink/60 font-mono text-xs tracking-widest uppercase tabular-nums"
            >
              Snapshot <span class="text-ink"
                >{data.snapshot.snapshot_age_ms}ms</span
              >
            </p>
            <p
              class="text-ink/60 mt-0.5 font-mono text-xs tracking-widest uppercase tabular-nums"
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
            class="border-line text-inkMuted mb-3 border-b pb-2 text-xs font-black tracking-widest uppercase"
          >
            Live inventory log
          </p>
          <div class="space-y-1.5">
            {#each eventLog as item, index (item)}
              <p
                class="text-ink/80 hover:text-ink truncate font-mono text-xs tabular-nums transition-colors"
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
          class="border-line text-inkMuted mb-3 flex items-center gap-2 border-b pb-2 text-xs font-black tracking-widest uppercase"
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
              class="btn btn-sm btn-block border-line bg-panelSoft text-ink hover:bg-panel mt-3 rounded-sm shadow-inner"
              onclick={() => (showAllAnnouncements = true)}
            >
              Show {hiddenAnnouncementCount} more update{hiddenAnnouncementCount ===
              1
                ? ''
                : 's'}
            </button>
          {/if}
        {:else}
          <p class="text-inkMuted p-2 text-xs">No updates yet.</p>
        {/if}
      </Panel>
    </section>

    <Panel padding="lg" sticky hMax flexColumn>
      <div>
        <div
          class="border-line mb-6 flex items-center justify-between border-b pb-4"
        >
          <h2 class="text-ink text-sm font-black tracking-wider uppercase">
            Selected Seats
          </h2>
          <button
            class="btn btn-ghost btn-xs bg-panelSoft hover:bg-signal hover:text-primary-content flex h-8 w-8 items-center justify-center rounded-sm p-0 transition-all"
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
                  class="border-line bg-panelSoft/70 flex items-center justify-center rounded-sm border p-3 font-mono text-sm tabular-nums shadow-sm"
                  in:slide
                >
                  <span class="text-ink font-bold">{seat.seat_id}</span>
                </div>
              {/each}
            </div>
          {:else}
            <div
              class="border-line flex h-full items-center justify-center rounded-sm border-2 border-dashed p-4"
            >
              <p class="text-inkMuted text-center text-sm">
                Choose available seats<br />from the map.
              </p>
            </div>
          {/if}
        </div>
      </div>

      <div class="border-line mt-6 border-t pt-6">
        <div class="flex items-center justify-between font-mono tabular-nums">
          <span class="text-inkMuted text-xs tracking-widest uppercase"
            >Reservation tickets</span
          >
          <strong
            class="text-ok text-2xl drop-shadow-[0_0_8px_rgba(16,185,129,0.5)]"
            >{seatState.selectedSeats.length}</strong
          >
        </div>

        {#if error}
          <p
            class="border-urgency/50 bg-urgency/10 text-urgency mt-4 rounded border p-3 text-xs leading-tight font-medium"
          >
            {error}
          </p>
        {/if}

        <PrimaryButton
          disabled={!seatState.selectedSeats.length || reserving || isCancelled}
          onclick={reserve}
        >
          <TicketCheck size={18} />
          {reserving ? reservingStatus : 'Reserve'}
        </PrimaryButton>
      </div>
    </Panel>
  </main>
{/if}
