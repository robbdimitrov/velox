<script lang="ts">
  import { Calendar, Plus, ExternalLink } from '@lucide/svelte';
  let { data } = $props();
</script>

<svelte:head>
  <title>Events - Velox Organizer</title>
</svelte:head>

<div class="mb-8 flex items-end justify-between">
  <div>
    <h1 class="text-3xl font-black uppercase tracking-tight">Your Events</h1>
    <p class="text-inkMuted text-sm mt-1">
      Manage your upcoming and past events.
    </p>
  </div>
  <a href="/organizer/events/new" class="btn btn-sm velox-action rounded">
    <Plus size={16} /> Create Event
  </a>
</div>

{#if data.events.length === 0}
  <div
    class="glass-panel p-12 rounded flex flex-col items-center justify-center text-center shadow-glow"
  >
    <div
      class="mb-4 flex h-16 w-16 items-center justify-center rounded bg-signal/10 text-signal shadow-inner"
    >
      <Calendar size={32} />
    </div>
    <h3 class="text-xl font-bold mb-2">No Events Found</h3>
    <p class="text-inkMuted max-w-md mb-6">
      You haven't created any events yet. Create an event to start selling
      tickets!
    </p>
    <a href="/organizer/events/new" class="btn btn-sm velox-action rounded">
      <Plus size={16} /> Create Your First Event
    </a>
  </div>
{:else}
  <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
    {#each data.events as event}
      <div
        class="glass-panel p-6 rounded shadow-glow relative overflow-hidden group hover:-translate-y-1 transition-all duration-300 flex flex-col"
      >
        <div
          class="absolute inset-0 bg-gradient-to-br from-signal/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity"
        ></div>
        <div class="relative z-10 flex-1 flex flex-col">
          <div class="flex justify-between items-start mb-4">
            <div
              class="flex h-10 w-10 items-center justify-center rounded bg-signal/20 text-signal shadow-inner"
            >
              <Calendar size={20} />
            </div>
            <span class="badge badge-sm badge-outline border-white/10">
              {new Date(event.startDate).toLocaleDateString()}
            </span>
          </div>
          <h3 class="font-bold text-lg leading-tight mb-1">{event.name}</h3>
          <p class="text-inkMuted text-sm mb-4 line-clamp-2 flex-1">
            {event.description || 'No description provided.'}
          </p>
          <div
            class="pt-4 border-t border-white/10 flex justify-between items-center mt-auto"
          >
            <div class="text-sm text-inkMuted">
              {event.status === 'published' ? 'Published' : 'Draft'}
            </div>
            <a
              href={`/organizer/events/${event.id}/dashboard`}
              class="btn btn-xs btn-ghost rounded text-signal hover:bg-signal/10"
            >
              Manage <ExternalLink size={14} />
            </a>
          </div>
        </div>
      </div>
    {/each}
  </div>
{/if}
