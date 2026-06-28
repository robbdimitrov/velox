<script lang="ts">
  import { Activity } from '@lucide/svelte';
  import { onDestroy, onMount } from 'svelte';
  import { slide } from 'svelte/transition';

  let { url }: { url: string } = $props();
  let messages = $state<string[]>(['System connected. Waiting for live updates...']);

  onMount(() => {
    if (!url || typeof EventSource === 'undefined') return;

    const source = new EventSource(url);
    source.addEventListener('update', (event) => {
      try {
        const data = JSON.parse(event.data);
        const msg = `${data.event_id} ${data.section_id} ${data.seat_id} is now ${data.status}`;
        messages = [msg, ...messages].slice(0, 4);
      } catch {
        messages = [event.data, ...messages].slice(0, 4);
      }
    });
    source.onerror = () => source.close();

    return () => source.close();
  });

  onDestroy(() => {
    messages = messages.slice(0, 4);
  });
</script>

<div class="grid h-28 grid-cols-[56px_1fr] overflow-hidden bg-black/50 backdrop-blur-md">
  <div class="flex flex-col items-center justify-center border-r border-white/10 bg-black/20 text-accent shadow-inner">
    <div class="relative">
      <div class="absolute inset-0 animate-ping rounded-full bg-accent/40 blur-sm"></div>
      <Activity size={24} class="relative drop-shadow-[0_0_8px_rgba(255,42,95,0.8)]" />
    </div>
  </div>
  <div class="flex flex-col divide-y divide-white/5 overflow-hidden py-1">
    {#each messages as message, index (message)}
      <div
        class="flex h-6 items-center px-4"
        in:slide={{ duration: 300 }}
      >
        <span class="mr-3 inline-block h-1.5 w-1.5 rounded-full bg-signal shadow-[0_0_5px_rgba(124,58,237,0.8)]"></span>
        <p class="truncate font-mono text-xs uppercase text-inkMuted transition-colors hover:text-white">
          {message}
        </p>
      </div>
    {/each}
  </div>
</div>
