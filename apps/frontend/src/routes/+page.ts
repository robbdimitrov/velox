import { GatewayError, createGatewayClient } from '$lib/api/client';
import type { DiscoveryResponse } from '$lib/api/types';
import type { PageLoad } from './$types';

const emptyDiscovery: DiscoveryResponse = {
  events: [],
  featured: [],
  meta: {
    projection_lag_ms: 0,
    cache_status: 'unavailable'
  }
};

export const load: PageLoad = async ({ fetch, url }) => {
  const client = createGatewayClient(fetch, '/api');
  const params = new URLSearchParams(url.searchParams);
  if (!params.has('city')) params.set('city', 'all');
  let loadError = '';
  const discovery = await client.listEvents(params).catch((err) => {
    if (err instanceof GatewayError && err.status === 502) {
      loadError = `Discovery unavailable: ${err.code ?? err.message}.`;
      return emptyDiscovery;
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
