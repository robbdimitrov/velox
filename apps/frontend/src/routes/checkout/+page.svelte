<script lang="ts">
  import { goto } from '$app/navigation';
  import {
    createGatewayClient,
    createIdempotencyKey,
    GatewayError
  } from '$lib/api/client';
  import {
    checkoutState,
    formatCountdown
  } from '$lib/state/checkout-state.svelte';
  import {
    LockKeyhole,
    AlertTriangle,
    CheckCircle2,
    XCircle
  } from '@lucide/svelte';
  import ActionLink from '$lib/components/ActionLink.svelte';
  import Panel from '$lib/components/Panel.svelte';
  import PrimaryButton from '$lib/components/PrimaryButton.svelte';

  let termsAccepted = $state(false);
  let cancelling = $state(false);
  let tick = $state(Date.now());

  $effect(() => {
    const timer = setInterval(() => {
      tick = Date.now();
      if (checkoutState.reservation && remaining <= 0) {
        checkoutState.clear();
      }
    }, 1000);
    return () => clearInterval(timer);
  });

  const remaining = $derived.by(() => {
    if (!checkoutState.reservation) return 0;
    return Math.max(
      0,
      checkoutState.reservation.expires_at_server_ms -
        (tick + checkoutState.serverOffsetMs)
    );
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
        'Reservation could not be confirmed. The idempotency key prevents duplicate requests.';
      checkoutState.submitted = false;
    }
  }

  async function cancel() {
    if (!checkoutState.reservation || checkoutState.submitted || cancelling)
      return;
    cancelling = true;
    checkoutState.error = '';

    try {
      const client = createGatewayClient(fetch, '/api');
      await client.cancelReservation(
        checkoutState.reservation.reservation_id,
        createIdempotencyKey(),
        checkoutState.reservation.reservation_token
      );
      checkoutState.clear();
      await goto('/');
    } catch (err) {
      // A 409 means the order already settled elsewhere (confirmed in
      // another tab, or a retried request) — retrying cancel can never
      // succeed for it, so tell the user that plainly instead of "try again".
      if (err instanceof GatewayError && err.status === 409) {
        checkoutState.error =
          'This reservation was already confirmed or cancelled elsewhere.';
      } else {
        checkoutState.error = 'Reservation could not be cancelled. Try again.';
      }
      cancelling = false;
    }
  }
</script>

<main class="mx-auto grid w-full max-w-5xl gap-6 lg:grid-cols-[1fr_360px]">
  {#if checkoutState.reservation}
    <Panel padding="xl" flexColumn>
      <h1 class="text-3xl font-black uppercase tracking-tight text-white">
        Confirm Reservation
      </h1>

      <div
        class="my-6 rounded border border-urgency/20 bg-urgency/5 py-6 text-center shadow-inner relative overflow-hidden shrink-0"
      >
        <div
          class="absolute inset-0 bg-[radial-gradient(ellipse_at_center,_var(--tw-gradient-stops))] from-urgency/10 to-transparent blur-xl"
        ></div>
        <p
          class="text-xs font-bold uppercase tracking-widest text-urgency mb-1 relative"
        >
          Hold expires in
        </p>
        <p
          class="font-mono tabular-nums relative text-5xl font-black text-urgency drop-shadow-[0_0_10px_rgba(239,68,68,0.8)]"
        >
          {formatCountdown(remaining)}
        </p>
      </div>

      <div class="flex-grow">
        <h3
          class="text-sm font-black uppercase tracking-widest text-inkMuted mb-4"
        >
          Secured Tickets
        </h3>
        <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
          {#each checkoutState.reservation.seats as seat}
            <div
              class="flex items-center justify-center rounded-sm border border-line bg-panelSoft/70 p-4 font-mono tabular-nums shadow-sm transition-colors hover:border-signal/30"
            >
              <span class="text-xl font-bold text-ink">{seat.seat_id}</span>
            </div>
          {/each}
        </div>
      </div>

      <div
        class="mt-6 pt-6 border-t border-white/10 flex justify-between items-center text-xs text-ink/40"
      >
        <span class="uppercase tracking-widest">Reservation version</span>
        <span class="font-mono tabular-nums"
          >{checkoutState.reservation.version}</span
        >
      </div>
    </Panel>

    <Panel padding="lg" sticky hMax>
      <div class="flex items-center gap-3 border-b border-white/10 pb-4 mb-6">
        <div class="rounded bg-signal p-2 shadow-md shadow-signal/20">
          <CheckCircle2 class="text-carbon" size={20} />
        </div>
        <h2 class="text-lg font-black uppercase tracking-wider text-white">
          Review & Complete
        </h2>
      </div>

      <label
        class="flex items-start gap-3 text-sm cursor-pointer hover:text-white transition-colors group mb-6 bg-black/20 p-4 rounded border border-white/5"
      >
        <input
          bind:checked={termsAccepted}
          class="checkbox checkbox-primary checkbox-sm mt-0.5 rounded"
          type="checkbox"
        />
        <span
          class="text-inkMuted group-hover:text-ink transition-colors leading-tight"
          >I accept the transfer, refund, and venue entry terms.</span
        >
      </label>

      {#if checkoutState.error}
        <div
          class="mb-6 flex items-start gap-2 rounded border border-urgency/50 bg-urgency/10 p-3 text-sm text-urgency"
        >
          <AlertTriangle size={18} class="shrink-0 mt-0.5" />
          <p>{checkoutState.error}</p>
        </div>
      {/if}

      <PrimaryButton
        disabled={!termsAccepted || checkoutState.submitted || remaining <= 0}
        onclick={submit}
      >
        <LockKeyhole size={18} />
        {checkoutState.submitted ? 'Securing...' : 'Complete Reservation'}
      </PrimaryButton>

      <button
        class="btn btn-ghost btn-block mt-3 rounded border border-white/10 text-inkMuted hover:text-urgency hover:border-urgency/40"
        disabled={checkoutState.submitted || cancelling}
        onclick={cancel}
      >
        <XCircle size={18} />
        {cancelling ? 'Cancelling...' : 'Cancel Reservation'}
      </button>
    </Panel>
  {:else}
    <div class="col-span-full mx-auto w-full max-w-2xl">
      <Panel padding="xl">
        <div
          class="flex min-h-[400px] flex-col items-center justify-center text-center"
        >
          <div class="mb-6 rounded bg-white/5 p-4 text-signal">
            <AlertTriangle size={48} />
          </div>
          <h1 class="text-3xl font-black uppercase text-white mb-3">
            No Active Reservation
          </h1>
          <p class="text-inkMuted text-lg max-w-md">
            Your hold has expired or you haven't selected any seats yet. Return
            to the seat map and secure your tickets.
          </p>
          <ActionLink href="/" size="lg">Find Events</ActionLink>
        </div>
      </Panel>
    </div>
  {/if}
</main>
