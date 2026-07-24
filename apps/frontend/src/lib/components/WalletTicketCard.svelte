<script lang="ts">
  import type { WalletTicket } from '$lib/api/types';
  import Panel from '$lib/components/Panel.svelte';

  let { ticket }: { ticket: WalletTicket } = $props();

  function qrCells(token: string) {
    const source = token || 'velox';
    return Array.from({ length: 49 }, (_, index) => {
      const code = source.charCodeAt(index % source.length);
      return (code + index * 17) % 5 < 2;
    });
  }

  function tokenFragment(token: string) {
    if (!token) return 'unavailable';
    return `${token.slice(0, 10)}...${token.slice(-8)}`;
  }
</script>

<Panel padding="none" overflowHidden>
  <article
    class="group transition-colors"
    class:opacity-50={ticket.status === 'CANCELLED'}
    class:grayscale={ticket.status === 'CANCELLED'}
  >
    <div class="flex gap-5 p-5">
      <div
        class="grid h-24 w-24 shrink-0 grid-cols-7 gap-0.5 rounded bg-white p-2 shadow-inner transition-transform group-hover:scale-105"
        aria-label="Signed ticket token pattern"
      >
        {#each qrCells(ticket.qr_token) as filled}
          <span
            class="rounded-[1px] {filled ? 'bg-carbon' : 'bg-transparent'}"
          ></span>
        {/each}
      </div>
      <div class="flex min-w-0 flex-1 flex-col justify-center">
        <h2
          class="text-ink group-hover:text-signal truncate text-xl font-black tracking-tight uppercase transition-colors"
        >
          {ticket.event}
        </h2>
        <p class="text-inkMuted text-sm font-medium">
          {ticket.venue}
        </p>
        <div
          class="border-line bg-panel/70 text-ink mt-2 inline-block rounded-sm border px-2 py-1 font-mono text-xs font-bold tabular-nums shadow-sm"
        >
          {ticket.seat} <span class="text-ink/40 mx-1">|</span>
          {ticket.status}
        </div>
        <p class="text-ink/40 mt-2 truncate font-mono text-[11px]">
          {tokenFragment(ticket.qr_token)}
        </p>
      </div>
    </div>
    <div
      class="border-line bg-panelSoft/60 flex items-center justify-between border-t px-5 py-3"
    >
      <span
        class="text-inkMuted flex items-center gap-2 font-mono text-xs font-bold tracking-widest uppercase tabular-nums"
      >
        <span
          class="h-1.5 w-1.5 rounded-full {ticket.transfer_status ===
          'AVAILABLE'
            ? 'bg-ok shadow-[0_0_5px_rgba(16,185,129,0.8)]'
            : 'bg-ink/30'}"
        ></span>
        {ticket.transfer_status}
      </span>
      <span
        class="border-line bg-panel text-inkMuted rounded-sm border px-3 py-2 text-xs font-bold tracking-widest uppercase"
      >
        Entry token expires {new Date(
          ticket.qr_token_expires_at
        ).toLocaleTimeString([], {
          hour: '2-digit',
          minute: '2-digit'
        })}
      </span>
    </div>
  </article>
</Panel>
