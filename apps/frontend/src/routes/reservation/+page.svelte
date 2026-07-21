<script lang="ts">
  import { goto } from '$app/navigation';
  import {
    createGatewayClient,
    createIdempotencyKey,
    GatewayError
  } from '$lib/api/client';
  import {
    reservationState,
    formatCountdown
  } from '$lib/state/reservation-state.svelte';
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
      if (reservationState.reservation && remaining <= 0) {
        reservationState.clear();
      }
    }, 1000);
    return () => clearInterval(timer);
  });

  const remaining = $derived.by(() => {
    if (!reservationState.reservation) return 0;
    return Math.max(
      0,
      reservationState.reservation.expires_at_server_ms -
        (tick + reservationState.serverOffsetMs)
    );
  });

  async function submit() {
    if (
      !reservationState.reservation ||
      reservationState.submitted ||
      !termsAccepted
    )
      return;
    reservationState.submitted = true;
    reservationState.error = '';

    try {
      const client = createGatewayClient(fetch, '/api');
      const result = await client.confirmReservation(
        {
          reservation_id: reservationState.reservation.reservation_id,
          terms_accepted: termsAccepted
        },
        createIdempotencyKey(),
        reservationState.reservation.reservation_token
      );

      if (result.status === 'CONFIRMED') {
        reservationState.clear();
        await goto('/wallet');
      } else {
        reservationState.error = `Reservation ${result.status.toLowerCase()}`;
      }
    } catch (err) {
      reservationState.error = reservationErrorMessage(err, 'confirm');
      reservationState.submitted = false;
    }
  }

  async function cancel() {
    if (
      !reservationState.reservation ||
      reservationState.submitted ||
      cancelling
    )
      return;
    cancelling = true;
    reservationState.error = '';

    try {
      const client = createGatewayClient(fetch, '/api');
      await client.cancelReservation(
        reservationState.reservation.reservation_id,
        createIdempotencyKey(),
        reservationState.reservation.reservation_token
      );
      reservationState.clear();
      await goto('/');
    } catch (err) {
      reservationState.error = reservationErrorMessage(err, 'cancel');
      cancelling = false;
    }
  }

  function reservationErrorMessage(err: unknown, action: 'confirm' | 'cancel') {
    if (!(err instanceof GatewayError)) {
      return action === 'confirm'
        ? 'Reservation could not be confirmed. Try again.'
        : 'Reservation could not be cancelled. Try again.';
    }
    if (
      err.code === 'reservation_expired' ||
      err.code === 'reservation_token_expired'
    ) {
      reservationState.clear();
      return 'This reservation hold expired.';
    }
    if (err.code === 'reservation_token_invalid') {
      reservationState.clear();
      return 'This reservation can no longer be verified.';
    }
    if (err.code === 'seat_not_available') {
      reservationState.clear();
      return 'One or more selected seats are no longer available.';
    }
    if (err.code === 'event_not_bookable') {
      reservationState.clear();
      return 'This event is no longer bookable.';
    }
    if (err.code === 'idempotency_key_conflict') {
      return 'This action was already submitted with different reservation details.';
    }
    if (err.code === 'rate_limited' || err.status === 429) {
      return 'Too many reservation attempts. Wait a moment and try again.';
    }
    if (err.status === 409) {
      return 'This reservation was already confirmed or cancelled elsewhere.';
    }
    return action === 'confirm'
      ? 'Reservation could not be confirmed. Try again.'
      : 'Reservation could not be cancelled. Try again.';
  }
</script>

<main class="mx-auto grid w-full max-w-5xl gap-6 lg:grid-cols-[1fr_360px]">
  {#if reservationState.reservation}
    <Panel padding="xl" flexColumn>
      <h1 class="text-3xl font-black uppercase tracking-tight text-ink">
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
          Reservation Tickets
        </h3>
        <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
          {#each reservationState.reservation.seats as seat}
            <div
              class="flex items-center justify-center rounded-sm border border-line bg-panelSoft/70 p-4 font-mono tabular-nums shadow-sm transition-colors hover:border-signal/30"
            >
              <span class="text-xl font-bold text-ink">{seat.seat_id}</span>
            </div>
          {/each}
        </div>
      </div>

      <div
        class="mt-6 flex items-center justify-between border-t border-line pt-6 text-xs text-inkMuted"
      >
        <span class="uppercase tracking-widest">Reservation version</span>
        <span class="font-mono tabular-nums"
          >{reservationState.reservation.version}</span
        >
      </div>
    </Panel>

    <Panel padding="lg" sticky hMax>
      <div class="mb-6 flex items-center gap-3 border-b border-line pb-4">
        <div class="rounded bg-signal p-2 shadow-md shadow-signal/20">
          <CheckCircle2 class="text-primary-content" size={20} />
        </div>
        <h2 class="text-lg font-black uppercase tracking-wider text-ink">
          Review & Complete
        </h2>
      </div>

      <label
        class="group mb-6 flex cursor-pointer items-start gap-3 rounded-sm border border-line bg-panelSoft/70 p-4 text-sm transition-colors hover:border-signal/40"
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

      {#if reservationState.error}
        <div
          class="mb-6 flex items-start gap-2 rounded border border-urgency/50 bg-urgency/10 p-3 text-sm text-urgency"
        >
          <AlertTriangle size={18} class="shrink-0 mt-0.5" />
          <p>{reservationState.error}</p>
        </div>
      {/if}

      <PrimaryButton
        disabled={!termsAccepted ||
          reservationState.submitted ||
          remaining <= 0}
        onclick={submit}
      >
        <LockKeyhole size={18} />
        {reservationState.submitted ? 'Confirming...' : 'Confirm reservation'}
      </PrimaryButton>

      <button
        class="btn btn-ghost btn-block mt-3 rounded-sm border border-line text-inkMuted hover:border-urgency/40 hover:text-urgency"
        disabled={reservationState.submitted || cancelling}
        onclick={cancel}
      >
        <XCircle size={18} />
        {cancelling ? 'Cancelling...' : 'Cancel reservation'}
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
          <h1 class="mb-3 text-3xl font-black uppercase text-ink">
            No Active Reservation
          </h1>
          <p class="text-inkMuted text-lg max-w-md">
            Your hold has expired or you haven't selected any seats yet. Return
            to the seat map and reserve seats.
          </p>
          <ActionLink href="/" size="lg">Find Events</ActionLink>
        </div>
      </Panel>
    </div>
  {/if}
</main>
