<script lang="ts">
  import type { Component } from 'svelte';
  import Panel from '$lib/components/Panel.svelte';

  type Tone = 'signal' | 'ok' | 'warn' | 'urgency';

  let {
    label,
    value,
    detail = '',
    icon: Icon,
    tone = 'signal'
  }: {
    label: string;
    value: string | number;
    detail?: string;
    icon?: Component<{ size?: number; class?: string }>;
    tone?: Tone;
  } = $props();
</script>

<Panel padding="lg" accent={tone === 'warn' ? 'signal' : tone}>
  <div class="flex items-start justify-between">
    <div>
      <p class="text-inkMuted mb-1 text-xs font-bold tracking-widest uppercase">
        {label}
      </p>
      <p class="text-ink font-mono text-3xl font-black tabular-nums">
        {value}
      </p>
      {#if detail}
        <p class="text-ink/40 mt-2 text-xs">{detail}</p>
      {/if}
    </div>
    {#if Icon}
      {#if tone === 'ok'}
        <div class="bg-ok/20 text-ok rounded-sm p-3 shadow-inner">
          <Icon size={28} />
        </div>
      {:else if tone === 'urgency'}
        <div class="bg-urgency/20 text-urgency rounded-sm p-3 shadow-inner">
          <Icon size={28} />
        </div>
      {:else if tone === 'warn'}
        <div class="bg-signal/20 text-warn rounded-sm p-3 shadow-inner">
          <Icon size={28} />
        </div>
      {:else}
        <div class="bg-signal/20 text-signal rounded-sm p-3 shadow-inner">
          <Icon size={28} />
        </div>
      {/if}
    {/if}
  </div>
</Panel>
