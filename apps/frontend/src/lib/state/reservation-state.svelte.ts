import type { ReserveOrderResponse } from '$lib/api/types';

export class ReservationState {
  reservation = $state<ReserveOrderResponse | null>(null);
  submitted = $state(false);
  error = $state('');
  serverOffsetMs = $state(0);

  msRemaining = $derived.by(() => {
    if (!this.reservation) return 0;
    return Math.max(
      0,
      this.reservation.expires_at_server_ms - (Date.now() + this.serverOffsetMs)
    );
  });

  load(reservation: ReserveOrderResponse) {
    this.reservation = reservation;
    this.submitted = false;
    this.error = '';
    this.serverOffsetMs = reservation.server_time_ms - Date.now();
  }

  clear() {
    this.reservation = null;
    this.submitted = false;
  }
}

export const reservationState = new ReservationState();

export function formatCountdown(ms: number) {
  const totalSeconds = Math.ceil(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60)
    .toString()
    .padStart(2, '0');
  const seconds = (totalSeconds % 60).toString().padStart(2, '0');
  return `${minutes}:${seconds}`;
}
