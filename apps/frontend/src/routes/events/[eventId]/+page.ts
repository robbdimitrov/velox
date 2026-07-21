import { error } from '@sveltejs/kit';
import { GatewayError, createGatewayClient } from '$lib/api/client';
import type {
  EventAnnouncement,
  EventSummary,
  SeatSnapshot
} from '$lib/api/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, url }) => {
  const client = createGatewayClient(fetch, '/api');
  const loadErrors: string[] = [];

  let event: EventSummary | undefined;
  try {
    event = await client.getEvent(params.eventId);
  } catch (err) {
    if (err instanceof GatewayError && err.status === 404) {
      throw error(404, err.message);
    }
    loadErrors.push(`Event detail unavailable: ${messageFromError(err)}`);
  }

  if (!event || needsDiscoveryBackfill(event)) {
    try {
      const discovery = await client.listEvents(new URLSearchParams());
      const discoveredEvent = discovery.events.find(
        (item) => item.id === params.eventId
      );
      event = event
        ? mergeEventBackfill(event, discoveredEvent)
        : discoveredEvent;
    } catch (err) {
      loadErrors.push(
        `Discovery backfill unavailable: ${messageFromError(err)}`
      );
    }
  }

  if (!event) throw error(404, 'Event not found');

  const sectionID =
    url.searchParams.get('section_id') ?? event.section_ids?.[0] ?? 'A';
  let announcements: EventAnnouncement[] = [];
  try {
    announcements = await client.getAnnouncements(params.eventId);
  } catch (err) {
    loadErrors.push(`Event updates unavailable: ${messageFromError(err)}`);
  }

  try {
    const snapshot = await client.getSeatSnapshot(params.eventId, sectionID);
    return {
      event,
      snapshot,
      sections: sectionOptions(event, snapshot),
      seatSseURL: client.seatSseURL(params.eventId, sectionID),
      gatewayBaseURL: client.apiBase,
      announcements,
      loadErrors,
      isRateLimited: false
    };
  } catch (err) {
    if (err instanceof GatewayError) {
      if (err.status === 429) {
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
          sections: sectionOptions(event),
          seatSseURL: client.seatSseURL(params.eventId, sectionID),
          gatewayBaseURL: client.apiBase,
          announcements,
          loadErrors: [
            ...loadErrors,
            `Seat inventory unavailable: ${messageFromError(err)}`
          ],
          isRateLimited: true
        };
      } else if (err.status === 404) {
        throw error(404, err.message);
      }
    }

    // Preserve SvelteKit errors thrown above; instanceof is unreliable here.
    throw err;
  }
};

function needsDiscoveryBackfill(event: EventSummary) {
  return (
    !event.starts_at ||
    !event.city ||
    event.venue === 'Venue pending' ||
    !event.section_ids?.length
  );
}

function mergeEventBackfill(
  event: EventSummary,
  fallback: EventSummary | undefined
): EventSummary {
  if (!fallback) return event;
  return {
    ...fallback,
    ...event,
    venue: event.venue === 'Venue pending' ? fallback.venue : event.venue,
    city: event.city || fallback.city,
    category: event.category === 'Live' ? fallback.category : event.category,
    starts_at: event.starts_at || fallback.starts_at,
    section_ids: event.section_ids?.length
      ? event.section_ids
      : fallback.section_ids,
    sections: event.sections?.length ? event.sections : fallback.sections,
    demand_score: event.demand_score || fallback.demand_score,
    projection_lag_ms: Math.max(
      event.projection_lag_ms,
      fallback.projection_lag_ms
    )
  };
}

function sectionOptions(event: EventSummary, snapshot?: SeatSnapshot) {
  const options = event.sections?.length
    ? event.sections
    : event.section_ids?.map((id) => ({ id, name: id }));
  if (options?.length) return options;
  if (snapshot?.section_id) {
    return [{ id: snapshot.section_id, name: snapshot.section_id }];
  }
  return [{ id: 'A', name: 'A' }];
}

function messageFromError(err: unknown) {
  if (err instanceof GatewayError) return err.code ?? err.message;
  if (err instanceof Error) return err.message;
  return 'request failed';
}
