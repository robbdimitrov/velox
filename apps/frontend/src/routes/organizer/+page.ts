import type { PageLoad } from './$types';

// Same-origin proxy path used by every other gateway call in this app (see
// apps/frontend/src/routes/api/[...path]/+server.ts) so this SSE connection
// doesn't need a cross-origin CSP connect-src allowance.
export const load: PageLoad = async () => {
  return {
    gatewayBaseURL: '/api'
  };
};
