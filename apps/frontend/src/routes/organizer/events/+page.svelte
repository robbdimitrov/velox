<script lang="ts">
  import { Calendar, Plus, ExternalLink } from '@lucide/svelte';
  import ActionLink from '$lib/components/ActionLink.svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import { pageTitle } from '$lib/pageTitle';
  import Panel from '$lib/components/Panel.svelte';

  let { data } = $props();
</script>

<svelte:head>
  <title>{pageTitle('Events')}</title>
</svelte:head>

<div class="mb-8 flex items-end justify-between">
  <div>
    <h1 class="text-3xl font-black tracking-tight uppercase">Your Events</h1>
    <p class="text-inkMuted mt-1 text-sm">
      Manage your upcoming and past events.
    </p>
  </div>
  <ActionLink href="/organizer/events/new">
    <Plus size={16} /> Create Event
  </ActionLink>
</div>

{#if data.events.length === 0}
  <EmptyState
    icon={Calendar}
    title="No Events Found"
    description="You haven't created any events yet. Create an event to start accepting reservations."
  >
    <ActionLink href="/organizer/events/new">
      <Plus size={16} /> Create Your First Event
    </ActionLink>
  </EmptyState>
{:else}
  <div class="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
    {#each data.events as event}
      <Panel padding="lg" overflowHidden flexColumn>
        <div
          class="from-signal/5 absolute inset-0 bg-gradient-to-br to-transparent opacity-0 transition-opacity group-hover:opacity-100"
        ></div>
        <div class="relative z-10 flex flex-1 flex-col">
          <div class="mb-4 flex items-start justify-between">
            <div
              class="bg-signal/20 text-signal flex h-10 w-10 items-center justify-center rounded shadow-inner"
            >
              <Calendar size={20} />
            </div>
            <span class="badge badge-sm badge-outline border-line">
              {new Date(event.startDate).toLocaleDateString()}
            </span>
          </div>
          <h3 class="mb-1 text-lg leading-tight font-bold">{event.name}</h3>
          <p class="text-inkMuted mb-4 line-clamp-2 flex-1 text-sm">
            {event.description || 'No description provided.'}
          </p>
          <div
            class="border-line mt-auto flex items-center justify-between border-t pt-4"
          >
            <div class="text-inkMuted text-sm">
              {event.status === 'published' ? 'Published' : 'Draft'}
            </div>
            <a
              href={`/organizer/events/${event.id}/dashboard`}
              class="btn btn-xs btn-ghost text-signal hover:bg-signal/10 rounded"
            >
              Manage <ExternalLink size={14} />
            </a>
          </div>
        </div>
      </Panel>
    {/each}
  </div>
{/if}
