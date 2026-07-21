export type ThemePreference = 'system' | 'light' | 'dark';
export type ResolvedTheme = 'velox-light' | 'velox-dark';

const storageKey = 'velox.theme';

export class ThemeState {
  preference = $state<ThemePreference>('system');
  systemDark = $state(false);
  initialized = $state(false);

  resolvedTheme = $derived<ResolvedTheme>(
    this.preference === 'dark' ||
      (this.preference === 'system' && this.systemDark)
      ? 'velox-dark'
      : 'velox-light'
  );

  init() {
    if (typeof window === 'undefined' || this.initialized) return;
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    this.systemDark = media.matches;
    this.preference = readStoredPreference();
    const sync = (event: MediaQueryListEvent) => {
      this.systemDark = event.matches;
    };
    media.addEventListener('change', sync);
    this.initialized = true;
    return () => media.removeEventListener('change', sync);
  }

  setPreference(preference: ThemePreference) {
    this.preference = preference;
    if (typeof localStorage === 'undefined') return;
    if (preference === 'system') {
      localStorage.removeItem(storageKey);
    } else {
      localStorage.setItem(storageKey, preference);
    }
  }

  clearOverride() {
    this.preference = 'system';
    if (typeof localStorage !== 'undefined') {
      localStorage.removeItem(storageKey);
    }
  }
}

function readStoredPreference(): ThemePreference {
  if (typeof localStorage === 'undefined') return 'system';
  const stored = localStorage.getItem(storageKey);
  return stored === 'light' || stored === 'dark' ? stored : 'system';
}

export const themeState = new ThemeState();
