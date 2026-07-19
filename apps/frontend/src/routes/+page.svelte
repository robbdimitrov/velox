<script lang="ts">
  import {
    Building2,
    CalendarDays,
    ChevronRight,
    Clock,
    Gauge,
    MapPin,
    SlidersHorizontal,
    Sparkles,
    Ticket,
    TrendingUp
  } from '@lucide/svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import type { EventSummary } from '$lib/api/types';
  import EventCard from '$lib/components/EventCard.svelte';
  import LiveTicker from '$lib/components/LiveTicker.svelte';
  import {
    filterState,
    filterStateToURLParams,
    hydrateFilterStateFromURL
  } from '$lib/state/filter-state.svelte';

  let { data } = $props();

  type VenueSummary = {
    name: string;
    city: string;
    imageURL: string;
    eventCount: number;
    topDemandScore: number;
    nextSaleStartsAt: string;
    availableEventCount: number;
  };

  const allEventsCategory = 'All live events';
  const standardCategories = ['Concerts', 'Sports', 'Theatre', 'Festivals'];

  const dateWindows = ['Any date', 'Today', 'This week', 'This month'];
  const MAX_QUERY_LENGTH = 120;
  let hydratedSearch = page.url.search;
  hydrateFilterStateFromURL(page.url.searchParams);

  const categoryOptions = $derived(
    uniqueOptions([
      allEventsCategory,
      filterState.eventType,
      ...standardCategories.filter((category) =>
        data.discovery.events.some(
          (event: EventSummary) => event.category === category
        )
      ),
      ...data.discovery.events.map((event: EventSummary) => event.category)
    ])
  );

  const cityOptions = $derived(
    uniqueOptions([
      'All cities',
      filterState.city,
      ...data.discovery.events.map((event: EventSummary) => event.city)
    ])
  );

  const FILTER_SYNC_DEBOUNCE_MS = 300;
  let filterSyncHandle: ReturnType<typeof setTimeout> | undefined;

  $effect(() => {
    if (page.url.search !== hydratedSearch) {
      hydratedSearch = page.url.search;
      hydrateFilterStateFromURL(page.url.searchParams);
    }
  });

  $effect(() => {
    const params = filterStateToURLParams();
    clearTimeout(filterSyncHandle);
    filterSyncHandle = setTimeout(() => {
      const search = params.toString();
      const target = search
        ? `${page.url.pathname}?${search}`
        : page.url.pathname;
      const current = `${page.url.pathname}${page.url.search}`;
      if (target !== current) {
        void goto(target, {
          keepFocus: true,
          noScroll: true,
          replaceState: true
        });
      }
    }, FILTER_SYNC_DEBOUNCE_MS);
    return () => clearTimeout(filterSyncHandle);
  });

  let filteredEvents = $derived(
    data.discovery.events
      .filter((event: EventSummary) => matchesFilters(event))
      .sort((a, b) => b.demand_score - a.demand_score)
  );

  let popularEvents = $derived(
    [...filteredEvents]
      .sort((a, b) => b.demand_score - a.demand_score)
      .slice(0, 3)
  );

  let allVenueSummaries = $derived(buildVenueSummaries(data.discovery.events));

  let venueSummaries = $derived(
    allVenueSummaries.filter((venue) => matchesVenue(venue)).slice(0, 4)
  );

  let totalAvailableEvents = $derived(
    filteredEvents.filter(
      (event: EventSummary) => event.remaining_bucket !== 'SOLD_OUT'
    ).length
  );

  let venueCount = $derived(allVenueSummaries.length);

  let highestDemandScore = $derived(
    Math.max(
      0,
      ...data.discovery.events.map((event: EventSummary) => event.demand_score)
    )
  );

  function uniqueOptions(values: string[]) {
    return Array.from(
      new Set(values.filter((value) => value && value.trim().length > 0))
    );
  }

  function matchesFilters(event: EventSummary) {
    const q = filterState.query.trim().slice(0, MAX_QUERY_LENGTH).toLowerCase();
    const searchable = [
      event.title,
      event.venue,
      event.city,
      event.category
    ].join(' ');

    if (q && !searchable.toLowerCase().includes(q)) return false;
    if (filterState.city !== 'All cities' && event.city !== filterState.city) {
      return false;
    }
    if (
      filterState.eventType !== 'All live events' &&
      event.category !== filterState.eventType
    ) {
      return false;
    }
    if (!matchesDateWindow(event.sale_starts_at, filterState.dateWindow)) {
      return false;
    }
    if (filterState.availableOnly && event.remaining_bucket === 'SOLD_OUT') {
      return false;
    }
    return true;
  }

  function matchesVenue(venue: VenueSummary) {
    const q = filterState.query.trim().slice(0, MAX_QUERY_LENGTH).toLowerCase();
    const searchable = `${venue.name} ${venue.city}`;

    if (q && !searchable.toLowerCase().includes(q)) return false;
    if (filterState.city !== 'All cities' && venue.city !== filterState.city) {
      return false;
    }
    if (filterState.availableOnly && venue.availableEventCount === 0) {
      return false;
    }
    if (!matchesDateWindow(venue.nextSaleStartsAt, filterState.dateWindow)) {
      return false;
    }
    return true;
  }

  function matchesDateWindow(value: string, dateWindow: string) {
    if (dateWindow === 'Any date') return true;

    const startsAt = new Date(value).getTime();
    const now = Date.now();
    const dayMs = 24 * 60 * 60 * 1000;

    if (!Number.isFinite(startsAt) || startsAt < now) return false;
    if (dateWindow === 'Today') {
      return new Date(startsAt).toDateString() === new Date(now).toDateString();
    }
    if (dateWindow === 'This week') return startsAt <= now + 7 * dayMs;
    if (dateWindow === 'This month') return startsAt <= now + 31 * dayMs;
    return true;
  }

  function buildVenueSummaries(events: EventSummary[]) {
    const venueByKey: Record<string, VenueSummary> = {};

    for (const event of events) {
      const key = `${event.venue}\u0000${event.city}`;
      const existing = venueByKey[key];

      if (!existing) {
        venueByKey[key] = {
          name: event.venue,
          city: event.city,
          imageURL: event.image_url,
          eventCount: 1,
          topDemandScore: event.demand_score,
          nextSaleStartsAt: event.sale_starts_at,
          availableEventCount: event.remaining_bucket === 'SOLD_OUT' ? 0 : 1
        };
        continue;
      }

      existing.eventCount += 1;
      existing.topDemandScore = Math.max(
        existing.topDemandScore,
        event.demand_score
      );
      if (
        new Date(event.sale_starts_at).getTime() <
        new Date(existing.nextSaleStartsAt).getTime()
      ) {
        existing.nextSaleStartsAt = event.sale_starts_at;
        existing.imageURL = event.image_url;
      }
      if (event.remaining_bucket !== 'SOLD_OUT') {
        existing.availableEventCount += 1;
      }
    }

    return Object.values(venueByKey).sort(
      (a, b) =>
        b.topDemandScore - a.topDemandScore ||
        a.nextSaleStartsAt.localeCompare(b.nextSaleStartsAt)
    );
  }

  function formatSaleTime(value: string) {
    return new Intl.DateTimeFormat('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    }).format(new Date(value));
  }
</script>

<main class="w-full space-y-6">
  <section
    class="overflow-hidden rounded-sm border border-line bg-panel/90 shadow-xl"
  >
    <div class="grid gap-0 lg:grid-cols-[1.08fr_0.92fr]">
      <div class="p-5 sm:p-7 lg:p-8">
        <p
          class="flex items-center gap-2 text-[0.72rem] font-extrabold uppercase tracking-[0.14em] text-signal"
        >
          <Sparkles size={15} /> Live discovery
        </p>
        <h1
          class="mt-3 max-w-3xl text-4xl font-black uppercase leading-none text-ink sm:text-6xl"
        >
          Inventory before the room moves.
        </h1>
        <p class="mt-5 max-w-2xl text-base font-medium text-inkMuted">
          Track demand, sale timing, and venue pressure from one live read
          model.
        </p>

        <div class="mt-6 flex flex-wrap gap-2">
          {#each categoryOptions as category}
            <button
              class="btn btn-xs rounded-sm border px-3 uppercase hover:border-signal hover:text-ink"
              class:btn-primary={filterState.eventType === category}
              class:text-primary-content={filterState.eventType === category}
              class:border-line={filterState.eventType !== category}
              class:bg-panelSoft={filterState.eventType !== category}
              class:text-inkMuted={filterState.eventType !== category}
              aria-pressed={filterState.eventType === category}
              onclick={() => (filterState.eventType = category)}
            >
              {category}
            </button>
          {/each}
        </div>

        <div class="mt-7 grid grid-cols-3 gap-3">
          <div class="rounded-sm border border-line bg-panelSoft/70 p-4">
            <p class="text-[10px] font-bold uppercase text-inkMuted">Events</p>
            <p class="font-mono tabular-nums mt-2 text-3xl font-black text-ink">
              {data.discovery.events.length}
            </p>
          </div>
          <div class="rounded-sm border border-line bg-panelSoft/70 p-4">
            <p class="text-[10px] font-bold uppercase text-inkMuted">Venues</p>
            <p class="font-mono tabular-nums mt-2 text-3xl font-black text-ink">
              {venueCount}
            </p>
          </div>
          <div class="rounded-sm border border-line bg-panelSoft/70 p-4">
            <p class="text-[10px] font-bold uppercase text-inkMuted">Demand</p>
            <p
              class="font-mono tabular-nums mt-2 text-3xl font-black text-signal"
            >
              {highestDemandScore}
            </p>
          </div>
        </div>
      </div>

      {#if popularEvents[0]}
        {@const leadEvent = popularEvents[0]}
        <a
          class="group relative min-h-[28rem] overflow-hidden border-t border-line bg-carbon lg:border-l lg:border-t-0"
          href={`/events/${leadEvent.id}`}
        >
          <img
            class="absolute inset-0 h-full w-full object-cover opacity-85 transition-transform duration-700 group-hover:scale-105"
            src={leadEvent.image_url}
            alt=""
          />
          <div
            class="absolute inset-0 bg-[linear-gradient(180deg,transparent_0%,rgba(0,0,0,0.18)_38%,rgba(0,0,0,0.82)_100%)]"
          ></div>
          <div class="absolute inset-x-0 bottom-0 p-6">
            <div class="mb-4 flex items-center justify-between gap-3">
              <span
                class="font-mono tabular-nums rounded-sm bg-signal px-3 py-1 text-xs font-black uppercase text-carbon"
              >
                Demand {leadEvent.demand_score}
              </span>
              <span
                class="font-mono tabular-nums rounded-sm border border-white/25 bg-black/45 px-3 py-1 text-xs uppercase text-white"
              >
                {formatSaleTime(leadEvent.sale_starts_at)}
              </span>
            </div>
            <p class="text-3xl font-black uppercase leading-none text-white">
              {leadEvent.title}
            </p>
            <p
              class="mt-3 flex items-center gap-2 text-sm font-semibold text-white/80"
            >
              <MapPin size={15} class="text-signal" />
              {leadEvent.venue}, {leadEvent.city}
            </p>
          </div>
        </a>
      {/if}
    </div>
    <LiveTicker url={data.tickerURL} />
  </section>

  <div class="grid gap-6 xl:grid-cols-[280px_1fr_340px]">
    <aside
      class="h-max rounded-sm border border-line bg-panel/90 p-5 shadow-xl xl:sticky xl:top-28"
    >
      <div
        class="mb-5 flex items-center justify-between border-b border-line pb-4"
      >
        <h2
          class="flex items-center gap-2 text-sm font-black uppercase text-ink"
        >
          <SlidersHorizontal size={17} class="text-signal" /> Filters
        </h2>
        <span class="font-mono tabular-nums text-xs text-inkMuted">
          {filteredEvents.length} hits
        </span>
      </div>

      <div class="space-y-5">
        <label class="form-control">
          <span class="label-text mb-1 font-medium text-inkMuted"
            >Event type</span
          >
          <select
            bind:value={filterState.eventType}
            class="select select-bordered select-sm w-full rounded-sm border-line bg-carbon/60 text-ink focus:border-signal focus:outline-none"
          >
            {#each categoryOptions as category}
              <option>{category}</option>
            {/each}
          </select>
        </label>

        <label class="form-control">
          <span class="label-text mb-1 font-medium text-inkMuted"
            >Date window</span
          >
          <select
            bind:value={filterState.dateWindow}
            class="select select-bordered select-sm w-full rounded-sm border-line bg-carbon/60 text-ink focus:border-signal focus:outline-none"
          >
            {#each dateWindows as dateWindow}
              <option>{dateWindow}</option>
            {/each}
          </select>
        </label>

        <label class="form-control">
          <span class="label-text mb-1 font-medium text-inkMuted">City</span>
          <div
            class="flex w-full items-center gap-2 rounded-sm border border-line bg-carbon/60 px-3 py-1.5 outline-none transition-colors focus-within:border-signal focus-within:ring-1 focus-within:ring-signal/50"
          >
            <MapPin size={15} class="text-signal" />
            <select
              bind:value={filterState.city}
              class="min-w-0 flex-1 bg-transparent text-sm outline-none"
            >
              {#each cityOptions as city}
                <option>{city}</option>
              {/each}
            </select>
          </div>
        </label>

        <label
          class="flex cursor-pointer items-center gap-3 text-sm text-inkMuted transition-colors hover:text-ink"
        >
          <input
            bind:checked={filterState.availableOnly}
            class="checkbox checkbox-primary checkbox-sm rounded-sm"
            type="checkbox"
          />
          <span>Available inventory only</span>
        </label>
      </div>

      <div class="mt-6 grid grid-cols-2 gap-3 border-t border-line pt-5">
        <div class="rounded-sm border border-line bg-panelSoft/70 p-3">
          <p class="text-[10px] font-semibold uppercase text-inkMuted">
            Read lag
          </p>
          <p class="font-mono tabular-nums mt-1 text-lg font-black text-ok">
            {data.discovery.meta.projection_lag_ms}ms
          </p>
        </div>
        <div class="rounded-sm border border-line bg-panelSoft/70 p-3">
          <p class="text-[10px] font-semibold uppercase text-inkMuted">Cache</p>
          <p class="mt-1 truncate text-xs font-black uppercase text-signal">
            {data.discovery.meta.cache_status}
          </p>
        </div>
      </div>
    </aside>

    <section class="flex min-w-0 flex-col gap-6">
      <section class="rounded-sm border border-line bg-panel/90 p-5 shadow-xl">
        <div
          class="mb-5 flex items-end justify-between gap-3 border-b border-line pb-4"
        >
          <div>
            <p
              class="flex items-center gap-2 text-[0.72rem] font-extrabold uppercase tracking-[0.14em] text-signal"
            >
              <TrendingUp size={16} /> Trending
            </p>
            <h2
              class="mt-2 text-xl font-extrabold leading-tight text-ink sm:text-2xl"
            >
              High-demand drops
            </h2>
          </div>
          <span
            class="font-mono tabular-nums rounded-sm border border-line bg-panelSoft/70 px-3 py-1 text-xs text-ok"
          >
            {totalAvailableEvents} available
          </span>
        </div>

        <div class="grid gap-4 md:grid-cols-3 xl:grid-cols-1 2xl:grid-cols-3">
          {#each popularEvents as event}
            <a
              class="group overflow-hidden rounded-sm border border-line bg-panelSoft transition-all duration-300 hover:-translate-y-1 hover:border-signal"
              href={`/events/${event.id}`}
            >
              <div class="relative h-36 overflow-hidden bg-carbon">
                <img
                  class="h-full w-full object-cover transition-transform duration-700 group-hover:scale-105"
                  src={event.image_url}
                  alt=""
                />
                <span
                  class="font-mono tabular-nums absolute right-3 top-3 rounded-sm bg-signal px-2 py-1 text-xs font-black text-carbon"
                >
                  {event.demand_score}
                </span>
              </div>
              <div class="p-4">
                <p
                  class="truncate text-lg font-black uppercase text-ink group-hover:text-signal"
                >
                  {event.title}
                </p>
                <p class="mt-1 flex items-center gap-1 text-sm text-inkMuted">
                  <MapPin size={13} class="text-signal" />
                  {event.venue}
                </p>
                <div class="mt-3 text-xs uppercase text-inkMuted">
                  <span>{formatSaleTime(event.sale_starts_at)}</span>
                </div>
              </div>
            </a>
          {:else}
            <p class="col-span-full py-8 text-center text-inkMuted">
              No popular events match the active filters.
            </p>
          {/each}
        </div>
      </section>

      <section class="rounded-sm border border-line bg-panel/90 p-5 shadow-xl">
        <div
          class="mb-5 flex items-end justify-between gap-3 border-b border-line pb-4"
        >
          <div>
            <p
              class="flex items-center gap-2 text-[0.72rem] font-extrabold uppercase tracking-[0.14em] text-signal"
            >
              <Ticket size={16} /> Inventory
            </p>
            <h2
              class="mt-2 text-xl font-extrabold leading-tight text-ink sm:text-2xl"
            >
              All matching events
            </h2>
          </div>
          <span
            class="font-mono tabular-nums rounded-sm border border-line bg-panelSoft/70 px-3 py-1 text-xs text-inkMuted"
          >
            {filteredEvents.length} results
          </span>
        </div>

        <div class="flex flex-col gap-4">
          {#each filteredEvents as event}
            <EventCard {event} />
          {:else}
            <div class="py-10 text-center text-inkMuted">
              <p>No events found matching your criteria.</p>
            </div>
          {/each}
        </div>
      </section>
    </section>

    <aside class="flex h-max flex-col gap-6 xl:sticky xl:top-28">
      <section class="rounded-sm border border-line bg-panel/90 p-5 shadow-xl">
        <div class="mb-5 flex items-center gap-3 border-b border-line pb-4">
          <div
            class="grid h-9 w-9 place-items-center rounded-sm bg-signal text-carbon"
          >
            <Building2 size={18} />
          </div>
          <h2 class="text-sm font-black uppercase text-ink">Top venues</h2>
        </div>

        <div class="grid gap-4">
          {#each venueSummaries as venue}
            <button
              type="button"
              class="group overflow-hidden rounded-sm border border-line bg-panelSoft text-left transition-colors hover:border-signal"
              onclick={() => (filterState.query = venue.name)}
            >
              <div class="grid grid-cols-[82px_1fr] gap-3 p-3">
                <img
                  class="h-20 w-full rounded-sm object-cover"
                  src={venue.imageURL}
                  alt=""
                />
                <div class="min-w-0">
                  <div class="flex items-start justify-between gap-2">
                    <div class="min-w-0">
                      <p
                        class="truncate font-black uppercase text-ink group-hover:text-signal"
                      >
                        {venue.name}
                      </p>
                      <p
                        class="mt-1 flex items-center gap-1 text-xs text-inkMuted"
                      >
                        <MapPin size={12} />
                        {venue.city}
                      </p>
                    </div>
                    <span
                      class="font-mono tabular-nums text-lg font-black text-signal"
                    >
                      {venue.topDemandScore}
                    </span>
                  </div>
                  <div class="mt-3 text-[11px] uppercase text-inkMuted">
                    <span
                      class="rounded-sm border border-line bg-panel/70 px-2 py-1 text-inkMuted"
                    >
                      {venue.eventCount} events
                    </span>
                  </div>
                </div>
              </div>
            </button>
          {:else}
            <p class="py-8 text-center text-sm text-inkMuted">
              No venues match the active filters.
            </p>
          {/each}
        </div>
      </section>

      <section class="rounded-sm border border-line bg-panel/90 p-5 shadow-xl">
        <div class="mb-5 flex items-center gap-3 border-b border-line pb-4">
          <div
            class="inline-grid h-9 w-9 place-items-center rounded-sm border border-line bg-panelSoft/70 text-signal"
          >
            <CalendarDays size={18} />
          </div>
          <h2 class="text-sm font-black uppercase text-ink">Featured drops</h2>
        </div>

        <div class="grid gap-4">
          {#each data.discovery.featured.slice(0, 3) as event}
            <a
              class="group block rounded-sm border border-line bg-panelSoft p-3 transition-all hover:border-signal"
              href={`/events/${event.id}`}
            >
              <div class="flex items-center gap-3">
                <img
                  class="h-16 w-16 rounded-sm object-cover"
                  src={event.image_url}
                  alt=""
                />
                <div class="min-w-0 flex-1">
                  <p
                    class="truncate font-black uppercase text-ink group-hover:text-signal"
                  >
                    {event.title}
                  </p>
                  <p class="mt-1 flex items-center gap-1 text-xs text-inkMuted">
                    <Clock size={12} />
                    {formatSaleTime(event.sale_starts_at)}
                  </p>
                </div>
                <ChevronRight
                  size={17}
                  class="shrink-0 text-inkMuted group-hover:text-signal"
                />
              </div>
            </a>
          {/each}
        </div>
      </section>

      <section class="rounded-sm border border-line bg-panel/90 p-5 shadow-xl">
        <div class="mb-5 flex items-center gap-3 border-b border-line pb-4">
          <div
            class="inline-grid h-9 w-9 place-items-center rounded-sm border border-line bg-panelSoft/70 text-signal"
          >
            <Gauge size={18} />
          </div>
          <h2 class="text-sm font-black uppercase text-ink">Demand board</h2>
        </div>

        <div class="space-y-3">
          {#each [...data.discovery.events]
            .sort((a, b) => b.demand_score - a.demand_score)
            .slice(0, 5) as event, index}
            <a
              class="grid grid-cols-[28px_1fr_auto] items-center gap-3 rounded-sm border border-line bg-panelSoft px-3 py-2 transition-colors hover:border-signal"
              href={`/events/${event.id}`}
            >
              <span class="font-mono tabular-nums text-xs text-inkMuted">
                {index + 1}
              </span>
              <span class="min-w-0 truncate text-sm font-bold text-ink">
                {event.title}
              </span>
              <span
                class="font-mono tabular-nums text-sm font-black text-signal"
              >
                {event.demand_score}
              </span>
            </a>
          {/each}
        </div>
      </section>
    </aside>
  </div>
</main>
