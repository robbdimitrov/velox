const DEFAULTS = {
  query: '',
  city: 'All cities',
  eventType: 'All live events',
  dateWindow: 'Any date',
  availableOnly: true
};

const DATE_WINDOWS = ['Any date', 'Today', 'This week', 'This month'];
const MAX_QUERY_LENGTH = 120;

export const filterState = $state({ ...DEFAULTS });

export function hydrateFilterStateFromURL(params: URLSearchParams) {
  filterState.query = normalizeQuery(params.get('q'));
  filterState.city = normalizeOption(params.get('city'), DEFAULTS.city);
  filterState.eventType = params.get('type') ?? DEFAULTS.eventType;
  filterState.dateWindow = normalizeAllowedOption(
    params.get('date'),
    DEFAULTS.dateWindow,
    DATE_WINDOWS
  );
  filterState.availableOnly = params.get('available') !== 'false';
}

export function filterStateToURLParams(): URLSearchParams {
  const params = new URLSearchParams();
  const query = normalizeQuery(filterState.query);
  if (query !== DEFAULTS.query) params.set('q', query);
  if (filterState.city !== DEFAULTS.city) params.set('city', filterState.city);
  if (filterState.eventType !== DEFAULTS.eventType)
    params.set('type', filterState.eventType);
  if (filterState.dateWindow !== DEFAULTS.dateWindow)
    params.set('date', filterState.dateWindow);
  if (filterState.availableOnly !== DEFAULTS.availableOnly)
    params.set('available', 'false');
  return params;
}

function normalizeQuery(value: string | null) {
  return (value ?? DEFAULTS.query).trim().slice(0, MAX_QUERY_LENGTH);
}

function normalizeOption(value: string | null, fallback: string) {
  const normalized = value?.trim();
  if (!normalized || normalized.toLowerCase() === 'all') return fallback;
  return normalized;
}

function normalizeAllowedOption(
  value: string | null,
  fallback: string,
  allowed: readonly string[]
) {
  const normalized = normalizeOption(value, fallback);
  return allowed.includes(normalized) ? normalized : fallback;
}
