<script lang="ts">
  import { LockKeyhole, ShieldCheck, Ticket } from '@lucide/svelte';
  import Panel from '$lib/components/Panel.svelte';

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

  function ledgerEventLabel(eventType: string) {
    if (eventType === 'TicketIssued') return 'Reservation ticket issued';
    return eventType;
  }
</script>

<main class="w-full">
  <Panel padding="lg">
    <div
      class="mb-6 flex flex-col justify-between gap-4 border-b border-line pb-6 sm:flex-row sm:items-center"
    >
      <div>
        <h1
          class="flex items-center gap-3 text-3xl font-black uppercase tracking-tight text-ink"
        >
          <Ticket class="text-signal" size={28} /> Reservation Wallet
        </h1>
        <p
          class="text-sm font-bold uppercase tracking-widest text-inkMuted mt-2"
        >
          {data.wallet.tickets.length} reservation tickets
        </p>
      </div>
      <div
        class="font-mono tabular-nums flex items-center gap-2 rounded-sm border border-ok/20 bg-ok/10 px-4 py-2 text-sm font-bold text-ok"
      >
        <ShieldCheck size={18} />
        {data.wallet.verification_state}
      </div>
    </div>

    {#if !data.authRequired && data.wallet.tickets.length > 0}
      <div class="flex flex-wrap items-center gap-2 mb-6">
        {#each HISTORY_FILTERS as label}
          <button
            class="btn btn-xs rounded-sm border-0 px-4 transition-all hover:bg-panel hover:text-ink"
            class:bg-signal={historyFilter === label}
            class:font-bold={historyFilter === label}
            class:text-primary-content={historyFilter === label}
            class:bg-panelSoft={historyFilter !== label}
            class:text-inkMuted={historyFilter !== label}
            onclick={() => (historyFilter = label)}
          >
            {label}
          </button>
        {/each}
      </div>
    {/if}

    {#if data.authRequired}
      <div
        class="grid min-h-[320px] place-items-center rounded-sm border border-line bg-panelSoft/70 p-8 text-center"
      >
        <div>
          <div
            class="mx-auto mb-4 grid h-14 w-14 place-items-center rounded-sm bg-signal text-primary-content"
          >
            <LockKeyhole size={28} />
          </div>
          <h2 class="text-xl font-black uppercase tracking-tight text-ink">
            Sign In Required
          </h2>
          <p class="mt-2 max-w-md text-sm text-inkMuted">
            Reservation ticket wallet access is limited to the signed-in
            reserver.
          </p>
        </div>
      </div>
    {:else if data.wallet.tickets.length === 0}
      <div
        class="grid min-h-[320px] place-items-center rounded-sm border border-line bg-panelSoft/70 p-8 text-center"
      >
        <div>
          <div
            class="mx-auto mb-4 grid h-14 w-14 place-items-center rounded bg-panel text-signal"
          >
            <Ticket size={28} />
          </div>
          <h2 class="text-xl font-black uppercase tracking-tight text-ink">
            No Reservation Tickets Yet
          </h2>
          <p class="mt-2 max-w-md text-sm text-inkMuted">
            Confirmed reservations will appear here with signed entry tokens and
            provenance.
          </p>
        </div>
      </div>
    {:else if filteredTickets.length === 0}
      <div
        class="grid min-h-[240px] place-items-center rounded-sm border border-line bg-panelSoft/70 p-8 text-center"
      >
        <p class="text-sm font-bold uppercase tracking-widest text-inkMuted">
          No reservation tickets match this status
        </p>
      </div>
    {:else}
      <div class="grid gap-6 lg:grid-cols-[400px_1fr]">
        <div class="space-y-4">
          {#each filteredTickets as ticket}
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
                        class="rounded-[1px] {filled
                          ? 'bg-carbon'
                          : 'bg-transparent'}"
                      ></span>
                    {/each}
                  </div>
                  <div class="min-w-0 flex-1 flex flex-col justify-center">
                    <h2
                      class="truncate text-xl font-black uppercase tracking-tight text-ink transition-colors group-hover:text-signal"
                    >
                      {ticket.event}
                    </h2>
                    <p class="text-sm text-inkMuted font-medium">
                      {ticket.venue}
                    </p>
                    <div
                      class="font-mono tabular-nums mt-2 inline-block rounded-sm border border-line bg-panel/70 px-2 py-1 text-xs font-bold text-ink shadow-sm"
                    >
                      {ticket.seat} <span class="text-ink/40 mx-1">|</span>
                      {ticket.status}
                    </div>
                    <p class="mt-2 truncate font-mono text-[11px] text-ink/40">
                      {tokenFragment(ticket.qr_token)}
                    </p>
                  </div>
                </div>
                <div
                  class="flex items-center justify-between border-t border-line bg-panelSoft/60 px-5 py-3"
                >
                  <span
                    class="font-mono tabular-nums flex items-center gap-2 text-xs font-bold uppercase tracking-widest text-inkMuted"
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
                    class="rounded-sm border border-line bg-panel px-3 py-2 text-xs font-bold uppercase tracking-widest text-inkMuted"
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
          {/each}
        </div>

        <Panel padding="lg" hMax>
          <h2
            class="mb-4 border-b border-line pb-4 text-sm font-black uppercase tracking-wider text-ink"
          >
            Provenance Ledger
          </h2>
          <div class="space-y-4">
            {#each filteredTickets as ticket}
              <details
                class="group overflow-hidden rounded-sm border border-line bg-panelSoft/70"
                open
              >
                <summary
                  class="font-mono tabular-nums cursor-pointer select-none border-b border-line bg-panel/70 p-4 text-xs font-bold uppercase tracking-widest text-signal transition-colors hover:bg-panel"
                >
                  Reservation ticket ID: {ticket.ticket_id}
                </summary>
                <div class="p-4 space-y-2">
                  {#each ticket.ledger as row}
                    <div
                      class="grid items-center gap-2 rounded-sm border border-line bg-panel/50 p-3 text-xs transition-colors hover:bg-panel md:grid-cols-[180px_1fr_120px]"
                    >
                      <span class="font-mono tabular-nums text-ink/40"
                        >{row.timestamp}</span
                      >
                      <span class="font-medium text-ink"
                        ><span class="font-bold text-ink"
                          >{ledgerEventLabel(row.event_type)}</span
                        >
                        <span class="text-ink/40 mx-1">·</span>
                        {row.actor}</span
                      >
                      <span
                        class="font-mono tabular-nums truncate text-right text-ink/30"
                        title={row.correlation_id}
                        >{row.correlation_id.split('-')[0]}...</span
                      >
                    </div>
                  {/each}
                </div>
              </details>
            {/each}
          </div>
        </Panel>
      </div>
    {/if}
  </Panel>
</main>
