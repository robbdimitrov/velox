<script lang="ts">
  import { Monitor, Moon, Sun } from '@lucide/svelte';
  import Panel from '$lib/components/Panel.svelte';
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

<svelte:head>
  <title>Settings - Velox Organizer</title>
</svelte:head>

<div class="mb-8 flex items-end justify-between">
  <div>
    <h1 class="text-3xl font-black uppercase tracking-tight">Settings</h1>
    <p class="text-inkMuted text-sm mt-1">
      Manage your organizer account and preferences.
    </p>
  </div>
</div>

<Panel padding="lg">
  <div class="mb-6 border-b border-line pb-4">
    <h2 class="text-sm font-black uppercase tracking-wider text-ink">
      Theme Preference
    </h2>
    <p class="mt-1 text-sm text-inkMuted">
      Choose how Velox should render on this device.
    </p>
  </div>

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
</Panel>
