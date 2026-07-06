<script lang="ts">
  import { QrCode, Send, ShieldCheck, Ticket } from '@lucide/svelte';
  let { data } = $props();

  const HISTORY_FILTERS = [
    'All',
    'Issued',
    'Transferred',
    'Used',
    'Upgraded',
    'Cancelled'
  ] as const;
  let historyFilter = $state<(typeof HISTORY_FILTERS)[number]>('All');

  let filteredTickets = $derived(
    historyFilter === 'All'
      ? data.wallet.tickets
      : data.wallet.tickets.filter(
          (ticket) => ticket.status === historyFilter.toUpperCase()
        )
  );
</script>

<main class="mx-auto max-w-6xl px-4 py-8">
  <section class="glass-panel p-6">
    <div
      class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-white/10 pb-6 mb-6"
    >
      <div>
        <h1
          class="text-3xl font-black uppercase tracking-tight text-white flex items-center gap-3"
        >
          <Ticket class="text-signal" size={28} /> Ticket Wallet
        </h1>
        <p
          class="text-sm font-bold uppercase tracking-widest text-inkMuted mt-2"
        >
          {data.wallet.tickets.length} upcoming tickets
        </p>
      </div>
      <div
        class="flex items-center gap-2 font-mono text-sm font-bold text-ok bg-ok/10 border border-ok/20 px-4 py-2 rounded-full shadow-[0_0_15px_rgba(16,185,129,0.2)]"
      >
        <ShieldCheck size={18} />
        {data.wallet.verification_state}
      </div>
    </div>

    <div class="flex flex-wrap items-center gap-2 mb-6">
      {#each HISTORY_FILTERS as label}
        <button
          class={`btn btn-xs rounded px-4 border-0 transition-all ${historyFilter === label ? 'bg-signal text-carbon font-bold' : 'bg-black/40 text-inkMuted hover:bg-black/60 hover:text-white'}`}
          onclick={() => (historyFilter = label)}
        >
          {label}
        </button>
      {/each}
    </div>

    <div class="grid gap-6 lg:grid-cols-[400px_1fr]">
      <div class="space-y-4">
        {#each filteredTickets as ticket}
          <article
            class={`glass-panel rounded overflow-hidden shadow-lg border border-white/10 bg-gradient-to-b from-black/60 to-black/30 hover:border-signal/40 transition-colors group ${ticket.status === 'CANCELLED' ? 'opacity-50 grayscale' : ''}`}
          >
            <div class="flex gap-5 p-5">
              <div
                class="grid h-24 w-24 shrink-0 place-items-center rounded bg-white text-carbon shadow-inner group-hover:scale-105 transition-transform"
                title={ticket.qr_token}
              >
                <QrCode size={64} />
              </div>
              <div class="min-w-0 flex-1 flex flex-col justify-center">
                <h2
                  class="truncate text-xl font-black uppercase text-white tracking-tight group-hover:text-signal transition-colors"
                >
                  {ticket.event}
                </h2>
                <p class="text-sm text-inkMuted font-medium">{ticket.venue}</p>
                <div
                  class="mt-2 inline-block bg-black/40 border border-white/5 rounded px-2 py-1 font-mono text-xs font-bold text-white shadow-sm"
                >
                  {ticket.seat} <span class="text-ink/40 mx-1">|</span>
                  {ticket.status}
                </div>
              </div>
            </div>
            <div
              class="flex items-center justify-between bg-black/40 px-5 py-3 border-t border-white/5"
            >
              <span
                class="font-mono text-xs font-bold uppercase tracking-widest text-inkMuted flex items-center gap-2"
              >
                <span
                  class="w-1.5 h-1.5 rounded-full {ticket.transfer_status ===
                  'AVAILABLE'
                    ? 'bg-ok shadow-[0_0_5px_rgba(16,185,129,0.8)]'
                    : 'bg-ink/30'}"
                ></span>
                {ticket.transfer_status}
              </span>
              <button
                class="btn btn-sm rounded border-0 bg-white/10 hover:bg-signal text-white font-bold transition-all disabled:opacity-30 disabled:hover:bg-white/10"
                disabled={ticket.transfer_status !== 'AVAILABLE' ||
                  ticket.status === 'CANCELLED'}
              >
                <Send size={14} class="mr-1" /> Transfer
              </button>
            </div>
          </article>
        {/each}
      </div>

      <div class="glass-panel p-6 h-max">
        <h2
          class="border-b border-white/10 pb-4 mb-4 text-sm font-black uppercase tracking-wider text-white"
        >
          Provenance Ledger
        </h2>
        <div class="space-y-4">
          {#each filteredTickets as ticket}
            <details
              class="group rounded border border-white/5 bg-black/30 overflow-hidden"
              open
            >
              <summary
                class="cursor-pointer font-mono text-xs font-bold uppercase tracking-widest text-signal bg-black/40 p-4 border-b border-white/5 hover:bg-black/60 transition-colors select-none"
              >
                Ticket ID: {ticket.ticket_id}
              </summary>
              <div class="p-4 space-y-2">
                {#each ticket.ledger as row}
                  <div
                    class="grid gap-2 rounded border border-white/5 bg-black/20 p-3 text-xs md:grid-cols-[180px_1fr_120px] items-center hover:bg-white/5 transition-colors"
                  >
                    <span class="font-mono text-ink/40">{row.timestamp}</span>
                    <span class="font-medium text-ink"
                      ><span class="text-white font-bold">{row.event_type}</span
                      > <span class="text-ink/40 mx-1">·</span>
                      {row.actor}</span
                    >
                    <span
                      class="font-mono text-ink/30 text-right truncate"
                      title={row.correlation_id}
                      >{row.correlation_id.split('-')[0]}...</span
                    >
                  </div>
                {/each}
              </div>
            </details>
          {/each}
        </div>
      </div>
    </div>
  </section>
</main>
