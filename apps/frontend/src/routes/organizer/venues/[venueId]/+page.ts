import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params }) => {
  try {
    const res = await fetch(
      `/api/organizer/venues/${encodeURIComponent(params.venueId)}/staff`
    );
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(
        body.error ?? body.message ?? 'Failed to load venue staff'
      );
    }
    return { staff: body.staff ?? [], venueId: params.venueId, loadError: '' };
  } catch (err) {
    return {
      staff: [],
      venueId: params.venueId,
      loadError:
        err instanceof Error ? err.message : 'Failed to load venue staff'
    };
  }
};
