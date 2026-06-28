<script lang="ts">
  import { goto } from '$app/navigation';
  import { createGatewayClient, createIdempotencyKey } from '$lib/api/client';
  import { checkoutState, formatCountdown } from '$lib/state/checkout-state.svelte';
  import { LockKeyhole, AlertTriangle, CheckCircle2 } from '@lucide/svelte';
  import PrimaryButton from '$lib/components/PrimaryButton.svelte';

  let termsAccepted = $state(false);
  let tick = $state(Date.now());

  $effect(() => {
    const timer = setInterval(() => (tick = Date.now()), 1000);
    return () => clearInterval(timer);
  });

  const remaining = $derived.by(() => {
    if (!checkoutState.reservation) return 0;
    return Math.max(
      0,
      checkoutState.reservation.expires_at_server_ms - (tick + checkoutState.serverOffsetMs)
    );
  });

  async function submit() {
    if (!checkoutState.reservation || checkoutState.submitted || !termsAccepted) return;
    checkoutState.submitted = true;
    checkoutState.error = '';

    try {
      const client = createGatewayClient(fetch, '/api');
      const result = await client.checkout(
        {
          reservation_id: checkoutState.reservation.reservation_id,
          payment_method_token: 'pm_test_token', // Hardcoded as payment is disabled
          billing_postal_code: '12345',
          terms_accepted: termsAccepted
        },
        createIdempotencyKey(),
        checkoutState.reservation.reservation_token
      );

      if (result.status === 'CONFIRMED') {
        checkoutState.clear();
        await goto('/wallet');
      } else {
        checkoutState.error = `Checkout ${result.status.toLowerCase()}`;
      }
    } catch {
      checkoutState.error = 'Reservation could not be confirmed. The idempotency key prevents duplicate requests.';
      checkoutState.submitted = false;
    }
  }
</script>

<main class="mx-auto grid max-w-5xl gap-6 px-4 py-8 lg:grid-cols-[1fr_360px]">
  {#if checkoutState.reservation}
    <section class="glass-panel p-8 flex flex-col h-full">
      <h1 class="text-3xl font-black uppercase tracking-tight text-transparent bg-clip-text bg-gradient-to-r from-white to-inkMuted">Confirm Reservation</h1>
      
      <div class="my-6 rounded-xl border border-urgency/20 bg-urgency/5 py-6 text-center shadow-inner relative overflow-hidden shrink-0">
        <div class="absolute inset-0 bg-[radial-gradient(ellipse_at_center,_var(--tw-gradient-stops))] from-urgency/10 to-transparent blur-xl"></div>
        <p class="text-xs font-bold uppercase tracking-widest text-urgency mb-1 relative">Hold expires in</p>
        <p class="mono-num font-mono text-5xl font-black text-urgency drop-shadow-[0_0_10px_rgba(255,42,95,0.8)] relative">
          {formatCountdown(remaining)}
        </p>
      </div>

      <div class="flex-grow">
        <h3 class="text-sm font-black uppercase tracking-widest text-inkMuted mb-4">Secured Tickets</h3>
        <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
          {#each checkoutState.reservation.seats as seat}
            <div class="flex items-center justify-center rounded-lg border border-white/5 bg-black/40 p-4 font-mono shadow-sm hover:border-signal/30 transition-colors">
              <span class="text-xl font-bold text-ink">{seat.seat_id}</span>
            </div>
          {/each}
        </div>
      </div>

      <div class="mt-6 pt-6 border-t border-white/10 flex justify-between items-center text-xs text-ink/40">
        <span class="uppercase tracking-widest">Reservation version</span>
        <span class="font-mono">{checkoutState.reservation.version}</span>
      </div>
    </section>

    <aside class="glass-panel h-max p-6 sticky top-28">
      <div class="flex items-center gap-3 border-b border-white/10 pb-4 mb-6">
        <div class="p-2 bg-gradient-to-br from-signal to-primary rounded-lg shadow-md">
          <CheckCircle2 class="text-white" size={20} />
        </div>
        <h2 class="text-lg font-black uppercase tracking-wider text-white">Review & Complete</h2>
      </div>

      <label class="flex items-start gap-3 text-sm cursor-pointer hover:text-white transition-colors group mb-6 bg-black/20 p-4 rounded-xl border border-white/5">
        <input bind:checked={termsAccepted} class="checkbox checkbox-primary checkbox-sm mt-0.5 rounded" type="checkbox" />
        <span class="text-inkMuted group-hover:text-ink transition-colors leading-tight">I accept the transfer, refund, and venue entry terms.</span>
      </label>

      {#if checkoutState.error}
        <div class="mb-6 flex items-start gap-2 rounded-lg border border-urgency/50 bg-urgency/10 p-3 text-sm text-urgency">
          <AlertTriangle size={18} class="shrink-0 mt-0.5" />
          <p>{checkoutState.error}</p>
        </div>
      {/if}

      <PrimaryButton disabled={!termsAccepted || checkoutState.submitted || remaining <= 0} onclick={submit}>
        <LockKeyhole size={18} />
        {checkoutState.submitted ? 'Securing...' : 'Complete Reservation'}
      </PrimaryButton>
    </aside>

  {:else}
    <section class="glass-panel p-12 text-center col-span-full max-w-2xl mx-auto flex flex-col items-center justify-center min-h-[400px]">
      <div class="p-4 bg-white/5 rounded-full mb-6 text-signal">
        <AlertTriangle size={48} />
      </div>
      <h1 class="text-3xl font-black uppercase text-white mb-3">No Active Reservation</h1>
      <p class="text-inkMuted text-lg max-w-md">
        Your hold has expired or you haven't selected any seats yet. Return to the seat map and secure your tickets.
      </p>
      <a class="btn btn-primary btn-lg rounded-full px-8 mt-8 shadow-glow text-white" href="/">Find Events</a>
    </section>
  {/if}
</main>
