<script lang="ts">
  import SystemHealthPanel from '$lib/components/SystemHealthPanel.svelte';
  import { onMount } from 'svelte';
  import { page } from '$app/stores';

  let eventId = $derived($page.params.eventId);
  let metrics = $state({ cpu: 45, memory: 60, activeUsers: 120, requestsPerSecond: 850 });
  let sseUrl = $derived(`/api/organizer/events/${eventId}/metrics/stream`);

  onMount(() => {
    if (typeof EventSource === 'undefined') return;

    // Connect to the SSE metrics stream
    const source = new EventSource(sseUrl);
    
    source.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.cpu !== undefined) metrics.cpu = data.cpu;
        if (data.memory !== undefined) metrics.memory = data.memory;
        if (data.activeUsers !== undefined) metrics.activeUsers = data.activeUsers;
        if (data.requestsPerSecond !== undefined) metrics.requestsPerSecond = data.requestsPerSecond;
      } catch (e) {
        // ignore
      }
    };
    source.onerror = () => source.close();

    return () => source.close();
  });
</script>

<div class="p-8 max-w-7xl mx-auto">
  <div class="mb-8 flex justify-between items-end">
    <div>
      <h1 class="text-3xl font-black uppercase text-white tracking-tight">Live Analytics</h1>
      <p class="text-signal uppercase tracking-widest text-sm mt-1">Event: {eventId}</p>
    </div>
  </div>

  <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
    <SystemHealthPanel 
      cpu={metrics.cpu} 
      memory={metrics.memory} 
      activeUsers={metrics.activeUsers} 
      requestsPerSecond={metrics.requestsPerSecond} 
    />
  </div>
</div>
