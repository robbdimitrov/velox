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
    <h1 class="text-3xl font-black tracking-tight uppercase">Your Venues</h1>
    <p class="text-inkMuted mt-1 text-sm">
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
  <div class="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
    {#each data.venues as venue}
      <Panel padding="lg" overflowHidden>
        <div
          class="from-signal/5 absolute inset-0 bg-gradient-to-br to-transparent opacity-0 transition-opacity group-hover:opacity-100"
        ></div>
        <div class="relative z-10">
          <div class="mb-4 flex items-start justify-between">
            <div
              class="bg-signal/20 text-signal flex h-10 w-10 items-center justify-center rounded shadow-inner"
            >
              <MapPin size={20} />
            </div>
            <span class="badge badge-sm badge-outline border-line"
              >{venue.capacity} capacity</span
            >
          </div>
          <h3 class="mb-1 text-lg leading-tight font-bold">{venue.name}</h3>
          <p class="text-inkMuted mb-4 flex items-center gap-1 text-sm">
            {venue.city}
          </p>
          <div class="border-line flex justify-end border-t pt-4">
            <button
              class="btn btn-xs btn-ghost text-signal hover:bg-signal/10 rounded"
            >
              Edit <ExternalLink size={14} />
            </button>
          </div>
        </div>
      </Panel>
    {/each}
  </div>
{/if}
