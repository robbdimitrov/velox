<script lang="ts">
  import { Activity } from '@lucide/svelte';
  import { slide } from 'svelte/transition';

  let { url }: { url: string } = $props();
  let messages = $state<string[]>([
    'System connected. Waiting for live updates...'
  ]);

  $effect(() => {
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
</script>

<div
  class="grid h-28 grid-cols-[56px_1fr] overflow-hidden border-t border-line bg-panel"
>
  <div
    class="flex flex-col items-center justify-center border-r border-line bg-panelSoft text-accent"
  >
    <div class="relative">
      <div
        class="absolute inset-0 animate-ping rounded-full bg-accent/30 blur-sm"
      ></div>
      <Activity size={24} class="relative" />
    </div>
  </div>
  <div class="flex flex-col divide-y divide-line overflow-hidden py-1">
    {#each messages as message, index (message)}
      <div class="flex h-6 items-center px-4" in:slide={{ duration: 300 }}>
        <span class="mr-3 inline-block h-1.5 w-1.5 rounded-full bg-accent"
        ></span>
        <p
          class="truncate font-mono text-xs uppercase text-inkMuted transition-colors hover:text-ink"
        >
          {message}
        </p>
      </div>
    {/each}
  </div>
</div>
