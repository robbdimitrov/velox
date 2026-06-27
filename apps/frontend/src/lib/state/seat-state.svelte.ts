import { SvelteSet, SvelteMap } from 'svelte/reactivity';
import type { Seat, SeatDelta } from '$lib/api/types';

export class SeatSelectionState {
  seats = $state<Seat[]>([]);
  selectedSeatIDs = new SvelteSet<string>();
  seatVersionByID = new SvelteMap<string, number>();
  serverOffsetMs = $state(0);

  selectedSeats = $derived(
    this.seats.filter((seat) => this.selectedSeatIDs.has(seat.seat_id))
  );
  selectedTotalCents = $derived(
    this.selectedSeats.reduce((sum, seat) => sum + seat.price_cents, 0)
  );

  load(seats: Seat[], serverTimeMs: number) {
    this.seats = seats;
    this.seatVersionByID.clear();
    for (const seat of seats) {
      this.seatVersionByID.set(seat.seat_id, seat.version);
    }
    this.serverOffsetMs = serverTimeMs - Date.now();
    this.selectedSeatIDs.clear();
  }

  toggleSeat(seat: Seat) {
    if (seat.status !== 'AVAILABLE' && !this.selectedSeatIDs.has(seat.seat_id))
      return;

    if (this.selectedSeatIDs.has(seat.seat_id)) {
      this.selectedSeatIDs.delete(seat.seat_id);
    } else {
      this.selectedSeatIDs.add(seat.seat_id);
    }
  }

  applyDelta(delta: SeatDelta) {
    const observedVersion = this.seatVersionByID.get(delta.seat_id) ?? 0;
    if (delta.version < observedVersion) return;

    this.seatVersionByID.set(delta.seat_id, delta.version);
    this.seats = this.seats.map((seat) =>
      seat.seat_id === delta.seat_id
        ? {
            ...seat,
            status: delta.status,
            version: delta.version,
            expires_at_server_ms: delta.expires_at_server_ms
          }
        : seat
    );

    if (delta.status !== 'AVAILABLE') {
      this.selectedSeatIDs.delete(delta.seat_id);
    }
  }

  expectedVersions() {
    return Object.fromEntries(
      this.selectedSeats.map((seat) => [
        seat.seat_id,
        this.seatVersionByID.get(seat.seat_id) ?? seat.version
      ])
    );
  }
}
