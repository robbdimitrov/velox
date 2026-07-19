import { describe, expect, it } from 'vitest';
import { createGatewayClient } from './client';

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
});
