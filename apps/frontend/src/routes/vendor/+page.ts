import { env } from '$env/dynamic/public';
import type { PageLoad } from './$types';

export const load: PageLoad = async () => {
  return {
    gatewayBaseURL: env.PUBLIC_GATEWAY_BASE_URL || 'http://localhost:8080'
  };
};
