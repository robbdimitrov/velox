import { createGatewayClient } from '$lib/api/client';
import { mockWallet } from '$lib/api/mock';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
  const client = createGatewayClient(fetch, '/api');
  try {
    return { wallet: await client.wallet() };
  } catch {
    return { wallet: mockWallet };
  }
};
