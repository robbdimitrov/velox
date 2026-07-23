<script lang="ts">
  import Panel from '$lib/components/Panel.svelte';
  import { pageTitle } from '$lib/pageTitle';
  import ThemePreferencePicker from '$lib/components/ThemePreferencePicker.svelte';
</script>

<svelte:head>
  <title>{pageTitle('Settings')}</title>
</svelte:head>

<div class="mb-8 flex items-end justify-between">
  <div>
    <h1 class="text-3xl font-black tracking-tight uppercase">Settings</h1>
    <p class="text-inkMuted mt-1 text-sm">
      Manage your organizer account and preferences.
    </p>
  </div>
</div>

<Panel padding="lg">
  <div class="border-line mb-6 border-b pb-4">
    <h2 class="text-ink text-sm font-black tracking-wider uppercase">
      Theme Preference
    </h2>
    <p class="text-inkMuted mt-1 text-sm">
      Choose how Velox should render on this device.
    </p>
  </div>

  <ThemePreferencePicker />
</Panel>
