import { describe, it, expect, beforeEach } from 'vitest';
import { SeatSelectionState } from './seat-state.svelte';
import type { Seat, SeatDelta } from '$lib/api/types';

describe('SeatSelectionState', () => {
  let state: SeatSelectionState;

  beforeEach(() => {
    state = new SeatSelectionState();
  });

  const mockSeats: Seat[] = [
    {
      seat_id: '1',
      x: 10,
      y: 10,
      status: 'AVAILABLE',
      price_cents: 1000,
      version: 1,
      expires_at_server_ms: 0,
      index: 1,
      section_id: 'S1',
      row: 'A',
      accessibility: false
    },
    {
      seat_id: '2',
      x: 20,
      y: 10,
      status: 'HELD',
      price_cents: 1500,
      version: 2,
      expires_at_server_ms: 0,
      index: 2,
      section_id: 'S1',
      row: 'A',
      accessibility: false
    }
  ];

  it('loads seats correctly', () => {
    state.load(mockSeats, Date.now() + 5000);
    expect(state.seats).toEqual(mockSeats);
    expect(state.seatVersionByID.get('1')).toBe(1);
    expect(state.seatVersionByID.get('2')).toBe(2);
    expect(state.selectedSeatIDs.size).toBe(0);
  });

  it('toggles available seat', () => {
    state.load(mockSeats, Date.now() + 5000);
    state.toggleSeat(mockSeats[0]);
    expect(state.selectedSeatIDs.has('1')).toBe(true);
    expect(state.selectedSeats.length).toBe(1);
    expect(state.selectedTotalCents).toBe(1000);

    state.toggleSeat(mockSeats[0]);
    expect(state.selectedSeatIDs.has('1')).toBe(false);
    expect(state.selectedSeats.length).toBe(0);
    expect(state.selectedTotalCents).toBe(0);
  });

  it('does not toggle unavailable seat unless already selected', () => {
    state.load(mockSeats, Date.now() + 5000);
    state.toggleSeat(mockSeats[1]); // HELD
    expect(state.selectedSeatIDs.has('2')).toBe(false);
  });

  it('applies delta and removes from selected if no longer available', () => {
    state.load(mockSeats, Date.now() + 5000);
    state.toggleSeat(mockSeats[0]); // select seat 1
    expect(state.selectedSeatIDs.has('1')).toBe(true);

    const delta: SeatDelta = {
      event_id: 'E1',
      section_id: 'S1',
      seat_id: '1',
      status: 'HELD',
      version: 2,
      expires_at_server_ms: Date.now() + 10000
    };

    state.applyDelta(delta);

    // Seat 1 is now HELD
    expect(state.seats.find((s) => s.seat_id === '1')?.status).toBe('HELD');
    expect(state.seatVersionByID.get('1')).toBe(2);
    // It should have been removed from selected
    expect(state.selectedSeatIDs.has('1')).toBe(false);
  });

  it('ignores older deltas', () => {
    state.load(mockSeats, Date.now() + 5000); // seat 1 is version 1

    const delta: SeatDelta = {
      event_id: 'E1',
      section_id: 'S1',
      seat_id: '1',
      status: 'HELD',
      version: 0, // older version
      expires_at_server_ms: 0
    };

    state.applyDelta(delta);

    // Seat 1 should still be AVAILABLE
    expect(state.seats.find((s) => s.seat_id === '1')?.status).toBe(
      'AVAILABLE'
    );
    expect(state.seatVersionByID.get('1')).toBe(1);
  });

  it('tracks unknown-seat versions without rebuilding seats', () => {
    state.load(mockSeats, Date.now() + 5000);
    const currentSeats = state.seats;

    state.applyDelta({
      event_id: 'E1',
      section_id: 'S1',
      seat_id: 'missing',
      status: 'HELD',
      version: 1,
      expires_at_server_ms: 0
    });

    expect(state.seats).toBe(currentSeats);
    expect(state.seatVersionByID.get('missing')).toBe(1);
  });
});
