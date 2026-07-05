const DEFAULTS = {
  query: '',
  city: 'All cities',
  eventType: 'All live events',
  availableOnly: true
};

export const filterState = $state({ ...DEFAULTS });

export function hydrateFilterStateFromURL(params: URLSearchParams) {
  filterState.query = params.get('q') ?? DEFAULTS.query;
  filterState.city = params.get('city') ?? DEFAULTS.city;
  filterState.eventType = params.get('type') ?? DEFAULTS.eventType;
  filterState.availableOnly = params.get('available') !== 'false';
}

export function filterStateToURLParams(): URLSearchParams {
  const params = new URLSearchParams();
  if (filterState.query !== DEFAULTS.query) params.set('q', filterState.query);
  if (filterState.city !== DEFAULTS.city) params.set('city', filterState.city);
  if (filterState.eventType !== DEFAULTS.eventType)
    params.set('type', filterState.eventType);
  if (filterState.availableOnly !== DEFAULTS.availableOnly)
    params.set('available', 'false');
  return params;
}
