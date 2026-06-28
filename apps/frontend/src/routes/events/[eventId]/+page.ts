import { GatewayError, createGatewayClient } from '$lib/api/client';
import { makeMockSeatSnapshot, mockDiscovery } from '$lib/api/mock';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, url }) => {
  const sectionID = url.searchParams.get('section_id') ?? 'A';
  const client = createGatewayClient(fetch, '/api');

  try {
    return {
      event:
        mockDiscovery.events.find((item) => item.id === params.eventId) ??
        mockDiscovery.events[0],
      snapshot: await client.getSeatSnapshot(params.eventId, sectionID),
      seatSseURL: client.seatSseURL(params.eventId, sectionID),
      gatewayBaseURL: client.apiBase,
      isRateLimited: false
    };
  } catch (err) {
    if (err instanceof GatewayError && err.status === 429) {
      return {
        event:
          mockDiscovery.events.find((item) => item.id === params.eventId) ??
          mockDiscovery.events[0],
        snapshot: makeMockSeatSnapshot(params.eventId, sectionID),
        seatSseURL: client.seatSseURL(params.eventId, sectionID),
        gatewayBaseURL: client.apiBase,
        isRateLimited: true
      };
    }
    return {
      event:
        mockDiscovery.events.find((item) => item.id === params.eventId) ??
        mockDiscovery.events[0],
      snapshot: makeMockSeatSnapshot(params.eventId, sectionID),
      seatSseURL: client.seatSseURL(params.eventId, sectionID),
      gatewayBaseURL: client.apiBase,
      isRateLimited: false
    };
  }
};
