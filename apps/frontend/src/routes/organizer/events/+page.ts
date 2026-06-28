import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
  try {
    const res = await fetch('/api/vendor/events');
    if (!res.ok) throw new Error('Failed to load events');
    const body = await res.json();
    return { events: body.events || [] };
  } catch {
    return { events: [] };
  }
};
