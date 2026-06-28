<script lang="ts">
  import { Activity, Server, Users, Database } from '@lucide/svelte';

  let { 
    cpu = 45, 
    memory = 60, 
    activeUsers = 120,
    requestsPerSecond = 850
  } = $props();
</script>

<div class="glass-panel p-6">
  <div class="flex items-center gap-2 mb-6 border-b border-white/10 pb-4">
    <Activity class="text-signal" size={20} />
    <h3 class="text-sm font-black uppercase tracking-wider text-white">System Health</h3>
  </div>

  <div class="grid grid-cols-2 gap-4">
    <div class="bg-black/40 rounded-lg p-4 border border-white/5">
      <div class="flex items-center gap-2 text-inkMuted mb-2">
        <Server size={14} />
        <span class="text-xs font-bold uppercase tracking-widest">CPU Usage</span>
      </div>
      <div class="text-2xl font-mono text-white">{cpu}%</div>
      <div class="w-full bg-black/60 h-1.5 rounded-full mt-3 overflow-hidden">
        <div class="bg-signal h-full rounded-full" style="width: {cpu}%"></div>
      </div>
    </div>

    <div class="bg-black/40 rounded-lg p-4 border border-white/5">
      <div class="flex items-center gap-2 text-inkMuted mb-2">
        <Database size={14} />
        <span class="text-xs font-bold uppercase tracking-widest">Memory</span>
      </div>
      <div class="text-2xl font-mono text-white">{memory}%</div>
      <div class="w-full bg-black/60 h-1.5 rounded-full mt-3 overflow-hidden">
        <div class="bg-primary h-full rounded-full" style="width: {memory}%"></div>
      </div>
    </div>

    <div class="bg-black/40 rounded-lg p-4 border border-white/5">
      <div class="flex items-center gap-2 text-inkMuted mb-2">
        <Users size={14} />
        <span class="text-xs font-bold uppercase tracking-widest">Active Users</span>
      </div>
      <div class="text-2xl font-mono text-white">{activeUsers}</div>
    </div>

    <div class="bg-black/40 rounded-lg p-4 border border-white/5">
      <div class="flex items-center gap-2 text-inkMuted mb-2">
        <Activity size={14} />
        <span class="text-xs font-bold uppercase tracking-widest">Req / Sec</span>
      </div>
      <div class="text-2xl font-mono text-white">{requestsPerSecond}</div>
    </div>
  </div>
</div>
