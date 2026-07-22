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
  <span class="text-inkMuted text-xs font-semibold tracking-wider uppercase">
    {label}
  </span>
  <div class="relative flex items-center">
    {#if Icon}
      <Icon size={18} class="text-inkMuted absolute left-3" />
    {/if}
    <input
      {id}
      {type}
      {min}
      bind:value
      {required}
      class="border-line bg-carbon/60 text-ink placeholder:text-inkMuted/50 focus:border-signal focus:ring-signal/50 w-full rounded-sm border py-3 pr-4 shadow-inner transition-colors outline-none focus:ring-1"
      class:pl-10={Icon}
      class:pl-4={!Icon}
      {placeholder}
    />
  </div>
</label>
