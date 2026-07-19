<script lang="ts">
  import { page } from '$app/stores';
  import { Users, UserPlus, Shield, MoreHorizontal } from '@lucide/svelte';
  import InviteStaffModal from '$lib/components/InviteStaffModal.svelte';
  import Panel from '$lib/components/Panel.svelte';

  let venueId = $derived($page.params.venueId);
  let showModal = $state(false);

  let staff = $state([
    {
      id: '1',
      name: 'Alice Smith',
      email: 'alice@velox.com',
      role: 'ADMIN',
      status: 'ACTIVE'
    },
    {
      id: '2',
      name: 'Bob Jones',
      email: 'bob@velox.com',
      role: 'MANAGER',
      status: 'ACTIVE'
    },
    {
      id: '3',
      name: 'Charlie Brown',
      email: 'charlie@velox.com',
      role: 'STAFF',
      status: 'INVITED'
    }
  ]);
</script>

<div class="space-y-8">
  <div class="mb-8 flex justify-between items-end">
    <div>
      <h1 class="text-3xl font-black uppercase tracking-tight text-ink">
        Staff Management
      </h1>
      <p class="text-signal uppercase tracking-widest text-sm mt-1">
        Venue: {venueId}
      </p>
    </div>
    <button
      class="btn btn-primary rounded-sm text-primary-content shadow-[0_12px_30px_rgba(242,184,75,0.22)]"
      onclick={() => (showModal = true)}
    >
      <UserPlus size={18} />
      Invite Staff
    </button>
  </div>

  <Panel padding="none" overflowHidden>
    <div class="flex items-center gap-3 border-b border-line p-6">
      <Users class="text-signal" size={20} />
      <h3 class="text-sm font-black uppercase tracking-wider text-ink">
        Team Members
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
            <th class="px-6 py-4 font-bold">Status</th>
            <th class="px-6 py-4 font-bold text-right">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-line">
          {#each staff as member}
            <tr class="transition-colors hover:bg-panelSoft/60">
              <td class="px-6 py-4">
                <div class="flex items-center gap-3">
                  <div
                    class="flex h-8 w-8 items-center justify-center rounded bg-signal/20 text-xs font-bold text-signal"
                  >
                    {member.name.charAt(0)}
                  </div>
                  <div>
                    <div class="font-medium text-ink">{member.name}</div>
                    <div class="text-xs text-inkMuted">{member.email}</div>
                  </div>
                </div>
              </td>
              <td class="px-6 py-4">
                <div
                  class="flex items-center gap-1.5 font-mono text-xs font-bold uppercase tracking-wider text-inkMuted"
                >
                  {#if member.role === 'ADMIN'}
                    <Shield size={14} class="text-signal" />
                  {/if}
                  {member.role}
                </div>
              </td>
              <td class="px-6 py-4">
                {#if member.status === 'ACTIVE'}
                  <span
                    class="rounded-sm border border-ok/30 bg-ok/10 px-2 py-1 text-[10px] font-black uppercase tracking-widest text-ok"
                  >
                    {member.status}
                  </span>
                {:else}
                  <span
                    class="rounded-sm border border-warn/30 bg-warn/10 px-2 py-1 text-[10px] font-black uppercase tracking-widest text-warn"
                  >
                    {member.status}
                  </span>
                {/if}
              </td>
              <td class="px-6 py-4 text-right">
                <button
                  class="btn btn-ghost btn-sm p-2 text-inkMuted hover:text-ink"
                >
                  <MoreHorizontal size={16} />
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </Panel>
</div>

<InviteStaffModal bind:show={showModal} />
