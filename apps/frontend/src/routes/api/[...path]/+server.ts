import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';

const GATEWAY_URL =
  env.GATEWAY_URL ?? 'http://apigateway.velox.svc.cluster.local';

export const fallback: RequestHandler = async ({ request, url }) => {
  const path = url.pathname.replace(/^\/api/, '');
  const targetUrl = new URL(path + url.search, GATEWAY_URL);

  const requestHeaders = new Headers(request.headers);
  requestHeaders.delete('host');
  requestHeaders.delete('connection');

  const init: RequestInit = {
    method: request.method,
    headers: requestHeaders
  };

  if (request.method !== 'GET' && request.method !== 'HEAD' && request.body) {
    init.body = request.body;
    Object.assign(init, { duplex: 'half' });
  }

  try {
    const response = await fetch(targetUrl, init);
    return new Response(response.body, {
      status: response.status,
      statusText: response.statusText,
      headers: response.headers
    });
  } catch (err) {
    console.error('Proxy error:', err);
    return new Response('Gateway error', { status: 502 });
  }
};
