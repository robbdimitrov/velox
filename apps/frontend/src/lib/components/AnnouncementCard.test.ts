// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest';
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/svelte';
import AnnouncementCard from './AnnouncementCard.svelte';
import type { EventAnnouncement } from '$lib/api/types';

afterEach(() => cleanup());

function makeAnnouncement(
  overrides: Partial<EventAnnouncement> = {}
): EventAnnouncement {
  return {
    id: 'ann_test',
    event_id: 'evt_test',
    title: 'Doors open early',
    body: 'Doors now open at 6:30 PM instead of 7:00 PM.',
    severity: 'INFO',
    created_at: '2026-08-15T18:00:00Z',
    ...overrides
  };
}

describe('AnnouncementCard', () => {
  it('renders the title and body', () => {
    render(AnnouncementCard, { announcement: makeAnnouncement() });

    expect(screen.getByText('Doors open early')).toBeInTheDocument();
    expect(
      screen.getByText('Doors now open at 6:30 PM instead of 7:00 PM.')
    ).toBeInTheDocument();
  });

  it.each([
    ['INFO', 'text-ink'],
    ['SCHEDULE_CHANGE', 'text-warning'],
    ['CANCELLATION', 'text-urgency']
  ] as const)('applies the %s severity tone class', (severity, expectedClass) => {
    render(AnnouncementCard, { announcement: makeAnnouncement({ severity }) });

    const heading = screen.getByText('Doors open early');
    expect(heading.className).toContain(expectedClass);
  });
});
