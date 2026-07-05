<script lang="ts">
  import { MapPin, CheckCircle, ArrowLeft } from '@lucide/svelte';

  let loading = $state(false);
  let error = $state('');

  // Form State
  let venueName = $state('');
  let venueCity = $state('');
  let venueAddress = $state('');
  let venueCapacity = $state(0);

  async function submitVenue() {
    loading = true;
    error = '';
    try {
      const payload = {
        name: venueName,
        city: venueCity,
        address: venueAddress,
        capacity: venueCapacity
      };

      const res = await fetch('/api/organizer/venues', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });

      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        throw new Error(d.message || 'Failed to create venue');
      }
      window.location.href = '/organizer/venues';
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>Create Venue - Velox Organizer</title>
</svelte:head>

<div class="max-w-3xl mx-auto mt-4">
  <div class="mb-8">
    <div class="flex items-center gap-2">
      <a
        href="/organizer/venues"
        class="btn btn-sm btn-ghost text-inkMuted hover:text-ink"
      >
        <ArrowLeft size={16} />
      </a>
      <div>
        <h1 class="text-3xl font-black uppercase tracking-tight">
          Create Venue
        </h1>
        <p class="text-inkMuted text-sm mt-1">
          Add a new physical location for your events.
        </p>
      </div>
    </div>
  </div>

  <div
    class="glass-panel p-8 rounded shadow-glow min-h-[400px] flex flex-col relative overflow-hidden"
  >
    {#if error}
      <div
        class="bg-danger/20 border border-danger/50 text-danger p-3 rounded mb-6 text-sm backdrop-blur-sm animate-pulse"
      >
        {error}
      </div>
    {/if}

    <div
      class="flex-1 space-y-5 animate-in slide-in-from-right fade-in duration-300"
    >
      <h2 class="text-xl font-bold mb-6 flex items-center gap-2">
        <MapPin size={20} class="text-info" />
        Venue Details
      </h2>

      <div class="space-y-2">
        <label
          class="text-xs font-semibold uppercase tracking-wider text-inkMuted"
          for="name">Venue Name</label
        >
        <input
          id="name"
          type="text"
          bind:value={venueName}
          class="w-full bg-black/40 border border-white/10 rounded py-3 px-4 text-ink placeholder:text-inkMuted/50 focus:border-info focus:ring-1 focus:ring-info transition-all outline-none"
          placeholder="e.g. Velox Arena"
        />
      </div>

      <div class="space-y-2">
        <label
          class="text-xs font-semibold uppercase tracking-wider text-inkMuted"
          for="city">City</label
        >
        <input
          id="city"
          type="text"
          bind:value={venueCity}
          class="w-full bg-black/40 border border-white/10 rounded py-3 px-4 text-ink placeholder:text-inkMuted/50 focus:border-info focus:ring-1 focus:ring-info transition-all outline-none"
          placeholder="e.g. Chicago"
        />
      </div>

      <div class="space-y-2">
        <label
          class="text-xs font-semibold uppercase tracking-wider text-inkMuted"
          for="address">Address</label
        >
        <input
          id="address"
          type="text"
          bind:value={venueAddress}
          class="w-full bg-black/40 border border-white/10 rounded py-3 px-4 text-ink placeholder:text-inkMuted/50 focus:border-info focus:ring-1 focus:ring-info transition-all outline-none"
          placeholder="e.g. 123 Main St"
        />
      </div>

      <div class="space-y-2">
        <label
          class="text-xs font-semibold uppercase tracking-wider text-inkMuted"
          for="capacity">Capacity</label
        >
        <input
          id="capacity"
          type="number"
          bind:value={venueCapacity}
          min="1"
          class="w-full bg-black/40 border border-white/10 rounded py-3 px-4 text-ink placeholder:text-inkMuted/50 focus:border-info focus:ring-1 focus:ring-info transition-all outline-none"
          placeholder="e.g. 5000"
        />
      </div>
    </div>

    <!-- Navigation -->
    <div class="mt-8 pt-6 border-t border-white/10 flex justify-end">
      <button
        type="button"
        class="btn btn-sm border-none bg-gradient-to-r from-signal to-accent text-white shadow-lg shadow-signal/20 hover:shadow-signal/40 hover:scale-105 rounded transition-all"
        onclick={submitVenue}
        disabled={loading ||
          !venueName ||
          !venueCity ||
          !venueAddress ||
          venueCapacity <= 0}
      >
        {#if loading}
          <span class="loading loading-spinner loading-sm"></span>
        {:else}
          Create Venue <CheckCircle size={16} />
        {/if}
      </button>
    </div>
  </div>
</div>
