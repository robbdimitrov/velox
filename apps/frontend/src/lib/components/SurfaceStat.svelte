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
      <p class="mb-1 text-xs font-bold uppercase tracking-widest text-inkMuted">
        {label}
      </p>
      <p class="font-mono text-3xl font-black tabular-nums text-ink">
        {value}
      </p>
      {#if detail}
        <p class="mt-2 text-xs text-ink/40">{detail}</p>
      {/if}
    </div>
    {#if Icon}
      {#if tone === 'ok'}
        <div class="rounded-sm bg-ok/20 p-3 text-ok shadow-inner">
          <Icon size={28} />
        </div>
      {:else if tone === 'urgency'}
        <div class="rounded-sm bg-urgency/20 p-3 text-urgency shadow-inner">
          <Icon size={28} />
        </div>
      {:else if tone === 'warn'}
        <div class="rounded-sm bg-signal/20 p-3 text-warn shadow-inner">
          <Icon size={28} />
        </div>
      {:else}
        <div class="rounded-sm bg-signal/20 p-3 text-signal shadow-inner">
          <Icon size={28} />
        </div>
      {/if}
    {/if}
  </div>
</Panel>
