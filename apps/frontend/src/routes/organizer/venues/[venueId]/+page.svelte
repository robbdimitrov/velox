<script lang="ts">
  import { Shield, Users } from '@lucide/svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import Panel from '$lib/components/Panel.svelte';

  let { data } = $props();
</script>

<svelte:head>
  <title>Venue Access — Velox</title>
</svelte:head>

<div class="space-y-8">
  <div class="mb-8 flex items-end justify-between">
    <div>
      <h1 class="text-ink text-3xl font-black tracking-tight uppercase">
        Venue Access
      </h1>
      <p class="text-signal mt-1 text-sm tracking-widest uppercase">
        Venue: {data.venueId}
      </p>
    </div>
  </div>

  {#if data.loadError}
    <Panel padding="lg">
      <div class="text-urgency text-sm font-bold tracking-widest uppercase">
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
      <div class="border-line flex items-center gap-3 border-b p-6">
        <Users class="text-signal" size={20} />
        <h3 class="text-ink text-sm font-black tracking-wider uppercase">
          Assigned Access
        </h3>
      </div>

      <div class="overflow-x-auto">
        <table class="table w-full">
          <thead>
            <tr
              class="border-line bg-panelSoft/70 text-inkMuted text-xs tracking-widest uppercase"
            >
              <th class="px-6 py-4 font-bold">Member</th>
              <th class="px-6 py-4 font-bold">Role</th>
            </tr>
          </thead>
          <tbody class="divide-line divide-y">
            {#each data.staff as member}
              <tr class="hover:bg-panelSoft/60 transition-colors">
                <td class="px-6 py-4">
                  <div class="flex items-center gap-3">
                    <div
                      class="bg-signal/20 text-signal flex h-8 w-8 items-center justify-center rounded text-xs font-bold"
                    >
                      {member.email.charAt(0).toUpperCase()}
                    </div>
                    <div>
                      <div class="text-ink font-medium">{member.email}</div>
                      <div class="text-inkMuted font-mono text-xs">
                        {member.id}
                      </div>
                    </div>
                  </div>
                </td>
                <td class="px-6 py-4">
                  <div
                    class="text-inkMuted flex items-center gap-1.5 font-mono text-xs font-bold tracking-wider uppercase"
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
