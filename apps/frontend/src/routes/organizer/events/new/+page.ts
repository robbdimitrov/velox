import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
  try {
    const res = await fetch('/api/organizer/venues');
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error ?? body.message ?? 'Failed to load venues');
    }
    const body = await res.json();
    return { venues: body.venues || [], loadError: '' };
  } catch (err) {
    return {
      venues: [],
      loadError: err instanceof Error ? err.message : 'Failed to load venues'
    };
  }
};
