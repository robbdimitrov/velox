<script lang="ts">
  import { goto } from '$app/navigation';
  import { createGatewayClient, createIdempotencyKey, formatMoney } from '$lib/api/client';
  import { checkoutState, formatCountdown } from '$lib/state/checkout-state.svelte';
  import { CreditCard, LockKeyhole, AlertTriangle } from '@lucide/svelte';

  let postalCode = $state('');
  let termsAccepted = $state(false);
  let paymentToken = $state('pm_test_card_token');
  let tick = $state(Date.now());

  $effect(() => {
    const timer = setInterval(() => (tick = Date.now()), 1000);
    return () => clearInterval(timer);
  });

  const remaining = $derived.by(() => {
    tick;
    return checkoutState.msRemaining;
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
          payment_method_token: paymentToken,
          billing_postal_code: postalCode,
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
      checkoutState.error = 'Payment could not be confirmed. The idempotency key prevents duplicate charge attempts.';
      checkoutState.submitted = false;
    }
  }
</script>

<main class="mx-auto grid max-w-5xl gap-6 px-4 py-8 lg:grid-cols-[1fr_360px]">
  {#if checkoutState.reservation}
    <section class="glass-panel p-8">
      <h1 class="text-3xl font-black uppercase tracking-tight text-transparent bg-clip-text bg-gradient-to-r from-white to-inkMuted">Checkout</h1>
      
      <div class="my-6 rounded-xl border border-urgency/20 bg-urgency/5 py-6 text-center shadow-inner relative overflow-hidden">
        <div class="absolute inset-0 bg-[radial-gradient(ellipse_at_center,_var(--tw-gradient-stops))] from-urgency/10 to-transparent blur-xl"></div>
        <p class="text-xs font-bold uppercase tracking-widest text-urgency mb-1 relative">Reservation expires in</p>
        <p class="mono-num font-mono text-5xl font-black text-urgency drop-shadow-[0_0_10px_rgba(255,42,95,0.8)] relative">
          {formatCountdown(remaining)}
        </p>
      </div>

      <div class="space-y-3">
        {#each checkoutState.reservation.seats as seat}
          <div class="flex justify-between items-center rounded-lg border border-white/5 bg-black/40 p-4 font-mono shadow-sm hover:border-signal/30 transition-colors">
            <span class="text-lg font-bold text-ink">{seat.seat_id}</span>
            <span class="text-signal font-black">{formatMoney(seat.price_cents)}</span>
          </div>
        {/each}
      </div>

      <dl class="mt-6 space-y-3 border-t border-white/10 pt-6 font-mono text-lg">
        <div class="flex justify-between items-center text-inkMuted">
          <dt>Fees</dt>
          <dd>{formatMoney(checkoutState.reservation.fees_cents)}</dd>
        </div>
        <div class="flex justify-between items-center text-2xl font-black text-white pt-2">
          <dt>Total</dt>
          <dd class="text-ok drop-shadow-[0_0_8px_rgba(16,185,129,0.5)]">{formatMoney(checkoutState.reservation.total_cents)}</dd>
        </div>
        <div class="flex justify-between items-center text-xs text-ink/40 pt-4">
          <dt class="uppercase tracking-widest">Reservation version</dt>
          <dd>{checkoutState.reservation.version}</dd>
        </div>
      </dl>
    </section>

    <aside class="glass-panel h-max p-6 sticky top-28">
      <div class="flex items-center gap-3 border-b border-white/10 pb-4 mb-6">
        <div class="p-2 bg-gradient-to-br from-signal to-primary rounded-lg shadow-md">
          <CreditCard class="text-white" size={20} />
        </div>
        <h2 class="text-lg font-black uppercase tracking-wider text-white">Payment</h2>
      </div>

      <label class="form-control mb-4">
        <span class="label-text text-inkMuted font-medium mb-1.5">Payment method token</span>
        <input bind:value={paymentToken} class="input input-bordered border-white/10 bg-black/40 text-ink rounded-lg focus:border-signal shadow-inner" />
      </label>

      <label class="form-control mb-6">
        <span class="label-text text-inkMuted font-medium mb-1.5">Billing postal code</span>
        <input bind:value={postalCode} class="input input-bordered border-white/10 bg-black/40 text-ink rounded-lg focus:border-signal shadow-inner" maxlength="12" />
      </label>

      <label class="flex items-start gap-3 text-sm cursor-pointer hover:text-white transition-colors group mb-6 bg-black/20 p-3 rounded-lg border border-white/5">
        <input bind:checked={termsAccepted} class="checkbox checkbox-primary checkbox-sm mt-0.5 rounded" type="checkbox" />
        <span class="text-inkMuted group-hover:text-ink transition-colors leading-tight">I accept the transfer, refund, and venue entry terms.</span>
      </label>

      {#if checkoutState.error}
        <div class="mb-6 flex items-start gap-2 rounded-lg border border-urgency/50 bg-urgency/10 p-3 text-sm text-urgency">
          <AlertTriangle size={18} class="shrink-0 mt-0.5" />
          <p>{checkoutState.error}</p>
        </div>
      {/if}

      <button class="btn w-full rounded-xl border-0 bg-gradient-to-r from-signal to-primary text-white font-bold text-lg shadow-glow hover:scale-[1.02] transition-all disabled:opacity-50 disabled:hover:scale-100 disabled:shadow-none" disabled={!termsAccepted || checkoutState.submitted || remaining <= 0} onclick={submit}>
        <LockKeyhole size={18} />
        {checkoutState.submitted ? 'Submitting...' : 'Complete Payment'}
      </button>
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
