<script lang="ts">
  import {
    MapPin,
    CheckCircle,
    ArrowLeft,
    Grid3X3,
    Plus,
    Trash2
  } from '@lucide/svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import Panel from '$lib/components/Panel.svelte';
  import TextField from '$lib/components/TextField.svelte';

  type SectionTemplate = {
    section_id: string;
    name: string;
    row_count: number;
    seats_per_row: number;
    price_cents: number;
    accessible_edge_seats: boolean;
  };

  let loading = $state(false);
  let error = $state('');

  let venueName = $state('');
  let venueCity = $state('');
  let venueAddress = $state('');
  let venueCapacity = $state(0);
  let sections = $state<SectionTemplate[]>([
    {
      section_id: 'A',
      name: 'A Section',
      row_count: 4,
      seats_per_row: 10,
      price_cents: 8500,
      accessible_edge_seats: true
    },
    {
      section_id: 'B',
      name: 'B Section',
      row_count: 4,
      seats_per_row: 10,
      price_cents: 8500,
      accessible_edge_seats: true
    }
  ]);

  const generatedCapacity = $derived(
    sections.reduce(
      (total, section) =>
        total +
        Math.max(0, section.row_count) * Math.max(0, section.seats_per_row),
      0
    )
  );

  function addSection() {
    const nextIndex = sections.length;
    const sectionID = String.fromCharCode('A'.charCodeAt(0) + nextIndex);
    sections = [
      ...sections,
      {
        section_id: sectionID,
        name: `${sectionID} Section`,
        row_count: 4,
        seats_per_row: 10,
        price_cents: 8500,
        accessible_edge_seats: true
      }
    ];
  }

  function removeSection(index: number) {
    sections = sections.filter((_, i) => i !== index);
  }

  async function submitVenue() {
    loading = true;
    error = '';
    try {
      const payload = {
        name: venueName,
        city: venueCity,
        address: venueAddress,
        capacity: venueCapacity || generatedCapacity,
        sections
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
        placeholder={`${generatedCapacity} generated seats`}
      />

      <div class="rounded-sm border border-line bg-panelSoft/70 p-4">
        <div class="mb-4 flex items-center justify-between gap-4">
          <div>
            <h3
              class="flex items-center gap-2 text-sm font-black uppercase tracking-widest text-ink"
            >
              <Grid3X3 size={18} class="text-signal" />
              Seat Template
            </h3>
            <p
              class="mt-1 text-xs font-bold uppercase tracking-widest text-inkMuted"
            >
              {generatedCapacity} generated seats
            </p>
          </div>
          <button
            class="btn btn-sm rounded-sm border border-line bg-panel text-ink hover:bg-signal hover:text-carbon"
            type="button"
            onclick={addSection}
            disabled={sections.length >= 8}
          >
            <Plus size={16} /> Section
          </button>
        </div>

        <div class="grid gap-3">
          {#each sections as section, index}
            <div
              class="grid gap-3 rounded-sm border border-line bg-panel/70 p-3 md:grid-cols-[80px_1fr_90px_110px_120px_auto]"
            >
              <label
                class="grid gap-1 text-xs font-bold uppercase tracking-widest text-inkMuted"
              >
                ID
                <input
                  class="input input-sm rounded-sm border-line bg-panelSoft font-mono text-ink"
                  bind:value={section.section_id}
                  maxlength="12"
                />
              </label>
              <label
                class="grid gap-1 text-xs font-bold uppercase tracking-widest text-inkMuted"
              >
                Name
                <input
                  class="input input-sm rounded-sm border-line bg-panelSoft text-ink"
                  bind:value={section.name}
                />
              </label>
              <label
                class="grid gap-1 text-xs font-bold uppercase tracking-widest text-inkMuted"
              >
                Rows
                <input
                  class="input input-sm rounded-sm border-line bg-panelSoft font-mono text-ink"
                  type="number"
                  min="1"
                  max="26"
                  bind:value={section.row_count}
                />
              </label>
              <label
                class="grid gap-1 text-xs font-bold uppercase tracking-widest text-inkMuted"
              >
                Seats/Row
                <input
                  class="input input-sm rounded-sm border-line bg-panelSoft font-mono text-ink"
                  type="number"
                  min="1"
                  max="50"
                  bind:value={section.seats_per_row}
                />
              </label>
              <label
                class="grid gap-1 text-xs font-bold uppercase tracking-widest text-inkMuted"
              >
                Price
                <input
                  class="input input-sm rounded-sm border-line bg-panelSoft font-mono text-ink"
                  type="number"
                  min="0"
                  step="100"
                  bind:value={section.price_cents}
                />
              </label>
              <div class="flex items-end gap-2">
                <label
                  class="flex h-8 items-center gap-2 text-xs font-bold uppercase tracking-widest text-inkMuted"
                >
                  <input
                    class="checkbox checkbox-primary checkbox-sm rounded"
                    type="checkbox"
                    bind:checked={section.accessible_edge_seats}
                  />
                  Edge
                </label>
                <button
                  class="btn btn-square btn-sm rounded-sm border border-line bg-panel text-inkMuted hover:border-urgency/40 hover:text-urgency"
                  type="button"
                  onclick={() => removeSection(index)}
                  disabled={sections.length <= 1}
                  title="Remove section"
                >
                  <Trash2 size={15} />
                </button>
              </div>
            </div>
          {/each}
        </div>
      </div>
    </div>

    <div class="mt-8 flex justify-end border-t border-line pt-6">
      <ActionButton
        onclick={submitVenue}
        disabled={loading ||
          !venueName ||
          !venueCity ||
          !venueAddress ||
          (venueCapacity || generatedCapacity) <= 0}
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
