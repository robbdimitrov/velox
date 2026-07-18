<script lang="ts">
  import {
    MapPin,
    Calendar as CalendarIcon,
    CheckCircle,
    ArrowRight,
    ArrowLeft
  } from '@lucide/svelte';
  let { data } = $props();

  let currentStep = $state(1);
  let loading = $state(false);
  let error = $state('');

  let selectedVenue = $state('');
  let eventName = $state('');
  let eventDescription = $state('');
  let eventDate = $state('');

  const steps = [
    { id: 1, title: 'Venue', icon: MapPin },
    { id: 2, title: 'Details', icon: CalendarIcon },
    { id: 3, title: 'Confirm', icon: CheckCircle }
  ];

  async function submitEvent() {
    loading = true;
    error = '';
    try {
      const payload = {
        venueId: selectedVenue,
        name: eventName,
        description: eventDescription,
        date: new Date(eventDate).toISOString()
      };

      const res = await fetch('/api/organizer/events', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });

      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        throw new Error(d.message || 'Failed to create event');
      }
      window.location.href = '/organizer';
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>Create Event - Velox Organizer</title>
</svelte:head>

<div class="content-narrow mt-4">
  <div class="mb-8">
    <h1 class="text-3xl font-black uppercase tracking-tight">Create Event</h1>
    <p class="text-inkMuted text-sm mt-1">
      Publish a new event to the marketplace.
    </p>
  </div>

  <ul class="steps steps-horizontal w-full mb-12 font-semibold">
    {#each steps as step}
      <li
        class="step {currentStep >= step.id
          ? 'step-primary text-signal'
          : 'text-inkMuted'}"
      >
        <div class="flex items-center gap-2 mt-2">
          <step.icon size={16} />
          {step.title}
        </div>
      </li>
    {/each}
  </ul>

  <div
    class="glass-panel p-8 rounded shadow-glow min-h-[400px] flex flex-col relative overflow-hidden"
  >
    {#if error}
      <div
        class="bg-urgency/20 border border-urgency/50 text-urgency p-3 rounded mb-6 text-sm backdrop-blur-sm animate-pulse"
      >
        {error}
      </div>
    {/if}

    {#if currentStep === 1}
      <div class="flex-1 animate-in slide-in-from-right fade-in duration-300">
        <h2 class="text-xl font-bold mb-6">Select a Venue</h2>
        {#if data.venues.length === 0}
          <div
            class="p-6 border border-warning/30 bg-warning/10 rounded text-center text-warning"
          >
            <p>You need to create a venue first.</p>
            <a href="/organizer/venues" class="btn btn-sm btn-warning mt-4"
              >Go to Venues</a
            >
          </div>
        {:else}
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            {#each data.venues as venue}
              <button
                type="button"
                onclick={() => (selectedVenue = venue.id)}
                class="text-left p-4 rounded border transition-all duration-300 {selectedVenue ===
                venue.id
                  ? 'bg-signal/20 border-signal shadow-inner shadow-signal/20'
                  : 'bg-black/40 border-white/10 hover:border-white/30'}"
              >
                <div class="font-bold">{venue.name}</div>
                <div class="text-xs text-inkMuted mt-1">
                  {venue.city}, {venue.country} &bull; {venue.capacity} capacity
                </div>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}

    {#if currentStep === 2}
      <div
        class="flex-1 space-y-5 animate-in slide-in-from-right fade-in duration-300"
      >
        <h2 class="text-xl font-bold mb-6">Event Details</h2>

        <div class="space-y-2">
          <label
            class="text-xs font-semibold uppercase tracking-wider text-inkMuted"
            for="name">Event Name</label
          >
          <input
            id="name"
            type="text"
            bind:value={eventName}
            class="velox-field w-full px-4 py-3 placeholder:text-inkMuted/50"
            placeholder="e.g. Summer Music Festival"
          />
        </div>

        <div class="space-y-2">
          <label
            class="text-xs font-semibold uppercase tracking-wider text-inkMuted"
            for="desc">Description</label
          >
          <textarea
            id="desc"
            bind:value={eventDescription}
            rows="3"
            class="velox-field w-full resize-none px-4 py-3 placeholder:text-inkMuted/50"
            placeholder="Tell attendees what to expect..."></textarea>
        </div>

        <div class="space-y-2">
          <label
            class="text-xs font-semibold uppercase tracking-wider text-inkMuted"
            for="date">Date & Time</label
          >
          <input
            id="date"
            type="datetime-local"
            bind:value={eventDate}
            class="velox-field w-full px-4 py-3 placeholder:text-inkMuted/50"
          />
        </div>
      </div>
    {/if}

    {#if currentStep === 3}
      <div class="flex-1 animate-in slide-in-from-right fade-in duration-300">
        <h2 class="text-xl font-bold mb-6">Review & Publish</h2>

        <div class="bg-black/40 border border-white/10 rounded p-6 space-y-4">
          <div>
            <div class="text-xs text-inkMuted uppercase font-semibold">
              Event Name
            </div>
            <div class="font-bold text-lg">{eventName || 'Untitled Event'}</div>
          </div>
          <div class="space-y-2">
            <div class="text-xs text-inkMuted uppercase font-semibold">
              Date & Time
            </div>
            <div>
              {eventDate ? new Date(eventDate).toLocaleString() : 'Not set'}
            </div>
          </div>
          <div>
            <div class="text-xs text-inkMuted uppercase font-semibold">
              Description
            </div>
            <div>{eventDescription || 'No description provided'}</div>
          </div>
          <div>
            <div class="text-xs text-inkMuted uppercase font-semibold">
              Venue ID
            </div>
            <div class="text-sm break-all">{selectedVenue}</div>
          </div>
        </div>
      </div>
    {/if}

    <div class="mt-8 pt-6 border-t border-white/10 flex justify-between">
      <button
        type="button"
        class="btn btn-sm btn-ghost text-ink {currentStep === 1
          ? 'invisible'
          : ''}"
        onclick={() => currentStep--}
      >
        <ArrowLeft size={16} /> Back
      </button>

      {#if currentStep < 3}
        <button
          type="button"
          class="btn btn-sm velox-action rounded"
          onclick={() => currentStep++}
          disabled={(currentStep === 1 && !selectedVenue) ||
            (currentStep === 2 && (!eventName || !eventDate))}
        >
          Next <ArrowRight size={16} />
        </button>
      {:else}
        <button
          type="button"
          class="btn btn-sm velox-action rounded transition-all hover:scale-105"
          onclick={submitEvent}
          disabled={loading}
        >
          {#if loading}
            <span class="loading loading-spinner loading-sm"></span>
          {:else}
            Publish Event <CheckCircle size={16} />
          {/if}
        </button>
      {/if}
    </div>
  </div>
</div>
