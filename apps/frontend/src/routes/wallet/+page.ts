import { createGatewayClient } from '$lib/api/client';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
  const client = createGatewayClient(fetch, '/api');
  return { wallet: await client.wallet() };
};
