<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    type = 'button',
    disabled = false,
    onclick,
    fullWidth = false,
    children
  }: {
    type?: 'button' | 'submit' | 'reset';
    disabled?: boolean;
    onclick?: () => void;
    fullWidth?: boolean;
    children: Snippet;
  } = $props();
</script>

<button
  {type}
  {disabled}
  {onclick}
  class="btn btn-primary btn-sm rounded-sm text-primary-content shadow-[0_12px_30px_rgba(242,184,75,0.22)] transition-transform hover:scale-[1.02] disabled:bg-inkMuted/30 disabled:text-ink/40 disabled:shadow-none disabled:hover:scale-100"
  class:w-full={fullWidth}
>
  {@render children()}
</button>
