<script lang="ts">
  import { Settings } from '@lucide/svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
</script>

<svelte:head>
  <title>Settings - Velox Organizer</title>
</svelte:head>

<div class="mb-8 flex items-end justify-between">
  <div>
    <h1 class="text-3xl font-black uppercase tracking-tight">Settings</h1>
    <p class="text-inkMuted text-sm mt-1">
      Manage your organizer account and preferences.
    </p>
  </div>
</div>

<EmptyState
  icon={Settings}
  title="Settings Coming Soon"
  description="Organizer settings and payout configurations are currently under development. Please check back later."
/>
