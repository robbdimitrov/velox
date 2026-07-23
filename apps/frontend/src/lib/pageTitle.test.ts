import { describe, expect, it } from 'vitest';
import { pageTitle } from './pageTitle';

describe('pageTitle', () => {
  it('returns the bare app name without a page title', () => {
    expect(pageTitle()).toBe('Velox');
    expect(pageTitle(null)).toBe('Velox');
    expect(pageTitle('   ')).toBe('Velox');
  });

  it('formats page titles with the app suffix', () => {
    expect(pageTitle('Wallet')).toBe('Wallet - Velox');
    expect(pageTitle('  Detail Night  ')).toBe('Detail Night - Velox');
  });
});
