import { env } from '$env/dynamic/private';
import type { Handle } from '@sveltejs/kit';

const GATEWAY_URL =
  env.GATEWAY_URL ?? 'http://apigateway.velox.svc.cluster.local';

export const handle: Handle = async ({ event, resolve }) => {
  const sessionToken = event.cookies.get('velox_session');

  if (!sessionToken) {
    event.locals.user = null;
    return resolve(event);
  }

  try {
    // Use global fetch against the gateway; event.fetch('/api/auth/me') would
    // re-enter this handle hook and recurse with the same session cookie.
    const response = await fetch(`${GATEWAY_URL}/auth/me`, {
      headers: {
        cookie: `velox_session=${sessionToken}`
      }
    });

    if (response.ok) {
      const data = await response.json();
      event.locals.user = data;
    } else {
      event.locals.user = null;
    }
  } catch {
    event.locals.user = null;
  }

  return resolve(event);
};
