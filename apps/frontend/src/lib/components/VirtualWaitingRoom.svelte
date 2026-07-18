<script lang="ts">
  import { invalidateAll } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Loader, Users } from '@lucide/svelte';

  let polling = $state(false);

  onMount(() => {
    const interval = setInterval(async () => {
      polling = true;
      await invalidateAll();
      polling = false;
    }, 5000);
    return () => clearInterval(interval);
  });
</script>

<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4"
>
  <div class="glass-panel app-auth flex flex-col items-center p-8 text-center">
    <div
      class="mb-6 flex h-16 w-16 items-center justify-center rounded bg-signal/20 text-signal"
    >
      <Users size={32} />
    </div>
    <h2 class="text-2xl font-black uppercase text-white mb-2">
      You are in line
    </h2>
    <p class="text-inkMuted mb-6">
      Due to high demand, you have been placed in a virtual waiting room. Please
      do not refresh this page.
    </p>

    <div class="flex items-center gap-2 text-sm text-signal font-mono">
      <Loader size={16} class="animate-spin" />
      {polling ? 'Checking your spot...' : 'Waiting...'}
    </div>
  </div>
</div>
