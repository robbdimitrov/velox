import { describe, it, expect, beforeEach } from 'vitest';
import {
  filterState,
  hydrateFilterStateFromURL,
  filterStateToURLParams
} from './filter-state.svelte';

describe('filterState URL round-trip', () => {
  beforeEach(() => {
    filterState.query = '';
    filterState.city = 'All cities';
    filterState.eventType = 'All live events';
    filterState.availableOnly = true;
  });

  it('hydrates from URL params, falling back to defaults for missing ones', () => {
    hydrateFilterStateFromURL(
      new URLSearchParams('city=Chicago&available=false')
    );

    expect(filterState.city).toBe('Chicago');
    expect(filterState.availableOnly).toBe(false);
    expect(filterState.query).toBe('');
    expect(filterState.eventType).toBe('All live events');
  });

  it('produces no params when state is all defaults', () => {
    const params = filterStateToURLParams();
    expect(params.toString()).toBe('');
  });

  it('only serializes fields that differ from their default', () => {
    filterState.city = 'Chicago';
    filterState.availableOnly = false;

    const params = filterStateToURLParams();

    expect(params.get('city')).toBe('Chicago');
    expect(params.get('available')).toBe('false');
    expect(params.has('q')).toBe(false);
    expect(params.has('type')).toBe(false);
  });

  it('round-trips hydrate -> serialize -> hydrate to the same state', () => {
    hydrateFilterStateFromURL(
      new URLSearchParams('q=riot&city=Austin&type=Concerts&available=false')
    );
    const params = filterStateToURLParams();

    filterState.query = '';
    filterState.city = 'All cities';
    filterState.eventType = 'All live events';
    filterState.availableOnly = true;

    hydrateFilterStateFromURL(params);

    expect(filterState.query).toBe('riot');
    expect(filterState.city).toBe('Austin');
    expect(filterState.eventType).toBe('Concerts');
    expect(filterState.availableOnly).toBe(false);
  });
});
