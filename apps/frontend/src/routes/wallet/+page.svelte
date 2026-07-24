<script lang="ts">
  import { LockKeyhole, ShieldCheck, Ticket } from '@lucide/svelte';
  import Panel from '$lib/components/Panel.svelte';
  import WalletTicketCard from '$lib/components/WalletTicketCard.svelte';
  import { pageTitle } from '$lib/pageTitle';

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

  function ledgerEventLabel(eventType: string) {
    if (eventType === 'TicketIssued') return 'Reservation ticket issued';
    return eventType;
  }
</script>

<svelte:head>
  <title>{pageTitle('Wallet')}</title>
</svelte:head>

<main class="w-full">
  <Panel padding="lg">
    <div
      class="border-line mb-6 flex flex-col justify-between gap-4 border-b pb-6 sm:flex-row sm:items-center"
    >
      <div>
        <h1
          class="text-ink flex items-center gap-3 text-3xl font-black tracking-tight uppercase"
        >
          <Ticket class="text-signal" size={28} /> Reservation Wallet
        </h1>
        <p
          class="text-inkMuted mt-2 text-sm font-bold tracking-widest uppercase"
        >
          {data.wallet.tickets.length} reservation tickets
        </p>
      </div>
      <div
        class="border-ok/20 bg-ok/10 text-ok flex items-center gap-2 rounded-sm border px-4 py-2 font-mono text-sm font-bold tabular-nums"
      >
        <ShieldCheck size={18} />
        {data.wallet.verification_state}
      </div>
    </div>

    {#if !data.authRequired && data.wallet.tickets.length > 0}
      <div class="mb-6 flex flex-wrap items-center gap-2">
        {#each HISTORY_FILTERS as label}
          <button
            class="btn btn-xs hover:bg-panel hover:text-ink rounded-sm border-0 px-4 transition-all"
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
        class="border-line bg-panelSoft/70 grid min-h-[320px] place-items-center rounded-sm border p-8 text-center"
      >
        <div>
          <div
            class="bg-signal text-primary-content mx-auto mb-4 grid h-14 w-14 place-items-center rounded-sm"
          >
            <LockKeyhole size={28} />
          </div>
          <h2 class="text-ink text-xl font-black tracking-tight uppercase">
            Sign In Required
          </h2>
          <p class="text-inkMuted mt-2 max-w-md text-sm">
            Reservation ticket wallet access is limited to the signed-in
            reserver.
          </p>
        </div>
      </div>
    {:else if data.wallet.tickets.length === 0}
      <div
        class="border-line bg-panelSoft/70 grid min-h-[320px] place-items-center rounded-sm border p-8 text-center"
      >
        <div>
          <div
            class="bg-panel text-signal mx-auto mb-4 grid h-14 w-14 place-items-center rounded"
          >
            <Ticket size={28} />
          </div>
          <h2 class="text-ink text-xl font-black tracking-tight uppercase">
            No Reservation Tickets Yet
          </h2>
          <p class="text-inkMuted mt-2 max-w-md text-sm">
            Confirmed reservations will appear here with signed entry tokens and
            provenance.
          </p>
        </div>
      </div>
    {:else if filteredTickets.length === 0}
      <div
        class="border-line bg-panelSoft/70 grid min-h-[240px] place-items-center rounded-sm border p-8 text-center"
      >
        <p class="text-inkMuted text-sm font-bold tracking-widest uppercase">
          No reservation tickets match this status
        </p>
      </div>
    {:else}
      <div class="grid gap-6 lg:grid-cols-[400px_1fr]">
        <div class="space-y-4">
          {#each filteredTickets as ticket}
            <WalletTicketCard {ticket} />
          {/each}
        </div>

        <Panel padding="lg" hMax>
          <h2
            class="border-line text-ink mb-4 border-b pb-4 text-sm font-black tracking-wider uppercase"
          >
            Provenance Ledger
          </h2>
          <div class="space-y-4">
            {#each filteredTickets as ticket}
              <details
                class="group border-line bg-panelSoft/70 overflow-hidden rounded-sm border"
                open
              >
                <summary
                  class="border-line bg-panel/70 text-signal hover:bg-panel cursor-pointer border-b p-4 font-mono text-xs font-bold tracking-widest uppercase tabular-nums transition-colors select-none"
                >
                  Reservation ticket ID: {ticket.ticket_id}
                </summary>
                <div class="space-y-2 p-4">
                  {#each ticket.ledger as row}
                    <div
                      class="border-line bg-panel/50 hover:bg-panel grid items-center gap-2 rounded-sm border p-3 text-xs transition-colors md:grid-cols-[180px_1fr_120px]"
                    >
                      <span class="text-ink/40 font-mono tabular-nums"
                        >{row.timestamp}</span
                      >
                      <span class="text-ink font-medium"
                        ><span class="text-ink font-bold"
                          >{ledgerEventLabel(row.event_type)}</span
                        >
                        <span class="text-ink/40 mx-1">·</span>
                        {row.actor}</span
                      >
                      <span
                        class="text-ink/30 truncate text-right font-mono tabular-nums"
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
