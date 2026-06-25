import type { Seat, SeatDelta } from '$lib/api/types';

export class SeatSelectionState {
  seats = $state<Seat[]>([]);
  selectedSeatIDs = $state<Set<string>>(new Set());
  seatVersionByID = $state<Map<string, number>>(new Map());
  serverOffsetMs = $state(0);

  selectedSeats = $derived(
    this.seats.filter((seat) => this.selectedSeatIDs.has(seat.seat_id))
  );
  selectedTotalCents = $derived(
    this.selectedSeats.reduce((sum, seat) => sum + seat.price_cents, 0)
  );

  load(seats: Seat[], serverTimeMs: number) {
    this.seats = seats;
    this.seatVersionByID = new Map(
      seats.map((seat) => [seat.seat_id, seat.version])
    );
    this.serverOffsetMs = serverTimeMs - Date.now();
    this.selectedSeatIDs.clear();
  }

  toggleSeat(seat: Seat) {
    if (seat.status !== 'AVAILABLE' && !this.selectedSeatIDs.has(seat.seat_id))
      return;

    const next = new Set(this.selectedSeatIDs);
    if (next.has(seat.seat_id)) {
      next.delete(seat.seat_id);
    } else {
      next.add(seat.seat_id);
    }
    this.selectedSeatIDs = next;
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
      const next = new Set(this.selectedSeatIDs);
      next.delete(delta.seat_id);
      this.selectedSeatIDs = next;
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
