import { env } from '$env/dynamic/private';
import type { Handle } from '@sveltejs/kit';

const GATEWAY_URL =
  env.GATEWAY_URL ?? 'http://apigateway.velox.svc.cluster.local';

export const handle: Handle = async ({ event, resolve }) => {
  const sessionToken = event.cookies.get('velox_session');

  if (!sessionToken) {
    event.locals.user = null;
  } else {
    try {
      // Use global fetch against the gateway; event.fetch('/api/auth/me') would
      // re-enter this handle hook and recurse with the same session cookie.
      const response = await fetch(`${GATEWAY_URL}/auth/me`, {
        headers: {
          cookie: `velox_session=${sessionToken}`
        }
      });

      event.locals.user = response.ok ? await response.json() : null;
    } catch {
      event.locals.user = null;
    }
  }

  const response = await resolve(event);
  response.headers.set('X-Content-Type-Options', 'nosniff');
  response.headers.set('X-Frame-Options', 'DENY');
  response.headers.set('Referrer-Policy', 'strict-origin-when-cross-origin');
  response.headers.set('Cross-Origin-Opener-Policy', 'same-origin');
  response.headers.set(
    'Permissions-Policy',
    'camera=(), microphone=(), geolocation=(), payment=(), usb=(), bluetooth=()'
  );
  // No Strict-Transport-Security or upgrade-insecure-requests: this
  // deployment has no TLS termination (see docs/deployment.md), so either
  // would promise a guarantee the transport doesn't hold.
  return response;
};
