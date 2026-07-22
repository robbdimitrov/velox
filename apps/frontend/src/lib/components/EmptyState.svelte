<script lang="ts">
  import type { Component, Snippet } from 'svelte';
  import Panel from '$lib/components/Panel.svelte';

  let {
    icon: Icon,
    title,
    description,
    children
  }: {
    icon: Component<{ size?: number; class?: string }>;
    title: string;
    description: string;
    children?: Snippet;
  } = $props();
</script>

<Panel padding="xl">
  <div class="flex flex-col items-center justify-center text-center">
    <div
      class="bg-signal/10 text-signal mb-4 flex h-16 w-16 items-center justify-center rounded-sm shadow-inner"
    >
      <Icon size={32} />
    </div>
    <h3 class="mb-2 text-xl font-bold">{title}</h3>
    <p class="text-inkMuted mb-6 max-w-md">{description}</p>
    {@render children?.()}
  </div>
</Panel>
