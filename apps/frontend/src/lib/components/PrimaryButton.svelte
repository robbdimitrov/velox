<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    disabled = false,
    onclick,
    children
  } = $props<{
    disabled?: boolean;
    onclick?: () => void;
    children?: Snippet;
  }>();
</script>

<button
  class="btn w-full mt-6 rounded border-0 bg-gradient-to-r from-signal to-primary text-white font-bold text-lg shadow-glow hover:scale-[1.02] transition-all disabled:from-inkMuted/30 disabled:to-inkMuted/30 disabled:text-white/40 disabled:hover:scale-100 disabled:shadow-none"
  {disabled}
  {onclick}
>
  {@render children?.()}
</button>
