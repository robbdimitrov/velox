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
  <button
    class="btn btn-sm border-none bg-gradient-to-r from-info to-accent text-white shadow-lg shadow-info/20 hover:shadow-info/40 rounded-lg"
  >
    <Plus size={16} /> Add Venue
  </button>
</div>

{#if data.venues.length === 0}
  <div
    class="glass-panel p-12 rounded-3xl flex flex-col items-center justify-center text-center shadow-glow"
  >
    <div
      class="w-16 h-16 rounded-full bg-info/10 flex items-center justify-center mb-4 text-info shadow-inner"
    >
      <MapPin size={32} />
    </div>
    <h3 class="text-xl font-bold mb-2">No Venues Found</h3>
    <p class="text-inkMuted max-w-md mb-6">
      You haven't added any venues yet. Venues are required before you can
      create an event.
    </p>
    <button
      class="btn btn-sm border-none bg-info text-white hover:bg-info/90 shadow-lg shadow-info/20 rounded-lg"
    >
      <Plus size={16} /> Create Your First Venue
    </button>
  </div>
{:else}
  <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
    {#each data.venues as venue}
      <div
        class="glass-panel p-6 rounded-2xl shadow-glow relative overflow-hidden group hover:-translate-y-1 transition-all duration-300"
      >
        <div
          class="absolute inset-0 bg-gradient-to-br from-info/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity"
        ></div>
        <div class="relative z-10">
          <div class="flex justify-between items-start mb-4">
            <div
              class="w-10 h-10 rounded-xl bg-info/20 flex items-center justify-center text-info shadow-inner"
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
              class="btn btn-xs btn-ghost text-info hover:bg-info/10 rounded"
            >
              Edit <ExternalLink size={14} />
            </button>
          </div>
        </div>
      </div>
    {/each}
  </div>
{/if}
