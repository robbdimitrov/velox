import { createGatewayClient, GatewayError } from '$lib/api/client';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
  const client = createGatewayClient(fetch, '/api');
  try {
    return { authRequired: false, wallet: await client.wallet() };
  } catch (err) {
    if (err instanceof GatewayError && err.status === 401) {
      return {
        authRequired: true,
        wallet: { verification_state: 'REQUIRED' as const, tickets: [] }
      };
    }
    throw err;
  }
};
