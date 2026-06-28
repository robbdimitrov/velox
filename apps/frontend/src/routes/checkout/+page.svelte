<script lang="ts">
  import { goto } from '$app/navigation';
  import { env } from '$env/dynamic/public';
  import {
    createGatewayClient,
    createIdempotencyKey,
    formatMoney
  } from '$lib/api/client';
  import {
    checkoutState,
    formatCountdown
  } from '$lib/state/checkout-state.svelte';
  import { CreditCard, LockKeyhole } from '@lucide/svelte';

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
    if (!checkoutState.reservation || checkoutState.submitted || !termsAccepted)
      return;
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
      checkoutState.error =
        'Payment could not be confirmed. The idempotency key prevents duplicate charge attempts.';
      checkoutState.submitted = false;
    }
  }
</script>

<main class="mx-auto grid max-w-5xl gap-4 px-4 py-5 lg:grid-cols-[1fr_360px]">
  {#if checkoutState.reservation}
    <section class="glass-panel rounded-2xl p-4">
      <h1 class="text-2xl font-black uppercase">Checkout</h1>
      <div class="my-4 border-y border-line py-4 text-center">
        <p class="text-xs uppercase text-ink/50">Reservation expires</p>
        <p class="mono-num font-mono text-5xl font-black text-urgency">
          {formatCountdown(remaining)}
        </p>
      </div>
      <div class="space-y-2">
        {#each checkoutState.reservation.seats as seat}
          <div
            class="flex justify-between border border-line bg-carbon p-3 font-mono"
          >
            <span>{seat.seat_id}</span>
            <span>{formatMoney(seat.price_cents)}</span>
          </div>
        {/each}
      </div>
      <dl class="mt-4 space-y-2 border-t border-line pt-4 font-mono">
        <div class="flex justify-between">
          <dt>Fees</dt>
          <dd>{formatMoney(checkoutState.reservation.fees_cents)}</dd>
        </div>
        <div class="flex justify-between text-xl font-black">
          <dt>Total</dt>
          <dd>{formatMoney(checkoutState.reservation.total_cents)}</dd>
        </div>
        <div class="flex justify-between text-xs text-ink/50">
          <dt>Reservation version</dt>
          <dd>{checkoutState.reservation.version}</dd>
        </div>
      </dl>
    </section>

    <aside class="glass-panel rounded-2xl h-max p-4">
      <div class="flex items-center gap-2 border-b border-line pb-3">
        <CreditCard class="text-signal" size={18} />
        <h2 class="text-sm font-black uppercase">Payment</h2>
      </div>
      <label class="form-control mt-4">
        <span class="label-text text-ink/70">Payment method token</span>
        <input
          bind:value={paymentToken}
          class="input input-bordered border-line bg-carbon"
        />
      </label>
      <label class="form-control mt-3">
        <span class="label-text text-ink/70">Billing postal code</span>
        <input
          bind:value={postalCode}
          class="input input-bordered border-line bg-carbon"
          maxlength="12"
        />
      </label>
      <label class="mt-4 flex items-start gap-3 text-sm">
        <input
          bind:checked={termsAccepted}
          class="checkbox checkbox-primary checkbox-sm mt-1"
          type="checkbox"
        />
        I accept the transfer, refund, and venue entry terms.
      </label>
      {#if checkoutState.error}
        <p class="mt-4 border border-urgency px-2 py-1 text-sm text-urgency">
          {checkoutState.error}
        </p>
      {/if}
      <button
        class="btn btn-primary mt-4 w-full"
        disabled={!termsAccepted || checkoutState.submitted || remaining <= 0}
        onclick={submit}
      >
        <LockKeyhole size={17} />
        {checkoutState.submitted ? 'Submitting' : 'Pay'}
      </button>
    </aside>
  {:else}
    <section class="glass-panel rounded-2xl p-6">
      <h1 class="text-xl font-black uppercase">No Active Reservation</h1>
      <p class="mt-2 text-ink/60">
        Return to the seat map and hold seats before checkout.
      </p>
      <a class="btn btn-primary mt-4" href="/">Find events</a>
    </section>
  {/if}
</main>
