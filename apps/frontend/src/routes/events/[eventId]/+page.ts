import { GatewayError, createGatewayClient } from '$lib/api/client';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, url }) => {
  const sectionID = url.searchParams.get('section_id') ?? 'A';
  const client = createGatewayClient(fetch, '/api');

  try {
    const discovery = await client.listEvents(new URLSearchParams());
    const event = discovery.events.find((item) => item.id === params.eventId);
    if (!event) throw new Error('Event not found');

    return {
      event,
      snapshot: await client.getSeatSnapshot(params.eventId, sectionID),
      seatSseURL: client.seatSseURL(params.eventId, sectionID),
      gatewayBaseURL: client.apiBase,
      isRateLimited: false
    };
  } catch (err) {
    if (err instanceof GatewayError && err.status === 429) {
      const discovery = await client
        .listEvents(new URLSearchParams())
        .catch(() => ({ events: [] }));
      const event = discovery.events.find((item) => item.id === params.eventId);

      return {
        event,
        snapshot: {
          event_id: params.eventId,
          section_id: sectionID,
          server_time_ms: Date.now(),
          snapshot_age_ms: 0,
          projection_lag_ms: 0,
          seats: []
        },
        seatSseURL: client.seatSseURL(params.eventId, sectionID),
        gatewayBaseURL: client.apiBase,
        isRateLimited: true
      };
    }
    throw err;
  }
};
