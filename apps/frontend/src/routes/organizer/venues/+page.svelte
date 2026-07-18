<script lang="ts">
  import { MapPin, Plus, ExternalLink } from '@lucide/svelte';
  let { data } = $props();
</script>

<svelte:head>
  <title>Venues - Velox Organizer</title>
</svelte:head>

<div class="mb-8 flex items-end justify-between">
  <div>
    <h1 class="text-3xl font-black uppercase tracking-tight">Your Venues</h1>
    <p class="text-inkMuted text-sm mt-1">
      Manage physical locations for your events.
    </p>
  </div>
  <a href="/organizer/venues/new" class="btn btn-sm velox-action rounded">
    <Plus size={16} /> Add Venue
  </a>
</div>

{#if data.venues.length === 0}
  <div
    class="glass-panel p-12 rounded flex flex-col items-center justify-center text-center shadow-glow"
  >
    <div
      class="mb-4 flex h-16 w-16 items-center justify-center rounded bg-signal/10 text-signal shadow-inner"
    >
      <MapPin size={32} />
    </div>
    <h3 class="text-xl font-bold mb-2">No Venues Found</h3>
    <p class="text-inkMuted max-w-md mb-6">
      You haven't added any venues yet. Venues are required before you can
      create an event.
    </p>
    <a href="/organizer/venues/new" class="btn btn-sm velox-action rounded">
      <Plus size={16} /> Create Your First Venue
    </a>
  </div>
{:else}
  <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
    {#each data.venues as venue}
      <div
        class="glass-panel p-6 rounded shadow-glow relative overflow-hidden group hover:-translate-y-1 transition-all duration-300"
      >
        <div
          class="absolute inset-0 bg-gradient-to-br from-signal/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity"
        ></div>
        <div class="relative z-10">
          <div class="flex justify-between items-start mb-4">
            <div
              class="flex h-10 w-10 items-center justify-center rounded bg-signal/20 text-signal shadow-inner"
            >
              <MapPin size={20} />
            </div>
            <span class="badge badge-sm badge-outline border-white/10"
              >{venue.capacity} capacity</span
            >
          </div>
          <h3 class="font-bold text-lg leading-tight mb-1">{venue.name}</h3>
          <p class="text-inkMuted text-sm flex items-center gap-1 mb-4">
            {venue.city}, {venue.country}
          </p>
          <div class="pt-4 border-t border-white/10 flex justify-end">
            <button
              class="btn btn-xs btn-ghost rounded text-signal hover:bg-signal/10"
            >
              Edit <ExternalLink size={14} />
            </button>
          </div>
        </div>
      </div>
    {/each}
  </div>
{/if}
