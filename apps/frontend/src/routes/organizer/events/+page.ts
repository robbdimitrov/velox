import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
  try {
    const res = await fetch('/api/organizer/events');
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(body.error ?? body.message ?? 'Failed to load events');
    }
    return { events: body.events ?? [], loadError: '' };
  } catch (err) {
    return {
      events: [],
      loadError: err instanceof Error ? err.message : 'Failed to load events'
    };
  }
};
