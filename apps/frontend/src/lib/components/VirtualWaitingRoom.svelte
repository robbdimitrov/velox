<script lang="ts">
  import { invalidateAll } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Loader, Users } from '@lucide/svelte';
  import AuthCard from '$lib/components/AuthCard.svelte';

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
  <AuthCard
    title="You are in line"
    description="Due to high demand, you have been placed in a virtual waiting room. Please do not refresh this page."
  >
    <div
      class="mb-6 flex h-16 w-16 items-center justify-center rounded bg-signal/20 text-signal"
    >
      <Users size={32} />
    </div>

    <div class="flex items-center gap-2 text-sm text-signal font-mono">
      <Loader size={16} class="animate-spin" />
      {polling ? 'Checking your spot...' : 'Waiting...'}
    </div>
  </AuthCard>
</div>
