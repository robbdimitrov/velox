import { GatewayError, createGatewayClient } from '$lib/api/client';
import { mockDiscovery } from '$lib/api/mock';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url }) => {
  const client = createGatewayClient(fetch, '/api');
  const params = new URLSearchParams(url.searchParams);
  if (!params.has('city')) params.set('city', 'all');
  let loadError = '';
  const discovery = await client.listEvents(params).catch((err) => {
    if (err instanceof GatewayError && err.status === 502) {
      loadError = `Discovery unavailable: ${err.code ?? err.message}. Showing demo data.`;
      return mockDiscovery;
    }
    throw err;
  });

  return {
    discovery,
    loadError,
    tickerURL: client.tickerURL(params),
    gatewayBaseURL: client.apiBase
  };
};
