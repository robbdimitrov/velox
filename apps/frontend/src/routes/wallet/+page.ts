import { env } from '$env/dynamic/public';
import { createGatewayClient } from '$lib/api/client';
import { mockWallet } from '$lib/api/mock';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
  const client = createGatewayClient(
    fetch,
    env.PUBLIC_GATEWAY_BASE_URL || 'http://localhost:8080'
  );
  try {
    return { wallet: await client.wallet() };
  } catch {
    return { wallet: mockWallet };
  }
};
