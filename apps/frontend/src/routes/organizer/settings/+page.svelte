<script lang="ts">
  import { Settings } from '@lucide/svelte';
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

<div
  class="glass-panel p-12 rounded-3xl flex flex-col items-center justify-center text-center shadow-glow"
>
  <div
    class="w-16 h-16 rounded-full bg-info/10 flex items-center justify-center mb-4 text-info shadow-inner"
  >
    <Settings size={32} />
  </div>
  <h3 class="text-xl font-bold mb-2">Settings Coming Soon</h3>
  <p class="text-inkMuted max-w-md">
    Organizer settings and payout configurations are currently under
    development. Please check back later.
  </p>
</div>
