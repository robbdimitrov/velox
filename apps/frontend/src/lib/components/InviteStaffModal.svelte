<script lang="ts">
  import { X, Mail, UserPlus } from '@lucide/svelte';

  let { show = $bindable(false) } = $props();
  let email = $state('');
  let role = $state('STAFF');

  function handleInvite() {
    email = '';
    role = 'STAFF';
    show = false;
  }
</script>

{#if show}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4"
  >
    <div class="glass-panel max-w-md w-full relative">
      <button
        class="absolute top-4 right-4 text-inkMuted hover:text-white transition-colors"
        onclick={() => (show = false)}
      >
        <X size={20} />
      </button>

      <div class="p-6">
        <div class="flex items-center gap-3 mb-6 border-b border-white/10 pb-4">
          <div class="p-2 bg-signal/20 rounded text-signal">
            <UserPlus size={20} />
          </div>
          <h3 class="text-lg font-black uppercase tracking-wider text-white">
            Invite Staff
          </h3>
        </div>

        <div class="space-y-4">
          <label class="form-control">
            <span class="label-text text-inkMuted font-medium mb-1"
              >Email Address</span
            >
            <div class="velox-field w-full flex items-center gap-2 px-3 py-2">
              <Mail size={16} class="text-inkMuted" />
              <input
                bind:value={email}
                type="email"
                placeholder="staff@venue.com"
                class="bg-transparent text-sm text-white outline-none w-full"
              />
            </div>
          </label>

          <label class="form-control">
            <span class="label-text text-inkMuted font-medium mb-1">Role</span>
            <select
              bind:value={role}
              class="select select-bordered velox-field w-full text-white"
            >
              <option value="ADMIN">Admin</option>
              <option value="MANAGER">Manager</option>
              <option value="STAFF">Staff</option>
            </select>
          </label>
        </div>

        <div class="mt-8 flex justify-end gap-3">
          <button
            class="btn btn-sm btn-ghost hover:bg-white/5 text-ink rounded"
            onclick={() => (show = false)}
          >
            Cancel
          </button>
          <button
            class="btn btn-sm velox-action rounded"
            onclick={handleInvite}
            disabled={!email}
          >
            Send Invite
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
