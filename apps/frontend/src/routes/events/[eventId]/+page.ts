import { PUBLIC_GATEWAY_BASE_URL } from '$env/static/public';
import { createGatewayClient } from '$lib/api/client';
import { makeMockSeatSnapshot, mockDiscovery } from '$lib/api/mock';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, url }) => {
  const sectionID = url.searchParams.get('section_id') ?? 'A';
  const client = createGatewayClient(fetch, PUBLIC_GATEWAY_BASE_URL || 'http://localhost:8080');

  try {
    return {
      event: mockDiscovery.events.find((item) => item.id === params.eventId) ?? mockDiscovery.events[0],
      snapshot: await client.getSeatSnapshot(params.eventId, sectionID),
      seatSocketURL: client.seatSocketURL(params.eventId, sectionID),
      gatewayBaseURL: client.apiBase
    };
  } catch {
    return {
      event: mockDiscovery.events.find((item) => item.id === params.eventId) ?? mockDiscovery.events[0],
      snapshot: makeMockSeatSnapshot(params.eventId, sectionID),
      seatSocketURL: client.seatSocketURL(params.eventId, sectionID),
      gatewayBaseURL: client.apiBase
    };
  }
};
