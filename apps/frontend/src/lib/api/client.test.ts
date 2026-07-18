import { describe, expect, it } from 'vitest';
import { createGatewayClient } from './client';

describe('gateway discovery mapping', () => {
  it('maps zero open seats to sold out and preserves gateway categories', async () => {
    const fetcher = async () =>
      new Response(
        JSON.stringify({
          events: [
            {
              id: 'evt_sold_out',
              name: 'Sold Out Night',
              category: 'Theatre',
              venue: 'Velox Arena',
              city: 'Chicago',
              starts_at: '2026-08-15T20:00:00Z',
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
      remaining_bucket: 'SOLD_OUT'
    });
  });
});
