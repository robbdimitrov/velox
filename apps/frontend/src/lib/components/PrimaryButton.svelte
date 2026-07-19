<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    disabled = false,
    onclick,
    type = 'button',
    flush = false,
    children
  } = $props<{
    disabled?: boolean;
    onclick?: () => void;
    type?: 'button' | 'submit' | 'reset';
    flush?: boolean;
    children?: Snippet;
  }>();
</script>

<button
  class="btn btn-primary w-full rounded-sm text-lg text-primary-content shadow-[0_12px_30px_rgba(242,184,75,0.22)] transition-all hover:scale-[1.02] disabled:bg-inkMuted/30 disabled:text-ink/40 disabled:shadow-none disabled:hover:scale-100"
  class:mt-6={!flush}
  {type}
  {disabled}
  {onclick}
>
  {@render children?.()}
</button>
