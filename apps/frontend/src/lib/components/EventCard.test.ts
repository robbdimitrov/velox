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
    image_url: '/event.jpg',
    sale_starts_at: '2026-08-15T20:00:00Z',
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

  it('renders the scarcity bucket label with underscores replaced by spaces', () => {
    render(EventCard, { event: makeEvent({ remaining_bucket: 'SOLD_OUT' }) });

    expect(screen.getByText('SOLD OUT')).toBeInTheDocument();
  });

  it.each([
    ['LOW', 'text-accent'],
    ['MEDIUM', 'text-warn'],
    ['HIGH', 'text-ok'],
    ['SOLD_OUT', 'text-urgency']
  ] as const)('applies the %s scarcity tone class', (bucket, expectedClass) => {
    render(EventCard, { event: makeEvent({ remaining_bucket: bucket }) });

    const badge = screen.getByText(bucket.replace('_', ' '));
    expect(badge.className).toContain(expectedClass);
  });
});
