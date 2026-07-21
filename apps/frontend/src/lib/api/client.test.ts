import { describe, expect, it } from 'vitest';
import { createGatewayClient, GatewayError } from './client';

describe('gateway discovery mapping', () => {
  it('maps backend metadata without event ID image overrides', async () => {
    const fetcher = async () =>
      new Response(
        JSON.stringify({
          events: [
            {
              id: 'evt_neon_riot',
              name: 'Sold Out Night',
              category: 'Theatre',
              image_key: 'event-zero-hour',
              venue: 'Velox Arena',
              city: 'Chicago',
              starts_at: '2026-08-15T20:00:00Z',
              sale_starts_at: '2026-08-01T16:00:00Z',
              section_ids: ['ORCH', 'BALC'],
              seats_open: 0,
              demand_score: 97
            }
          ],
          projection_lag_ms: 12
        }),
        { headers: { 'Content-Type': 'application/json' } }
      );

    const discovery = await createGatewayClient(
      fetcher as typeof fetch,
      '/api'
    ).listEvents(new URLSearchParams());

    expect(discovery.events[0]).toMatchObject({
      category: 'Theatre',
      image_key: 'event-zero-hour',
      image_url: '/event-zero-hour.svg',
      sale_starts_at: '2026-08-01T16:00:00Z',
      section_ids: ['ORCH', 'BALC'],
      remaining_bucket: 'SOLD_OUT'
    });
  });

  it('maps the public event detail endpoint', async () => {
    const fetcher = async (input: RequestInfo | URL) => {
      expect(String(input)).toBe('/api/events/evt_detail');
      return new Response(
        JSON.stringify({
          event: {
            id: 'evt_detail',
            name: 'Detail Night',
            description: 'Backend owned copy.',
            image_key: 'event-final-whistle',
            sale_starts_at: '2026-08-01T16:00:00Z',
            sections: [{ id: 'FLOOR', name: 'Floor' }]
          },
          projection_lag_ms: 3
        }),
        { headers: { 'Content-Type': 'application/json' } }
      );
    };

    const event = await createGatewayClient(
      fetcher as typeof fetch,
      '/api'
    ).getEvent('evt_detail');

    expect(event).toMatchObject({
      id: 'evt_detail',
      title: 'Detail Night',
      description: 'Backend owned copy.',
      image_url: '/event-final-whistle.svg',
      section_ids: ['FLOOR'],
      sections: [{ id: 'FLOOR', name: 'Floor' }],
      projection_lag_ms: 3
    });
  });

  it('uses backend seat geometry and accessibility when provided', async () => {
    const fetcher = async () =>
      new Response(
        JSON.stringify({
          seats: [
            {
              seat_id: 'A-01',
              section_id: 'A',
              row: 'A',
              number: 1,
              x: 101,
              y: 202,
              accessibility: true,
              price_cents: 8650,
              status: 'AVAILABLE',
              version: 4
            }
          ],
          server_time_ms: 123,
          snapshot_age_ms: 7,
          projection_lag_ms: 9
        }),
        { headers: { 'Content-Type': 'application/json' } }
      );

    const snapshot = await createGatewayClient(
      fetcher as typeof fetch,
      '/api'
    ).getSeatSnapshot('evt_detail', 'A');

    expect(snapshot).toMatchObject({
      server_time_ms: 123,
      snapshot_age_ms: 7,
      projection_lag_ms: 9,
      seats: [
        {
          seat_id: 'A-01',
          x: 101,
          y: 202,
          accessibility: true,
          version: 4
        }
      ]
    });
  });

  it('maps reservation token and selected prices from the backend', async () => {
    const fetcher = async (input: RequestInfo | URL, init?: RequestInit) => {
      expect(String(input)).toBe('/api/reservations');
      expect(new Headers(init?.headers).get('Idempotency-Key')).toBe(
        'idem-reserve'
      );
      return new Response(
        JSON.stringify({
          order: {
            id: 'ord_1',
            reservation_id: 'res_ord_1',
            reservation_token: 'signed-token',
            event_id: 'evt_detail',
            section_id: 'A',
            seat_ids: ['A-01'],
            seats: [{ seat_id: 'A-01', price_cents: 8650 }],
            status: 'PENDING',
            total_cents: 8650,
            fees_cents: 0,
            expires_at_server_ms: 1760000000000,
            server_time_ms: 1759999700000
          }
        }),
        { headers: { 'Content-Type': 'application/json' } }
      );
    };

    const reservation = await createGatewayClient(
      fetcher as typeof fetch,
      '/api'
    ).reserveSeats(
      {
        event_id: 'evt_detail',
        section_id: 'A',
        seat_ids: ['A-01'],
        expected_versions: { 'A-01': 1 }
      },
      'idem-reserve'
    );

    expect(reservation).toMatchObject({
      order_id: 'ord_1',
      reservation_id: 'res_ord_1',
      reservation_token: 'signed-token',
      server_time_ms: 1759999700000,
      seats: [{ seat_id: 'A-01', price_cents: 8650 }],
      total_cents: 8650
    });
    expect(reservation.reservation_token).not.toBe(reservation.reservation_id);
  });

  it('rejects reservation responses without a backend token', async () => {
    const fetcher = async () =>
      new Response(
        JSON.stringify({
          order: {
            id: 'ord_1',
            reservation_id: 'res_ord_1',
            event_id: 'evt_detail',
            section_id: 'A',
            seat_ids: ['A-01'],
            status: 'PENDING',
            total_cents: 8650
          }
        }),
        { headers: { 'Content-Type': 'application/json' } }
      );

    await expect(
      createGatewayClient(fetcher as typeof fetch, '/api').reserveSeats(
        {
          event_id: 'evt_detail',
          section_id: 'A',
          seat_ids: ['A-01'],
          expected_versions: { 'A-01': 1 }
        },
        'idem-reserve'
      )
    ).rejects.toMatchObject({ code: 'upstream_error' });
  });

  it('sends reservation token and maps checkout ticket IDs', async () => {
    const fetcher = async (input: RequestInfo | URL, init?: RequestInit) => {
      expect(String(input)).toBe('/api/reservations/res_ord_1/confirm');
      const headers = new Headers(init?.headers);
      expect(headers.get('Idempotency-Key')).toBe('idem-confirm');
      expect(headers.get('Reservation-Token')).toBe('signed-token');
      return new Response(
        JSON.stringify({
          order_id: 'ord_1',
          status: 'CONFIRMED',
          wallet_ticket_ids: ['tkt_1']
        }),
        { headers: { 'Content-Type': 'application/json' } }
      );
    };

    const checkout = await createGatewayClient(
      fetcher as typeof fetch,
      '/api'
    ).checkout(
      { reservation_id: 'res_ord_1', terms_accepted: true },
      'idem-confirm',
      'signed-token'
    );

    expect(checkout).toEqual({
      order_id: 'ord_1',
      status: 'CONFIRMED',
      wallet_ticket_ids: ['tkt_1']
    });
  });

  it('does not synthesize missing checkout ticket IDs', async () => {
    const fetcher = async () =>
      new Response(JSON.stringify({ order_id: 'ord_1', status: 'CONFIRMED' }), {
        headers: { 'Content-Type': 'application/json' }
      });

    await expect(
      createGatewayClient(fetcher as typeof fetch, '/api').checkout(
        { reservation_id: 'res_ord_1', terms_accepted: true },
        'idem-confirm',
        'signed-token'
      )
    ).rejects.toBeInstanceOf(GatewayError);
  });
});
