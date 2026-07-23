export function pageTitle(title?: string | null): string {
  const page = title?.trim();
  return page ? `${page} - Velox` : 'Velox';
}
