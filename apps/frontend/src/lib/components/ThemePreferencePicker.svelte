<script lang="ts">
  import { Monitor, Moon, Sun } from '@lucide/svelte';
  import {
    themeState,
    type ThemePreference
  } from '$lib/state/theme-state.svelte';

  const options: Array<{
    value: ThemePreference;
    label: string;
    description: string;
    icon: typeof Monitor;
  }> = [
    {
      value: 'system',
      label: 'System',
      description: 'Follow this device preference.',
      icon: Monitor
    },
    {
      value: 'light',
      label: 'Light',
      description: 'Use the high-contrast light theme.',
      icon: Sun
    },
    {
      value: 'dark',
      label: 'Dark',
      description: 'Use the high-contrast dark theme.',
      icon: Moon
    }
  ];
</script>

<div class="grid gap-3 sm:grid-cols-3">
  {#each options as option}
    <button
      type="button"
      class="rounded-sm border p-4 text-left transition-colors {themeState.preference ===
      option.value
        ? 'border-signal bg-signal/10 text-ink'
        : 'border-line bg-panelSoft text-inkMuted'}"
      aria-pressed={themeState.preference === option.value}
      onclick={() => themeState.setPreference(option.value)}
    >
      <div class="mb-3 flex items-center gap-2">
        <option.icon size={18} class="text-signal" />
        <span class="font-bold uppercase">{option.label}</span>
      </div>
      <p class="text-sm">{option.description}</p>
    </button>
  {/each}
</div>
