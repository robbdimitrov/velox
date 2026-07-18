<script lang="ts">
  import { page } from '$app/stores';
  import { Users, UserPlus, Shield, MoreHorizontal } from '@lucide/svelte';
  import InviteStaffModal from '$lib/components/InviteStaffModal.svelte';

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
      <h1 class="text-3xl font-black uppercase text-white tracking-tight">
        Staff Management
      </h1>
      <p class="text-signal uppercase tracking-widest text-sm mt-1">
        Venue: {venueId}
      </p>
    </div>
    <button class="btn velox-action rounded" onclick={() => (showModal = true)}>
      <UserPlus size={18} />
      Invite Staff
    </button>
  </div>

  <div class="glass-panel overflow-hidden">
    <div class="flex items-center gap-3 p-6 border-b border-white/10">
      <Users class="text-signal" size={20} />
      <h3 class="text-sm font-black uppercase tracking-wider text-white">
        Team Members
      </h3>
    </div>

    <div class="overflow-x-auto">
      <table class="table w-full">
        <thead>
          <tr
            class="border-white/10 text-inkMuted text-xs uppercase tracking-widest bg-black/20"
          >
            <th class="px-6 py-4 font-bold">Member</th>
            <th class="px-6 py-4 font-bold">Role</th>
            <th class="px-6 py-4 font-bold">Status</th>
            <th class="px-6 py-4 font-bold text-right">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-white/5">
          {#each staff as member}
            <tr class="hover:bg-white/5 transition-colors">
              <td class="px-6 py-4">
                <div class="flex items-center gap-3">
                  <div
                    class="flex h-8 w-8 items-center justify-center rounded bg-signal/20 text-xs font-bold text-signal"
                  >
                    {member.name.charAt(0)}
                  </div>
                  <div>
                    <div class="text-white font-medium">{member.name}</div>
                    <div class="text-xs text-inkMuted">{member.email}</div>
                  </div>
                </div>
              </td>
              <td class="px-6 py-4">
                <div
                  class="flex items-center gap-1.5 text-xs font-mono font-bold uppercase tracking-wider text-inkMuted"
                >
                  {#if member.role === 'ADMIN'}
                    <Shield size={14} class="text-signal" />
                  {/if}
                  {member.role}
                </div>
              </td>
              <td class="px-6 py-4">
                <span
                  class={`rounded border px-2 py-1 text-[10px] font-black uppercase tracking-widest ${member.status === 'ACTIVE' ? 'border-ok/30 text-ok bg-ok/10' : 'border-warn/30 text-warn bg-warn/10'}`}
                >
                  {member.status}
                </span>
              </td>
              <td class="px-6 py-4 text-right">
                <button
                  class="btn btn-ghost btn-sm text-inkMuted hover:text-white p-2"
                >
                  <MoreHorizontal size={16} />
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </div>
</div>

<InviteStaffModal bind:show={showModal} />
