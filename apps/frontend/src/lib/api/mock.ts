import type {
  DiscoveryResponse,
  ReserveOrderResponse,
  Seat,
  SeatSnapshot,
  WalletResponse
} from './types';

const now = Date.now();

export const mockDiscovery: DiscoveryResponse = {
  events: [
    {
      id: 'evt_midnight-array',
      title: 'Midnight Array',
      venue: 'North Pier Hall',
      city: 'Chicago',
      category: 'Concerts',
      image_url: '/event-midnight-array.svg',
      sale_starts_at: new Date(now + 18 * 60_000).toISOString(),
      remaining_bucket: 'LOW',
      demand_score: 98,
      min_price_cents: 8800,
      projection_lag_ms: 42
    },
    {
      id: 'evt_final-whistle',
      title: 'Final Whistle Derby',
      venue: 'Civic Bowl',
      city: 'Austin',
      category: 'Sports',
      image_url: '/event-final-whistle.svg',
      sale_starts_at: new Date(now + 51 * 60_000).toISOString(),
      remaining_bucket: 'MEDIUM',
      demand_score: 91,
      min_price_cents: 12400,
      projection_lag_ms: 73
    },
    {
      id: 'evt_zero-hour',
      title: 'Zero Hour Theatre',
      venue: 'Atlas Stage',
      city: 'New York',
      category: 'Theatre',
      image_url: '/event-zero-hour.svg',
      sale_starts_at: new Date(now + 3 * 60 * 60_000).toISOString(),
      remaining_bucket: 'HIGH',
      demand_score: 84,
      min_price_cents: 6400,
      projection_lag_ms: 55
    }
  ],
  featured: [],
  meta: {
    projection_lag_ms: 73,
    cache_status: 'mock-stale-while-revalidate'
  }
};

mockDiscovery.featured = mockDiscovery.events;

export function makeMockSeatSnapshot(
  eventID = 'evt_midnight-array',
  sectionID = 'A'
): SeatSnapshot {
  const seats: Seat[] = Array.from({ length: 176 }, (_, index) => {
    const col = index % 22;
    const row = Math.floor(index / 22);
    const seatID = `${sectionID}-${row + 1}-${col + 1}`;
    const held = index % 19 === 0;
    const sold = index % 29 === 0;
    return {
      index,
      seat_id: seatID,
      section_id: sectionID,
      row: String.fromCharCode(65 + row),
      x: 28 + col * 28,
      y: 32 + row * 32,
      price_cents: 7200 + row * 500,
      accessibility: col === 0 || col === 21,
      status: sold ? 'SOLD' : held ? 'HELD' : 'AVAILABLE',
      version: sold || held ? 2 : 1,
      expires_at_server_ms: held ? now + 320_000 : undefined
    };
  });

  return {
    event_id: eventID,
    section_id: sectionID,
    server_time_ms: now,
    snapshot_age_ms: 250,
    projection_lag_ms: 81,
    seats
  };
}

export function makeMockReservation(seatIDs: string[]): ReserveOrderResponse {
  const subtotal = seatIDs.length * 8800;
  return {
    order_id: 'ord_mock_8831',
    reservation_id: 'res_mock_8831',
    reservation_token: 'mock-reservation-token',
    expires_at_server_ms: Date.now() + 8 * 60_000,
    server_time_ms: Date.now(),
    version: 4,
    seats: seatIDs.map((seat_id) => ({ seat_id, price_cents: 8800 })),
    fees_cents: Math.round(subtotal * 0.14),
    total_cents: subtotal + Math.round(subtotal * 0.14)
  };
}

export const mockWallet: WalletResponse = {
  verification_state: 'VERIFIED',
  tickets: [
    {
      ticket_id: 'velox_8831',
      event: 'Midnight Array',
      venue: 'North Pier Hall',
      seat: 'A-4-12',
      gate: 'N3',
      transfer_status: 'AVAILABLE',
      qr_token_expires_at: new Date(now + 90_000).toISOString(),
      ledger: [
        {
          event_type: 'TicketIssued',
          timestamp: new Date(now - 120_000).toISOString(),
          actor: 'orderservice',
          correlation_id: 'corr_4PMY2E'
        },
        {
          event_type: 'PaymentConfirmed',
          timestamp: new Date(now - 160_000).toISOString(),
          actor: 'payment-provider',
          correlation_id: 'corr_4PMY2E'
        }
      ]
    }
  ]
};
