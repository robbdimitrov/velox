import { error } from '@sveltejs/kit';
import { GatewayError, createGatewayClient } from '$lib/api/client';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, url }) => {
  const sectionID = url.searchParams.get('section_id') ?? 'A';
  const client = createGatewayClient(fetch, '/api');

  try {
    const discovery = await client.listEvents(new URLSearchParams());
    const event = discovery.events.find((item) => item.id === params.eventId);
    if (!event) throw error(404, 'Event not found');

    const [announcements, snapshot] = await Promise.all([
      client.getAnnouncements(params.eventId).catch(() => []),
      client.getSeatSnapshot(params.eventId, sectionID)
    ]);

    return {
      event,
      snapshot,
      seatSseURL: client.seatSseURL(params.eventId, sectionID),
      gatewayBaseURL: client.apiBase,
      announcements,
      isRateLimited: false
    };
  } catch (err) {
    if (err instanceof GatewayError) {
      if (err.status === 429) {
        const discovery = await client
          .listEvents(new URLSearchParams())
          .catch(() => ({ events: [] }));
        const event = discovery.events.find(
          (item) => item.id === params.eventId
        );
        const announcements = await client
          .getAnnouncements(params.eventId)
          .catch(() => []);

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
          announcements,
          isRateLimited: true
        };
      } else if (err.status === 404) {
        throw error(404, err.message);
      }
    }

    // Check if it's already a SvelteKit error (e.g. from the `throw error(404)` above)
    // SvelteKit errors have a `status` and `body` property internally, but instanceof doesn't work easily.
    // Re-throw it directly.
    throw err;
  }
};
