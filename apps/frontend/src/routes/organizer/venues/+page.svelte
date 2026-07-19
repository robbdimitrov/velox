<script lang="ts">
  import { MapPin, Plus, ExternalLink } from '@lucide/svelte';
  import ActionLink from '$lib/components/ActionLink.svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import Panel from '$lib/components/Panel.svelte';

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
  <ActionLink href="/organizer/venues/new">
    <Plus size={16} /> Add Venue
  </ActionLink>
</div>

{#if data.venues.length === 0}
  <EmptyState
    icon={MapPin}
    title="No Venues Found"
    description="You haven't added any venues yet. Venues are required before you can create an event."
  >
    <ActionLink href="/organizer/venues/new">
      <Plus size={16} /> Create Your First Venue
    </ActionLink>
  </EmptyState>
{:else}
  <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
    {#each data.venues as venue}
      <Panel padding="lg" overflowHidden>
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
            {venue.city}
          </p>
          <div class="pt-4 border-t border-white/10 flex justify-end">
            <button
              class="btn btn-xs btn-ghost rounded text-signal hover:bg-signal/10"
            >
              Edit <ExternalLink size={14} />
            </button>
          </div>
        </div>
      </Panel>
    {/each}
  </div>
{/if}
