import { describe, it, expect } from 'vitest';
import {
  computeGeneratedCapacity,
  createSectionTemplate,
  nextSectionID,
  type SectionTemplate
} from './venue-form';

describe('nextSectionID', () => {
  it('assigns sequential letters starting from A', () => {
    expect(nextSectionID(0)).toBe('A');
    expect(nextSectionID(1)).toBe('B');
    expect(nextSectionID(7)).toBe('H');
  });
});

describe('createSectionTemplate', () => {
  it('builds a default 4x10 section for the given ID', () => {
    expect(createSectionTemplate('C')).toEqual({
      section_id: 'C',
      name: 'C Section',
      row_count: 4,
      seats_per_row: 10,
      accessible_edge_seats: true
    });
  });
});

describe('computeGeneratedCapacity', () => {
  const section = (
    overrides: Partial<SectionTemplate> = {}
  ): SectionTemplate => ({
    section_id: 'A',
    name: 'A Section',
    row_count: 4,
    seats_per_row: 10,
    accessible_edge_seats: true,
    ...overrides
  });

  it('sums row count times seats per row across sections', () => {
    const sections = [
      section({ row_count: 4, seats_per_row: 10 }),
      section({ row_count: 2, seats_per_row: 5 })
    ];

    expect(computeGeneratedCapacity(sections)).toBe(50);
  });

  it('returns 0 for no sections', () => {
    expect(computeGeneratedCapacity([])).toBe(0);
  });

  it('clamps negative row or seat counts to 0', () => {
    const sections = [section({ row_count: -3, seats_per_row: 10 })];

    expect(computeGeneratedCapacity(sections)).toBe(0);
  });
});
