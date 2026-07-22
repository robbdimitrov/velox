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
  class="border-line bg-panel grid h-28 grid-cols-[56px_1fr] overflow-hidden border-t"
>
  <div
    class="border-line bg-panelSoft text-accent flex flex-col items-center justify-center border-r"
  >
    <div class="relative">
      <div
        class="bg-accent/30 absolute inset-0 animate-ping rounded-full blur-sm"
      ></div>
      <Activity size={24} class="relative" />
    </div>
  </div>
  <div class="divide-line flex flex-col divide-y overflow-hidden py-1">
    {#each messages as message, index (message)}
      <div class="flex h-6 items-center px-4" in:slide={{ duration: 300 }}>
        <span class="bg-accent mr-3 inline-block h-1.5 w-1.5 rounded-full"
        ></span>
        <p
          class="text-inkMuted hover:text-ink truncate font-mono text-xs uppercase transition-colors"
        >
          {message}
        </p>
      </div>
    {/each}
  </div>
</div>
