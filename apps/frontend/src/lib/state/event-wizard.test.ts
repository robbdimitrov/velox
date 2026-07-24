import { describe, it, expect } from 'vitest';
import { canAdvanceStep, type EventWizardFields } from './event-wizard';

function makeFields(
  overrides: Partial<EventWizardFields> = {}
): EventWizardFields {
  return {
    selectedVenue: 'venue_1',
    eventName: 'Summer Fest',
    eventDate: '2026-08-15T20:00',
    eventCategory: 'Concerts',
    ...overrides
  };
}

describe('canAdvanceStep', () => {
  it('requires a selected venue on step 1', () => {
    expect(canAdvanceStep(1, makeFields({ selectedVenue: '' }))).toBe(false);
    expect(canAdvanceStep(1, makeFields())).toBe(true);
  });

  it('requires name, date, and category on step 2', () => {
    expect(canAdvanceStep(2, makeFields({ eventName: '' }))).toBe(false);
    expect(canAdvanceStep(2, makeFields({ eventDate: '' }))).toBe(false);
    expect(canAdvanceStep(2, makeFields({ eventCategory: '' }))).toBe(false);
    expect(canAdvanceStep(2, makeFields())).toBe(true);
  });

  it('allows advancing past the final step', () => {
    expect(canAdvanceStep(3, makeFields())).toBe(true);
  });
});
