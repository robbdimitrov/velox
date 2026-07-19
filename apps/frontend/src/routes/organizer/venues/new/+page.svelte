<script lang="ts">
  import { MapPin, CheckCircle, ArrowLeft } from '@lucide/svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import Panel from '$lib/components/Panel.svelte';
  import TextField from '$lib/components/TextField.svelte';

  let loading = $state(false);
  let error = $state('');

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

<div class="mx-auto w-full max-w-3xl">
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

  <Panel padding="xl" overflowHidden flexColumn>
    {#if error}
      <div
        class="bg-urgency/20 border border-urgency/50 text-urgency p-3 rounded mb-6 text-sm backdrop-blur-sm animate-pulse"
      >
        {error}
      </div>
    {/if}

    <div
      class="flex-1 space-y-5 animate-in slide-in-from-right fade-in duration-300"
    >
      <h2 class="text-xl font-bold mb-6 flex items-center gap-2">
        <MapPin size={20} class="text-signal" />
        Venue Details
      </h2>

      <TextField
        id="name"
        label="Venue Name"
        bind:value={venueName}
        placeholder="e.g. Velox Arena"
      />
      <TextField
        id="city"
        label="City"
        bind:value={venueCity}
        placeholder="e.g. Chicago"
      />
      <TextField
        id="address"
        label="Address"
        bind:value={venueAddress}
        placeholder="e.g. 123 Main St"
      />
      <TextField
        id="capacity"
        label="Capacity"
        type="number"
        min="1"
        bind:value={venueCapacity}
        placeholder="e.g. 5000"
      />
    </div>

    <div class="mt-8 flex justify-end border-t border-line pt-6">
      <ActionButton
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
      </ActionButton>
    </div>
  </Panel>
</div>
