<script lang="ts">
  import { Shield, Users } from '@lucide/svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import Panel from '$lib/components/Panel.svelte';

  let { data } = $props();
</script>

<svelte:head>
  <title>Venue Access - Velox Organizer</title>
</svelte:head>

<div class="space-y-8">
  <div class="mb-8 flex justify-between items-end">
    <div>
      <h1 class="text-3xl font-black uppercase tracking-tight text-ink">
        Venue Access
      </h1>
      <p class="text-signal uppercase tracking-widest text-sm mt-1">
        Venue: {data.venueId}
      </p>
    </div>
  </div>

  {#if data.loadError}
    <Panel padding="lg">
      <div class="text-sm font-bold uppercase tracking-widest text-urgency">
        {data.loadError}
      </div>
    </Panel>
  {:else if data.staff.length === 0}
    <EmptyState
      icon={Users}
      title="No Staff Access"
      description="No additional venue access records are assigned for this venue."
    />
  {:else}
    <Panel padding="none" overflowHidden>
      <div class="flex items-center gap-3 border-b border-line p-6">
        <Users class="text-signal" size={20} />
        <h3 class="text-sm font-black uppercase tracking-wider text-ink">
          Assigned Access
        </h3>
      </div>

      <div class="overflow-x-auto">
        <table class="table w-full">
          <thead>
            <tr
              class="border-line bg-panelSoft/70 text-xs uppercase tracking-widest text-inkMuted"
            >
              <th class="px-6 py-4 font-bold">Member</th>
              <th class="px-6 py-4 font-bold">Role</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-line">
            {#each data.staff as member}
              <tr class="transition-colors hover:bg-panelSoft/60">
                <td class="px-6 py-4">
                  <div class="flex items-center gap-3">
                    <div
                      class="flex h-8 w-8 items-center justify-center rounded bg-signal/20 text-xs font-bold text-signal"
                    >
                      {member.email.charAt(0).toUpperCase()}
                    </div>
                    <div>
                      <div class="font-medium text-ink">{member.email}</div>
                      <div class="font-mono text-xs text-inkMuted">
                        {member.id}
                      </div>
                    </div>
                  </div>
                </td>
                <td class="px-6 py-4">
                  <div
                    class="flex items-center gap-1.5 font-mono text-xs font-bold uppercase tracking-wider text-inkMuted"
                  >
                    {#if member.role === 'organizer'}
                      <Shield size={14} class="text-signal" />
                    {/if}
                    {member.role}
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </Panel>
  {/if}
</div>
