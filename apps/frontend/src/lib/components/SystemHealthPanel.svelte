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
  <div class="border-line mb-6 flex items-center gap-2 border-b pb-4">
    <Activity class="text-signal" size={20} />
    <h3 class="text-ink text-sm font-black tracking-wider uppercase">
      System Health
    </h3>
  </div>

  <div class="grid grid-cols-2 gap-4">
    {#each metrics as metric}
      {@const Icon = metric.icon}
      <div class="border-line bg-panelSoft/70 rounded-sm border p-4">
        <div class="text-inkMuted mb-2 flex items-center gap-2">
          <Icon size={14} />
          <span class="text-xs font-bold tracking-widest uppercase">
            {metric.label}
          </span>
        </div>
        <div class="text-ink font-mono text-2xl tabular-nums">
          {metric.value}
        </div>
        {#if metric.progress !== undefined}
          <div
            class="bg-carbon/70 mt-3 h-1.5 w-full overflow-hidden rounded-full"
          >
            <div
              class="bg-signal h-full rounded-full"
              style="width: {metric.progress}%"
            ></div>
          </div>
        {/if}
      </div>
    {/each}
  </div>
</Panel>
