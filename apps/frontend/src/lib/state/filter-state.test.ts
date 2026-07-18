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
    filterState.dateWindow = 'Any date';
    filterState.availableOnly = true;
  });

  it('hydrates from URL params, falling back to defaults for missing ones', () => {
    hydrateFilterStateFromURL(
      new URLSearchParams('city=Chicago&date=This+week&available=false')
    );

    expect(filterState.city).toBe('Chicago');
    expect(filterState.dateWindow).toBe('This week');
    expect(filterState.availableOnly).toBe(false);
    expect(filterState.query).toBe('');
    expect(filterState.eventType).toBe('All live events');
  });

  it('normalizes gateway sentinels and unsupported fixed filter values', () => {
    hydrateFilterStateFromURL(
      new URLSearchParams(
        'q=%20abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz&city=all&date=Next+year'
      )
    );

    expect(filterState.query).toHaveLength(120);
    expect(filterState.city).toBe('All cities');
    expect(filterState.dateWindow).toBe('Any date');
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
    expect(params.has('date')).toBe(false);
  });

  it('round-trips hydrate -> serialize -> hydrate to the same state', () => {
    hydrateFilterStateFromURL(
      new URLSearchParams(
        'q=riot&city=Austin&type=Concerts&date=This+month&available=false'
      )
    );
    const params = filterStateToURLParams();

    filterState.query = '';
    filterState.city = 'All cities';
    filterState.eventType = 'All live events';
    filterState.dateWindow = 'Any date';
    filterState.availableOnly = true;

    hydrateFilterStateFromURL(params);

    expect(filterState.query).toBe('riot');
    expect(filterState.city).toBe('Austin');
    expect(filterState.eventType).toBe('Concerts');
    expect(filterState.dateWindow).toBe('This month');
    expect(filterState.availableOnly).toBe(false);
  });
});
