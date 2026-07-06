import { createGatewayClient } from '$lib/api/client';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params }) => {
  const client = createGatewayClient(fetch, '/api');
  const announcements = await client
    .getAnnouncements(params.eventId)
    .catch(() => []);

  return { announcements };
};
