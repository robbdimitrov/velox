<script lang="ts">
  import { Activity } from '@lucide/svelte';
  import { slide } from 'svelte/transition';

  const reconnectDelayMs = 3000;

  let { url }: { url: string } = $props();
  let nextMessageId = 0;
  let messages = $state<{ id: number; text: string }[]>([
    { id: nextMessageId++, text: 'Connecting to live updates...' }
  ]);

  function pushMessage(text: string) {
    messages = [{ id: nextMessageId++, text }, ...messages].slice(0, 4);
  }

  $effect(() => {
    if (!url || typeof EventSource === 'undefined') return;

    let source: EventSource;
    let reconnectTimer: ReturnType<typeof setTimeout>;
    let stopped = false;

    function connect() {
      source = new EventSource(url);
      source.addEventListener('open', () =>
        pushMessage('Live updates connected.')
      );
      source.addEventListener('update', (event) => {
        try {
          const data = JSON.parse(event.data);
          pushMessage(
            `${data.event_id} ${data.section_id} ${data.seat_id} is now ${data.status}`
          );
        } catch {
          pushMessage(event.data);
        }
      });
      source.onerror = () => {
        source.close();
        if (stopped) return;
        pushMessage('Live updates disconnected. Reconnecting...');
        reconnectTimer = setTimeout(connect, reconnectDelayMs);
      };
    }

    connect();

    return () => {
      stopped = true;
      clearTimeout(reconnectTimer);
      source.close();
    };
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
    {#each messages as message (message.id)}
      <div class="flex h-6 items-center px-4" in:slide={{ duration: 300 }}>
        <span class="bg-accent mr-3 inline-block h-1.5 w-1.5 rounded-full"
        ></span>
        <p
          class="text-inkMuted hover:text-ink truncate font-mono text-xs uppercase transition-colors"
        >
          {message.text}
        </p>
      </div>
    {/each}
  </div>
</div>
