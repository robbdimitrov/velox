export type EventWizardFields = {
  selectedVenue: string;
  eventName: string;
  eventDate: string;
  eventCategory: string;
};

export function canAdvanceStep(
  step: number,
  fields: EventWizardFields
): boolean {
  if (step === 1) return fields.selectedVenue !== '';
  if (step === 2)
    return Boolean(
      fields.eventName && fields.eventDate && fields.eventCategory
    );
  return true;
}
