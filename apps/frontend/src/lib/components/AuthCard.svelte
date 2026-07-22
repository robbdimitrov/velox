<script lang="ts">
  import type { Snippet } from 'svelte';
  import Panel from '$lib/components/Panel.svelte';

  let {
    title,
    description,
    children
  }: { title: string; description: string; children: Snippet } = $props();
</script>

<div class="mx-auto w-[min(calc(100%_-_2rem),28rem)]">
  <Panel padding="xl" overflowHidden>
    <div
      class="from-signal/10 pointer-events-none absolute inset-0 bg-gradient-to-br to-transparent"
    ></div>
    <div class="relative z-10">
      <h1 class="mb-2 text-3xl font-black tracking-tight uppercase">
        {title}
      </h1>
      <p class="text-inkMuted mb-8 text-sm">
        {description}
      </p>

      {@render children()}
    </div>
  </Panel>
</div>
