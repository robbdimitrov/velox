<script lang="ts">
  import { QrCode, Send, ShieldCheck } from '@lucide/svelte';

  let { data } = $props();
</script>

<main class="mx-auto max-w-6xl px-4 py-5">
  <section class="thin-panel p-4">
    <div
      class="flex flex-wrap items-center justify-between gap-3 border-b border-line pb-4"
    >
      <div>
        <h1 class="text-2xl font-black uppercase">Ticket Wallet</h1>
        <p class="text-sm text-ink/60">
          {data.wallet.tickets.length} upcoming tickets
        </p>
      </div>
      <div class="flex items-center gap-2 font-mono text-sm text-ok">
        <ShieldCheck size={18} />
        {data.wallet.verification_state}
      </div>
    </div>

    <div class="mt-4 grid gap-4 lg:grid-cols-[360px_1fr]">
      <div class="space-y-3">
        {#each data.wallet.tickets as ticket}
          <article class="border border-line bg-carbon p-4">
            <div class="flex gap-4">
              <div
                class="grid h-24 w-24 place-items-center border border-line bg-ink text-carbon"
              >
                <QrCode size={58} />
              </div>
              <div class="min-w-0 flex-1">
                <h2 class="truncate text-lg font-black uppercase">
                  {ticket.event}
                </h2>
                <p class="text-sm text-ink/60">{ticket.venue}</p>
                <p class="mt-2 font-mono">{ticket.seat} · Gate {ticket.gate}</p>
              </div>
            </div>
            <div
              class="mt-4 flex items-center justify-between border-t border-line pt-3 text-sm"
            >
              <span class="font-mono text-ink/60">{ticket.transfer_status}</span
              >
              <button
                class="btn btn-sm btn-primary"
                disabled={ticket.transfer_status !== 'AVAILABLE'}
              >
                <Send size={15} /> Transfer
              </button>
            </div>
          </article>
        {/each}
      </div>

      <div class="border border-line bg-carbon p-4">
        <h2 class="border-b border-line pb-3 text-sm font-black uppercase">
          Provenance Ledger
        </h2>
        {#each data.wallet.tickets as ticket}
          <details class="border-b border-line py-3" open>
            <summary class="cursor-pointer font-mono text-sm"
              >{ticket.ticket_id}</summary
            >
            <div class="mt-3 space-y-2">
              {#each ticket.ledger as row}
                <div
                  class="grid gap-1 border border-line p-2 text-xs md:grid-cols-[190px_1fr_120px]"
                >
                  <span class="font-mono text-ink/50">{row.timestamp}</span>
                  <span>{row.event_type} · {row.actor}</span>
                  <span class="font-mono text-ink/50">{row.correlation_id}</span
                  >
                </div>
              {/each}
            </div>
          </details>
        {/each}
      </div>
    </div>
  </section>
</main>
