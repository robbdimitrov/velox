<script lang="ts">
  import type { Component } from 'svelte';

  let {
    id,
    label,
    type = 'text',
    placeholder = '',
    min,
    icon: Icon,
    value = $bindable<string | number>(''),
    required = false
  }: {
    id: string;
    label: string;
    type?: string;
    placeholder?: string;
    min?: string | number;
    icon?: Component<{ size?: number; class?: string }>;
    value: string | number;
    required?: boolean;
  } = $props();
</script>

<label class="form-control space-y-2">
  <span class="text-xs font-semibold uppercase tracking-wider text-inkMuted">
    {label}
  </span>
  <div class="relative flex items-center">
    {#if Icon}
      <Icon size={18} class="absolute left-3 text-inkMuted" />
    {/if}
    <input
      {id}
      {type}
      {min}
      bind:value
      {required}
      class="w-full rounded-sm border border-line bg-carbon/60 py-3 pr-4 text-ink shadow-inner outline-none transition-colors placeholder:text-inkMuted/50 focus:border-signal focus:ring-1 focus:ring-signal/50"
      class:pl-10={Icon}
      class:pl-4={!Icon}
      {placeholder}
    />
  </div>
</label>
