<script lang="ts">
  import { Activity } from '@lucide/svelte';
  import { onDestroy, onMount } from 'svelte';

  let { url }: { url: string } = $props();
  let messages = $state<string[]>([
    'North Pier Hall section A entering sale window',
    'Civic Bowl upper deck inventory refreshed',
    'Wallet projection lag stable under 100ms'
  ]);

  onMount(() => {
    if (!url || typeof EventSource === 'undefined') return;

    const source = new EventSource(url);
    source.onmessage = (event) => {
      messages = [event.data, ...messages].slice(0, 4);
    };
    source.onerror = () => source.close();

    return () => source.close();
  });

  onDestroy(() => {
    messages = messages.slice(0, 4);
  });
</script>

<section class="thin-panel grid h-28 grid-cols-[44px_1fr] overflow-hidden">
  <div
    class="flex items-center justify-center border-r border-line text-urgency"
  >
    <Activity size={20} />
  </div>
  <div class="divide-y divide-line">
    {#each messages as message}
      <p class="h-7 truncate px-3 py-1 font-mono text-xs uppercase text-ink/70">
        {message}
      </p>
    {/each}
  </div>
</section>
