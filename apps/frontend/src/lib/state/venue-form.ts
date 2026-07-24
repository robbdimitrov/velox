export type SectionTemplate = {
  section_id: string;
  name: string;
  row_count: number;
  seats_per_row: number;
  accessible_edge_seats: boolean;
};

export function nextSectionID(existingCount: number): string {
  return String.fromCharCode('A'.charCodeAt(0) + existingCount);
}

export function createSectionTemplate(sectionID: string): SectionTemplate {
  return {
    section_id: sectionID,
    name: `${sectionID} Section`,
    row_count: 4,
    seats_per_row: 10,
    accessible_edge_seats: true
  };
}

export function computeGeneratedCapacity(sections: SectionTemplate[]): number {
  return sections.reduce(
    (total, section) =>
      total +
      Math.max(0, section.row_count) * Math.max(0, section.seats_per_row),
    0
  );
}
