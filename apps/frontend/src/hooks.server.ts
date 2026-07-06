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
    // Calls the gateway directly (the same target apps/frontend/src/routes/
    // api/[...path]/+server.ts's proxy uses) with the global fetch, not
    // event.fetch. event.fetch on a same-origin path re-enters this app's
    // own request pipeline - including this same handle hook - so calling
    // '/api/auth/me' that way recurses: the re-entrant request carries the
    // same cookie, decides it also needs to check auth, and fetches
    // '/api/auth/me' again, unbounded, exploding memory until the process
    // OOMs. A direct external call to the real backend has no such cycle.
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
