import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
  try {
    const res = await fetch('/api/vendor/venues');
    if (!res.ok) throw new Error('Failed to load venues');
    const body = await res.json();
    return { venues: body.venues || [] };
  } catch {
    return { venues: [] };
  }
};
