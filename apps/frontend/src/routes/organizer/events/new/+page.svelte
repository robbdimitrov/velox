<script lang="ts">
  import {
    MapPin,
    Calendar as CalendarIcon,
    CheckCircle,
    ArrowRight,
    ArrowLeft
  } from '@lucide/svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import Panel from '$lib/components/Panel.svelte';
  import TextAreaField from '$lib/components/TextAreaField.svelte';
  import TextField from '$lib/components/TextField.svelte';

  let { data } = $props();

  let currentStep = $state(1);
  let loading = $state(false);
  let error = $state('');

  let selectedVenue = $state('');
  let eventName = $state('');
  let eventDescription = $state('');
  let eventCategory = $state('Concerts');
  let eventDate = $state('');

  const categoryOptions = ['Concerts', 'Sports', 'Theatre', 'Festivals'];

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
        venue_id: selectedVenue,
        name: eventName,
        description: eventDescription,
        category: eventCategory,
        starts_at: new Date(eventDate).toISOString()
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
    } catch (err: unknown) {
      error = err instanceof Error ? err.message : 'Failed to create event';
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>Create Event - Velox Organizer</title>
</svelte:head>

<div class="mx-auto w-full max-w-3xl">
  <div class="mb-8">
    <h1 class="text-3xl font-black tracking-tight uppercase">Create Event</h1>
    <p class="text-inkMuted mt-1 text-sm">
      Publish a new event for reservations.
    </p>
  </div>

  <ul class="steps steps-horizontal mb-12 w-full font-semibold">
    {#each steps as step}
      <li
        class="step {currentStep >= step.id
          ? 'step-primary text-signal'
          : 'text-inkMuted'}"
      >
        <div class="mt-2 flex items-center gap-2">
          <step.icon size={16} />
          {step.title}
        </div>
      </li>
    {/each}
  </ul>

  <Panel padding="xl" overflowHidden flexColumn>
    {#if error}
      <div
        class="bg-urgency/20 border-urgency/50 text-urgency mb-6 animate-pulse rounded border p-3 text-sm backdrop-blur-sm"
      >
        {error}
      </div>
    {/if}

    {#if data.loadError}
      <div
        class="border-urgency/50 bg-urgency/10 text-urgency mb-6 rounded-sm border p-3 text-sm font-semibold"
      >
        {data.loadError}
      </div>
    {/if}

    {#if currentStep === 1}
      <div class="animate-in slide-in-from-right fade-in flex-1 duration-300">
        <h2 class="mb-6 text-xl font-bold">Select a Venue</h2>
        {#if data.loadError}
          <div
            class="border-urgency/30 bg-urgency/10 text-urgency rounded-sm border p-6 text-center"
          >
            <p>Venue data could not be loaded.</p>
          </div>
        {:else if data.venues.length === 0}
          <div
            class="border-warning/30 bg-warning/10 text-warning rounded border p-6 text-center"
          >
            <p>You need to create a venue first.</p>
            <a href="/organizer/venues" class="btn btn-sm btn-warning mt-4"
              >Go to Venues</a
            >
          </div>
        {:else}
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            {#each data.venues as venue}
              <button
                type="button"
                onclick={() => (selectedVenue = venue.id)}
                class="rounded-sm border p-4 text-left transition-all duration-300 {selectedVenue ===
                venue.id
                  ? 'bg-signal/20 border-signal shadow-signal/20 shadow-inner'
                  : 'bg-panelSoft/70 border-line hover:border-signal/50'}"
              >
                <div class="font-bold">{venue.name}</div>
                <div class="text-inkMuted mt-1 text-xs">
                  {venue.city} &bull; {venue.capacity} capacity
                </div>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}

    {#if currentStep === 2}
      <div
        class="animate-in slide-in-from-right fade-in flex-1 space-y-5 duration-300"
      >
        <h2 class="mb-6 text-xl font-bold">Event Details</h2>

        <TextField
          id="name"
          label="Event Name"
          bind:value={eventName}
          placeholder="e.g. Summer Music Festival"
        />
        <TextAreaField
          id="desc"
          label="Description"
          bind:value={eventDescription}
          placeholder="Tell attendees what to expect..."
        />
        <label class="form-control space-y-2">
          <span
            class="text-inkMuted text-xs font-semibold tracking-wider uppercase"
          >
            Category
          </span>
          <select
            bind:value={eventCategory}
            class="select select-bordered border-line bg-carbon/60 text-ink focus:border-signal focus:ring-signal/50 w-full rounded-sm focus:ring-1 focus:outline-none"
          >
            {#each categoryOptions as category}
              <option>{category}</option>
            {/each}
          </select>
        </label>
        <TextField
          id="date"
          label="Date & Time"
          type="datetime-local"
          bind:value={eventDate}
        />
      </div>
    {/if}

    {#if currentStep === 3}
      <div class="animate-in slide-in-from-right fade-in flex-1 duration-300">
        <h2 class="mb-6 text-xl font-bold">Review & Publish</h2>

        <div
          class="border-line bg-panelSoft/70 space-y-4 rounded-sm border p-6"
        >
          <div>
            <div class="text-inkMuted text-xs font-semibold uppercase">
              Event Name
            </div>
            <div class="text-lg font-bold">{eventName || 'Untitled Event'}</div>
          </div>
          <div class="space-y-2">
            <div class="text-inkMuted text-xs font-semibold uppercase">
              Date & Time
            </div>
            <div>
              {eventDate ? new Date(eventDate).toLocaleString() : 'Not set'}
            </div>
          </div>
          <div>
            <div class="text-inkMuted text-xs font-semibold uppercase">
              Category
            </div>
            <div>{eventCategory}</div>
          </div>
          <div>
            <div class="text-inkMuted text-xs font-semibold uppercase">
              Description
            </div>
            <div>{eventDescription || 'No description provided'}</div>
          </div>
          <div>
            <div class="text-inkMuted text-xs font-semibold uppercase">
              Venue ID
            </div>
            <div class="text-sm break-all">{selectedVenue}</div>
          </div>
        </div>
      </div>
    {/if}

    <div class="border-line mt-8 flex justify-between border-t pt-6">
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
        <ActionButton
          onclick={() => currentStep++}
          disabled={(currentStep === 1 && !selectedVenue) ||
            (currentStep === 2 && (!eventName || !eventDate || !eventCategory))}
        >
          Next <ArrowRight size={16} />
        </ActionButton>
      {:else}
        <ActionButton onclick={submitEvent} disabled={loading}>
          {#if loading}
            <span class="loading loading-spinner loading-sm"></span>
          {:else}
            Publish event <CheckCircle size={16} />
          {/if}
        </ActionButton>
      {/if}
    </div>
  </Panel>
</div>
