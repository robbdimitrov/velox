import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { CheckoutState, formatCountdown } from './checkout-state.svelte';
import type { ReserveOrderResponse } from '$lib/api/types';

describe('CheckoutState', () => {
  let state: CheckoutState;

  beforeEach(() => {
    state = new CheckoutState();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  const mockReservation: ReserveOrderResponse = {
    order_id: 'O1',
    reservation_id: 'R1',
    reservation_token: 'T1',
    expires_at_server_ms: 10000,
    server_time_ms: 5000,
    version: 1,
    seats: [],
    fees_cents: 100,
    total_cents: 1000
  };

  it('initializes correctly', () => {
    expect(state.reservation).toBeNull();
    expect(state.submitted).toBe(false);
    expect(state.error).toBe('');
    expect(state.msRemaining).toBe(0);
  });

  it('loads reservation and calculates remaining ms', () => {
    vi.setSystemTime(5000); // Local time is 5000, server time is 5000 => offset 0
    state.load(mockReservation);

    expect(state.reservation).toEqual(mockReservation);
    expect(state.serverOffsetMs).toBe(0);
    expect(state.msRemaining).toBe(5000); // 10000 - (5000 + 0)

    vi.setSystemTime(7000);
    expect(state.msRemaining).toBe(3000); // 10000 - (7000 + 0)
  });

  it('clears state', () => {
    vi.setSystemTime(5000);
    state.load(mockReservation);
    state.submitted = true;

    state.clear();
    expect(state.reservation).toBeNull();
    expect(state.submitted).toBe(false);
  });
});

describe('formatCountdown', () => {
  it('formats ms to mm:ss', () => {
    expect(formatCountdown(0)).toBe('00:00');
    expect(formatCountdown(1000)).toBe('00:01');
    expect(formatCountdown(59000)).toBe('00:59');
    expect(formatCountdown(60000)).toBe('01:00');
    expect(formatCountdown(61000)).toBe('01:01');
    expect(formatCountdown(3599000)).toBe('59:59');
  });
});
