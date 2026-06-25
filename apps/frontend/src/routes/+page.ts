import { PUBLIC_GATEWAY_BASE_URL } from '$env/static/public';
import { createGatewayClient } from '$lib/api/client';
import { mockDiscovery } from '$lib/api/mock';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url }) => {
  const client = createGatewayClient(fetch, PUBLIC_GATEWAY_BASE_URL || 'http://localhost:8080');
  const params = new URLSearchParams(url.searchParams);
  if (!params.has('city')) params.set('city', 'all');

  try {
    return {
      discovery: await client.listEvents(params),
      tickerURL: client.tickerURL(params),
      gatewayBaseURL: client.apiBase
    };
  } catch {
    return {
      discovery: mockDiscovery,
      tickerURL: client.tickerURL(params),
      gatewayBaseURL: client.apiBase
    };
  }
};
