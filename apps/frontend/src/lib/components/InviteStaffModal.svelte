<script lang="ts">
  import { X, Mail, UserPlus } from '@lucide/svelte';
  import Panel from '$lib/components/Panel.svelte';
  import PrimaryButton from '$lib/components/PrimaryButton.svelte';
  import TextField from '$lib/components/TextField.svelte';

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
    <div class="w-full max-w-md">
      <Panel padding="lg">
        <button
          class="absolute top-4 right-4 text-inkMuted transition-colors hover:text-ink"
          onclick={() => (show = false)}
        >
          <X size={20} />
        </button>

        <div class="mb-6 flex items-center gap-3 border-b border-line pb-4">
          <div class="p-2 bg-signal/20 rounded text-signal">
            <UserPlus size={20} />
          </div>
          <h3 class="text-lg font-black uppercase tracking-wider text-ink">
            Invite Staff
          </h3>
        </div>

        <div class="space-y-4">
          <TextField
            id="staff-email"
            label="Email Address"
            type="email"
            bind:value={email}
            icon={Mail}
            placeholder="staff@venue.com"
          />

          <label class="form-control">
            <span class="label-text text-inkMuted font-medium mb-1">Role</span>
            <select
              bind:value={role}
              class="select select-bordered w-full rounded-sm border-line bg-carbon/60 text-ink focus:border-signal focus:outline-none focus:ring-1 focus:ring-signal/50"
            >
              <option value="ADMIN">Admin</option>
              <option value="MANAGER">Manager</option>
              <option value="STAFF">Staff</option>
            </select>
          </label>
        </div>

        <div class="mt-8 flex justify-end gap-3">
          <button
            class="btn btn-ghost btn-sm rounded-sm text-ink hover:bg-panelSoft"
            onclick={() => (show = false)}
          >
            Cancel
          </button>
          <PrimaryButton onclick={handleInvite} disabled={!email} flush>
            Send Invite
          </PrimaryButton>
        </div>
      </Panel>
    </div>
  </div>
{/if}
