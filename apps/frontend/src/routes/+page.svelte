<script lang="ts">
  import {
    CalendarDays,
    MapPin,
    SlidersHorizontal,
    Sparkles
  } from '@lucide/svelte';
  import EventCard from '$lib/components/EventCard.svelte';
  import LiveTicker from '$lib/components/LiveTicker.svelte';
  import { replaceState } from '$app/navigation';
  import { page } from '$app/state';
  import {
    filterState,
    hydrateFilterStateFromURL,
    filterStateToURLParams
  } from '$lib/state/filter-state.svelte';

  let { data } = $props();
  const categories = [
    'All live events',
    'Concerts',
    'Sports',
    'Theatre',
    'Festivals'
  ];

  hydrateFilterStateFromURL(page.url.searchParams);

  const FILTER_SYNC_DEBOUNCE_MS = 300;
  let filterSyncHandle: ReturnType<typeof setTimeout> | undefined;
  $effect(() => {
    const params = filterStateToURLParams();
    clearTimeout(filterSyncHandle);
    filterSyncHandle = setTimeout(() => {
      const search = params.toString();
      const target = search
        ? `${page.url.pathname}?${search}`
        : page.url.pathname;
      replaceState(target, page.state);
    }, FILTER_SYNC_DEBOUNCE_MS);
    return () => clearTimeout(filterSyncHandle);
  });

  let filteredEvents = $derived(
    data.discovery.events.filter((event) => {
      const q = filterState.query.toLowerCase();
      if (
        q &&
        !event.title.toLowerCase().includes(q) &&
        !event.venue.toLowerCase().includes(q) &&
        !event.city.toLowerCase().includes(q)
      ) {
        return false;
      }
      if (filterState.city !== 'All cities' && filterState.city !== '') {
        if (event.city.toLowerCase() !== filterState.city.toLowerCase())
          return false;
      }
      if (filterState.eventType !== 'All live events') {
        const cat = filterState.eventType.toLowerCase();
        if (!event.title.toLowerCase().includes(cat)) {
          return false;
        }
      }
      if (filterState.availableOnly && event.remaining_bucket === 'SOLD_OUT') {
        return false;
      }
      return true;
    })
  );
</script>

<main class="grid gap-6 px-4 lg:grid-cols-[280px_1fr_340px]">
  <aside class="glass-panel h-max p-6 sticky top-28">
    <div
      class="flex items-center justify-between border-b border-white/10 pb-4 mb-6"
    >
      <h2
        class="text-sm font-black uppercase tracking-wider text-ink/80 flex items-center gap-2"
      >
        <SlidersHorizontal size={17} class="text-signal" /> Filters
      </h2>
    </div>
    <div class="space-y-6">
      <label class="form-control">
        <span class="label-text text-inkMuted font-medium mb-1">Event type</span
        >
        <select
          bind:value={filterState.eventType}
          class="select select-bordered select-sm border-white/10 bg-black/40 rounded focus:border-signal"
        >
          {#each categories as category}
            <option>{category}</option>
          {/each}
        </select>
      </label>
      <label class="form-control">
        <span class="label-text text-inkMuted font-medium mb-1"
          >Date window</span
        >
        <input
          class="input input-bordered input-sm border-white/10 bg-black/40 rounded focus:border-signal cursor-pointer"
          type="date"
        />
      </label>
      <label class="form-control">
        <span class="label-text text-inkMuted font-medium mb-1">City</span>
        <div
          class="flex items-center gap-2 border border-white/10 bg-black/40 px-3 py-1.5 rounded focus-within:border-signal transition-colors"
        >
          <MapPin size={15} class="text-signal" />
          <input
            bind:value={filterState.city}
            class="min-w-0 flex-1 bg-transparent text-sm outline-none"
            placeholder="All cities"
          />
        </div>
      </label>

      <label
        class="flex items-center gap-3 text-sm mt-4 cursor-pointer hover:text-white transition-colors group"
      >
        <input
          bind:checked={filterState.availableOnly}
          class="checkbox checkbox-primary checkbox-sm rounded"
          type="checkbox"
        />
        <span class="group-hover:text-signal transition-colors"
          >Show available inventory only</span
        >
      </label>
    </div>
  </aside>

  <section class="min-w-0 flex flex-col gap-6">
    <div class="flex flex-wrap items-center gap-3">
      {#each categories as category}
        <button
          class={`btn btn-sm rounded-full px-6 border-0 shadow-md transition-all duration-300 ${filterState.eventType === category ? 'bg-gradient-to-r from-signal to-primary text-white shadow-glow hover:scale-105' : 'bg-black/40 text-inkMuted hover:bg-black/60 hover:text-white'}`}
          onclick={() => (filterState.eventType = category)}
        >
          {category}
        </button>
      {/each}
    </div>

    <div
      class="rounded overflow-hidden shadow-lg border border-white/5 bg-black/40"
    >
      <LiveTicker url={data.tickerURL} />
    </div>

    <div class="glass-panel p-6">
      <div
        class="flex flex-col md:flex-row md:items-end justify-between border-b border-white/10 pb-4 mb-4 gap-4"
      >
        <div>
          <h1
            class="text-3xl font-black uppercase tracking-tight flex items-center gap-3 text-transparent bg-clip-text bg-gradient-to-r from-white to-inkMuted"
          >
            <Sparkles class="text-accent" size={28} /> Trending Demand
          </h1>
          <p class="text-sm text-inkMuted mt-1">
            Real-time projection discovery with flash sale pressure caching.
          </p>
        </div>
        <div
          class="flex items-center gap-2 bg-black/40 px-3 py-1 rounded-full border border-white/5"
        >
          <div class="w-2 h-2 rounded-full bg-ok animate-pulse"></div>
          <span class="font-mono text-xs text-ok"
            >{data.discovery.meta.projection_lag_ms}ms lag</span
          >
        </div>
      </div>
      <div class="flex flex-col gap-4 mt-6">
        {#each filteredEvents as event}
          <EventCard {event} />
        {:else}
          <div class="text-center py-10 text-inkMuted">
            <p>No events found matching your criteria.</p>
          </div>
        {/each}
      </div>
    </div>
  </section>

  <aside class="glass-panel h-max p-6 sticky top-28">
    <div class="flex items-center gap-3 border-b border-white/10 pb-4 mb-6">
      <div
        class="p-2 bg-gradient-to-br from-accent to-orange-500 rounded shadow-md"
      >
        <CalendarDays class="text-white" size={18} />
      </div>
      <h2 class="text-sm font-black uppercase tracking-wider text-ink/80">
        Featured Drops
      </h2>
    </div>
    <div class="grid gap-5">
      {#each data.discovery.featured as event}
        <a
          class="group relative block overflow-hidden rounded border border-white/10 bg-black/50 shadow-lg hover:border-signal/50 hover:shadow-glow transition-all duration-300"
          href={`/events/${event.id}`}
        >
          <div
            class="absolute inset-0 bg-gradient-to-t from-black/90 via-black/30 to-transparent z-10"
          ></div>
          <img
            class="h-44 w-full object-cover group-hover:scale-110 transition-transform duration-700 ease-out"
            src={event.image_url}
            alt=""
          />
          <div class="absolute bottom-0 left-0 p-5 z-20 w-full">
            <p
              class="font-black uppercase tracking-wide text-white group-hover:text-signal transition-colors text-lg drop-shadow-md truncate"
            >
              {event.title}
            </p>
            <p
              class="text-xs text-inkMuted font-medium mt-1 flex items-center gap-1"
            >
              <MapPin size={12} />
              {event.venue}
            </p>
          </div>
        </a>
      {/each}
    </div>
  </aside>
</main>
