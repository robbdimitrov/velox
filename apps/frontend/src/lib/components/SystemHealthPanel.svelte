<script lang="ts">
  import { Activity, Server, Users, Database } from '@lucide/svelte';
  import Panel from '$lib/components/Panel.svelte';

  let {
    cpu = 45,
    memory = 60,
    activeUsers = 120,
    requestsPerSecond = 850
  } = $props();

  const metrics = $derived([
    { icon: Server, label: 'CPU Usage', value: `${cpu}%`, progress: cpu },
    { icon: Database, label: 'Memory', value: `${memory}%`, progress: memory },
    { icon: Users, label: 'Active Users', value: activeUsers },
    { icon: Activity, label: 'Req / Sec', value: requestsPerSecond }
  ]);
</script>

<Panel padding="lg">
  <div class="mb-6 flex items-center gap-2 border-b border-line pb-4">
    <Activity class="text-signal" size={20} />
    <h3 class="text-sm font-black uppercase tracking-wider text-ink">
      System Health
    </h3>
  </div>

  <div class="grid grid-cols-2 gap-4">
    {#each metrics as metric}
      {@const Icon = metric.icon}
      <div class="rounded-sm border border-line bg-panelSoft/70 p-4">
        <div class="mb-2 flex items-center gap-2 text-inkMuted">
          <Icon size={14} />
          <span class="text-xs font-bold uppercase tracking-widest">
            {metric.label}
          </span>
        </div>
        <div class="font-mono tabular-nums text-2xl text-ink">
          {metric.value}
        </div>
        {#if metric.progress !== undefined}
          <div
            class="mt-3 h-1.5 w-full overflow-hidden rounded-full bg-carbon/70"
          >
            <div
              class="h-full rounded-full bg-signal"
              style="width: {metric.progress}%"
            ></div>
          </div>
        {/if}
      </div>
    {/each}
  </div>
</Panel>
