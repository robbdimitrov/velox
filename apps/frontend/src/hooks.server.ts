import type { Handle } from '@sveltejs/kit';

export const handle: Handle = async ({ event, resolve }) => {
  const sessionToken = event.cookies.get('velox_session');

  if (!sessionToken) {
    event.locals.user = null;
    return resolve(event);
  }

  try {
    const response = await event.fetch('/api/auth/me', {
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
