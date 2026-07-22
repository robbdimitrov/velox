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
      <h1 class="text-ink text-3xl font-black tracking-tight uppercase">
        Confirm Reservation
      </h1>

      <div
        class="border-urgency/20 bg-urgency/5 relative my-6 shrink-0 overflow-hidden rounded border py-6 text-center shadow-inner"
      >
        <div
          class="from-urgency/10 absolute inset-0 bg-[radial-gradient(ellipse_at_center,_var(--tw-gradient-stops))] to-transparent blur-xl"
        ></div>
        <p
          class="text-urgency relative mb-1 text-xs font-bold tracking-widest uppercase"
        >
          Hold expires in
        </p>
        <p
          class="text-urgency relative font-mono text-5xl font-black tabular-nums drop-shadow-[0_0_10px_rgba(239,68,68,0.8)]"
        >
          {formatCountdown(remaining)}
        </p>
      </div>

      <div class="flex-grow">
        <h3
          class="text-inkMuted mb-4 text-sm font-black tracking-widest uppercase"
        >
          Reservation Tickets
        </h3>
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
          {#each reservationState.reservation.seats as seat}
            <div
              class="border-line bg-panelSoft/70 hover:border-signal/30 flex items-center justify-center rounded-sm border p-4 font-mono tabular-nums shadow-sm transition-colors"
            >
              <span class="text-ink text-xl font-bold">{seat.seat_id}</span>
            </div>
          {/each}
        </div>
      </div>

      <div
        class="border-line text-inkMuted mt-6 flex items-center justify-between border-t pt-6 text-xs"
      >
        <span class="tracking-widest uppercase">Reservation version</span>
        <span class="font-mono tabular-nums"
          >{reservationState.reservation.version}</span
        >
      </div>
    </Panel>

    <Panel padding="lg" sticky hMax>
      <div class="border-line mb-6 flex items-center gap-3 border-b pb-4">
        <div class="bg-signal shadow-signal/20 rounded p-2 shadow-md">
          <CheckCircle2 class="text-primary-content" size={20} />
        </div>
        <h2 class="text-ink text-lg font-black tracking-wider uppercase">
          Review & Complete
        </h2>
      </div>

      <label
        class="group border-line bg-panelSoft/70 hover:border-signal/40 mb-6 flex cursor-pointer items-start gap-3 rounded-sm border p-4 text-sm transition-colors"
      >
        <input
          bind:checked={termsAccepted}
          class="checkbox checkbox-primary checkbox-sm mt-0.5 rounded"
          type="checkbox"
        />
        <span
          class="text-inkMuted group-hover:text-ink leading-tight transition-colors"
          >I accept the transfer, refund, and venue entry terms.</span
        >
      </label>

      {#if reservationState.error}
        <div
          class="border-urgency/50 bg-urgency/10 text-urgency mb-6 flex items-start gap-2 rounded border p-3 text-sm"
        >
          <AlertTriangle size={18} class="mt-0.5 shrink-0" />
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
        class="btn btn-ghost btn-block border-line text-inkMuted hover:border-urgency/40 hover:text-urgency mt-3 rounded-sm border"
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
          <div class="text-signal mb-6 rounded bg-white/5 p-4">
            <AlertTriangle size={48} />
          </div>
          <h1 class="text-ink mb-3 text-3xl font-black uppercase">
            No Active Reservation
          </h1>
          <p class="text-inkMuted max-w-md text-lg">
            Your hold has expired or you haven't selected any seats yet. Return
            to the seat map and reserve seats.
          </p>
          <ActionLink href="/" size="lg">Find Events</ActionLink>
        </div>
      </Panel>
    </div>
  {/if}
</main>
