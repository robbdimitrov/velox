<script lang="ts">
  import { CalendarDays, MapPin, SlidersHorizontal } from 'lucide-svelte';
  import EventCard from '$lib/components/EventCard.svelte';
  import LiveTicker from '$lib/components/LiveTicker.svelte';

  let { data } = $props();
  let activeCategory = $state('Concerts');
  const categories = ['Concerts', 'Sports', 'Theatre', 'Festivals'];
</script>

<main class="mx-auto grid max-w-7xl gap-4 px-4 py-5 lg:grid-cols-[260px_1fr_340px]">
  <aside class="thin-panel h-max p-4">
    <div class="flex items-center justify-between border-b border-line pb-3">
      <h2 class="text-sm font-black uppercase">Filters</h2>
      <SlidersHorizontal size={17} />
    </div>
    <div class="mt-4 space-y-4">
      <label class="form-control">
        <span class="label-text text-ink/70">Event type</span>
        <select class="select select-bordered select-sm border-line bg-carbon">
          <option>All live events</option>
          <option>Concerts</option>
          <option>Sports</option>
          <option>Theatre</option>
        </select>
      </label>
      <label class="form-control">
        <span class="label-text text-ink/70">Date window</span>
        <input class="input input-bordered input-sm border-line bg-carbon" type="date" />
      </label>
      <label class="form-control">
        <span class="label-text text-ink/70">City</span>
        <div class="flex items-center gap-2 border border-line bg-carbon px-2 py-1">
          <MapPin size={15} />
          <input class="min-w-0 flex-1 bg-transparent text-sm outline-none" value="All cities" />
        </div>
      </label>
      <label class="form-control">
        <span class="label-text text-ink/70">Max price</span>
        <input class="range range-primary range-sm" type="range" min="50" max="500" value="180" />
      </label>
      <label class="flex items-center gap-3 text-sm">
        <input class="checkbox checkbox-primary checkbox-sm" type="checkbox" checked />
        Show available inventory only
      </label>
    </div>
  </aside>

  <section class="min-w-0">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      {#each categories as category}
        <button
          class={`btn btn-sm ${activeCategory === category ? 'btn-primary' : 'border-line bg-panel text-ink'}`}
          onclick={() => (activeCategory = category)}
        >
          {category}
        </button>
      {/each}
    </div>

    <LiveTicker url={data.tickerURL} />

    <div class="mt-4 thin-panel p-4">
      <div class="flex items-end justify-between border-b border-line pb-3">
        <div>
          <h1 class="text-2xl font-black uppercase">Trending Demand</h1>
          <p class="text-sm text-ink/60">Read-model discovery, cached for flash sale pressure.</p>
        </div>
        <span class="font-mono text-xs text-ink/50">{data.discovery.meta.projection_lag_ms}ms lag</span>
      </div>
      <div>
        {#each data.discovery.events as event}
          <EventCard {event} />
        {/each}
      </div>
    </div>
  </section>

  <aside class="thin-panel h-max p-4">
    <div class="flex items-center gap-2 border-b border-line pb-3">
      <CalendarDays class="text-signal" size={18} />
      <h2 class="text-sm font-black uppercase">Featured Drops</h2>
    </div>
    <div class="mt-3 grid gap-3">
      {#each data.discovery.featured as event}
        <a class="group block border border-line bg-carbon" href={`/events/${event.id}`}>
          <img class="h-32 w-full object-cover" src={event.image_url} alt="" />
          <div class="p-3">
            <p class="font-black uppercase group-hover:text-signal">{event.title}</p>
            <p class="text-sm text-ink/60">{event.venue}</p>
          </div>
        </a>
      {/each}
    </div>
  </aside>
</main>
