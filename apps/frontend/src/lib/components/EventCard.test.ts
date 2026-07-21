// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest';
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/svelte';
import EventCard from './EventCard.svelte';
import type { EventSummary } from '$lib/api/types';

afterEach(() => cleanup());

function makeEvent(overrides: Partial<EventSummary> = {}): EventSummary {
  return {
    id: 'evt_test',
    title: 'Neon Riot Live',
    venue: 'Velox Arena',
    city: 'Chicago',
    category: 'Concerts',
    starts_at: '2026-08-15T20:00:00Z',
    remaining_bucket: 'HIGH',
    demand_score: 94,
    projection_lag_ms: 120,
    ...overrides
  };
}

describe('EventCard', () => {
  it('renders event title, venue, and city', () => {
    render(EventCard, { event: makeEvent() });

    expect(screen.getByText('Neon Riot Live')).toBeInTheDocument();
    expect(screen.getByText(/Velox Arena/)).toBeInTheDocument();
    expect(screen.getByText(/Chicago/)).toBeInTheDocument();
  });

  it('links to the event detail page', () => {
    render(EventCard, { event: makeEvent({ id: 'evt_neon_riot' }) });

    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('href', '/events/evt_neon_riot');
  });

  it('renders the full bucket label', () => {
    render(EventCard, { event: makeEvent({ remaining_bucket: 'FULL' }) });

    expect(screen.getByText('FULL')).toBeInTheDocument();
  });

  it.each([
    ['LOW', 'text-urgency'],
    ['MEDIUM', 'text-warn'],
    ['HIGH', 'text-ok'],
    ['FULL', 'text-urgency']
  ] as const)('applies the %s scarcity tone class', (bucket, expectedClass) => {
    render(EventCard, { event: makeEvent({ remaining_bucket: bucket }) });

    const badge = screen.getByText(bucket);
    expect(badge.className).toContain(expectedClass);
  });
});
