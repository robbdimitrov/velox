import { createGatewayClient } from '$lib/api/client';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url }) => {
  const client = createGatewayClient(fetch, '/api');
  const params = new URLSearchParams(url.searchParams);
  if (!params.has('city')) params.set('city', 'all');

  return {
    discovery: await client.listEvents(params),
    tickerURL: client.tickerURL(params),
    gatewayBaseURL: client.apiBase
  };
};
