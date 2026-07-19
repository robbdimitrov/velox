<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    href,
    size = 'sm',
    children
  }: { href: string; size?: 'sm' | 'md' | 'lg'; children: Snippet } = $props();
</script>

<a
  {href}
  class="btn btn-primary rounded-sm text-primary-content shadow-[0_12px_30px_rgba(242,184,75,0.22)]"
  class:btn-sm={size === 'sm'}
  class:btn-lg={size === 'lg'}
>
  {@render children()}
</a>
