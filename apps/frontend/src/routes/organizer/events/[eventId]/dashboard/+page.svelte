<script lang="ts">
  import ActionButton from '$lib/components/ActionButton.svelte';
  import Panel from '$lib/components/Panel.svelte';
  import SystemHealthPanel from '$lib/components/SystemHealthPanel.svelte';
  import TextAreaField from '$lib/components/TextAreaField.svelte';
  import TextField from '$lib/components/TextField.svelte';
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { createGatewayClient, GatewayError } from '$lib/api/client';
  import type { EventAnnouncement } from '$lib/api/types';
  import { Megaphone, OctagonAlert, Send } from '@lucide/svelte';

  let { data } = $props();

  const client = createGatewayClient(fetch, '/api');

  let eventId = $derived($page.params.eventId);
  let metrics = $state({
    cpu: 45,
    memory: 60,
    activeUsers: 120,
    requestsPerSecond: 850
  });
  let sseUrl = $derived(`/api/organizer/events/${eventId}/metrics/stream`);

  onMount(() => {
    if (typeof EventSource === 'undefined') return;

    const source = new EventSource(sseUrl);

    source.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.cpu !== undefined) metrics.cpu = data.cpu;
        if (data.memory !== undefined) metrics.memory = data.memory;
        if (data.activeUsers !== undefined)
          metrics.activeUsers = data.activeUsers;
        if (data.requestsPerSecond !== undefined)
          metrics.requestsPerSecond = data.requestsPerSecond;
      } catch {
        return;
      }
    };
    source.onerror = () => source.close();

    return () => source.close();
  });

  let announcements = $state<EventAnnouncement[]>([]);
  $effect(() => {
    eventId; // establishes the reactive dependency so navigation resyncs data
    announcements = data.announcements;
  });
  let announcementTitle = $state('');
  let announcementBody = $state('');
  let announcementSeverity = $state<
    'INFO' | 'SCHEDULE_CHANGE' | 'CANCELLATION'
  >('INFO');
  let postingAnnouncement = $state(false);
  let announcementError = $state('');

  async function postAnnouncement() {
    if (!eventId || !announcementTitle.trim() || !announcementBody.trim())
      return;
    postingAnnouncement = true;
    announcementError = '';

    try {
      const created = await client.postAnnouncement(eventId, {
        title: announcementTitle,
        body: announcementBody,
        severity: announcementSeverity
      });
      announcements = [created, ...announcements];
      announcementTitle = '';
      announcementBody = '';
      announcementSeverity = 'INFO';
    } catch (err) {
      announcementError =
        err instanceof GatewayError
          ? err.message
          : 'Failed to post update. Try again.';
    } finally {
      postingAnnouncement = false;
    }
  }

  let cancelling = $state(false);
  let cancelError = $state('');
  let cancelResult = $state<{
    status: string;
    cancelled_orders: number;
  } | null>(null);

  async function cancelEvent() {
    if (!eventId) return;
    const confirmed = confirm(
      'Cancel this event? All outstanding reservations will be cancelled and this cannot be undone.'
    );
    if (!confirmed) return;

    cancelling = true;
    cancelError = '';

    try {
      const result = await client.cancelEvent(eventId);
      cancelResult = {
        status: result.status,
        cancelled_orders: result.cancelled_orders
      };
    } catch (err) {
      cancelError =
        err instanceof GatewayError
          ? err.message
          : 'Failed to cancel event. Try again.';
    } finally {
      cancelling = false;
    }
  }
</script>

<svelte:head>
  <title>Live Analytics — Velox</title>
</svelte:head>

<div class="space-y-8">
  <div class="mb-8 flex items-end justify-between">
    <div>
      <h1 class="text-ink text-3xl font-black tracking-tight uppercase">
        Live Analytics
      </h1>
      <p class="text-signal mt-1 text-sm tracking-widest uppercase">
        Event: {eventId}
      </p>
    </div>
  </div>

  <div class="grid grid-cols-1 gap-8 lg:grid-cols-2">
    <SystemHealthPanel
      cpu={metrics.cpu}
      memory={metrics.memory}
      activeUsers={metrics.activeUsers}
      requestsPerSecond={metrics.requestsPerSecond}
    />

    <Panel padding="lg">
      <div class="border-line mb-6 flex items-center gap-2 border-b pb-4">
        <Megaphone class="text-signal" size={20} />
        <h3 class="text-ink text-sm font-black tracking-wider uppercase">
          Post Update
        </h3>
      </div>

      <form
        class="space-y-3"
        onsubmit={(e) => {
          e.preventDefault();
          postAnnouncement();
        }}
      >
        <TextField
          id="announcement-title"
          label="Title"
          bind:value={announcementTitle}
          placeholder="Title"
        />
        <TextAreaField
          id="announcement-body"
          label="Body"
          bind:value={announcementBody}
          placeholder="What do fans need to know?"
        />
        <select
          bind:value={announcementSeverity}
          class="select select-sm border-line bg-carbon/60 text-ink focus:border-signal focus:ring-signal/50 w-full rounded-sm focus:ring-1 focus:outline-none"
        >
          <option value="INFO">Info</option>
          <option value="SCHEDULE_CHANGE">Schedule Change</option>
          <option value="CANCELLATION">Cancellation</option>
        </select>

        {#if announcementError}
          <p class="text-urgency text-xs font-medium">{announcementError}</p>
        {/if}

        <ActionButton
          type="submit"
          fullWidth
          disabled={postingAnnouncement ||
            !announcementTitle.trim() ||
            !announcementBody.trim()}
        >
          <Send size={14} />
          {postingAnnouncement ? 'Posting...' : 'Post Update'}
        </ActionButton>
      </form>

      {#if announcements.length}
        <div class="border-line mt-6 space-y-2 border-t pt-4">
          {#each announcements as announcement (announcement.id)}
            <div
              class="border-line bg-panelSoft/70 rounded-sm border p-3 text-xs"
            >
              <p class="text-ink font-bold">{announcement.title}</p>
              <p class="text-inkMuted mt-1">{announcement.body}</p>
            </div>
          {/each}
        </div>
      {/if}
    </Panel>

    <div class="lg:col-span-2">
      <Panel padding="lg" accent="urgency">
        <div
          class="border-urgency/20 mb-6 flex items-center gap-2 border-b pb-4"
        >
          <OctagonAlert class="text-urgency" size={20} />
          <h3 class="text-urgency text-sm font-black tracking-wider uppercase">
            Danger Zone
          </h3>
        </div>

        <div
          class="flex flex-col justify-between gap-4 sm:flex-row sm:items-center"
        >
          <p class="text-inkMuted max-w-lg text-sm">
            Cancelling this event notifies reservation holders, cancels
            outstanding reservations, and marks the event as unavailable for new
            reservations. This action cannot be undone.
          </p>
          <button
            class="btn btn-sm bg-urgency hover:bg-urgency/80 shrink-0 rounded border-0 font-bold text-white disabled:opacity-40"
            onclick={cancelEvent}
            disabled={cancelling || cancelResult?.status === 'CANCELLED'}
          >
            {cancelling ? 'Cancelling...' : 'Cancel Event'}
          </button>
        </div>

        {#if cancelError}
          <p class="text-urgency mt-4 text-xs font-medium">{cancelError}</p>
        {/if}

        {#if cancelResult}
          <p
            class="border-urgency/40 bg-urgency/10 text-urgency mt-4 rounded border p-3 text-xs font-medium"
          >
            Event marked {cancelResult.status}. {cancelResult.cancelled_orders} reservation{cancelResult.cancelled_orders ===
            1
              ? ''
              : 's'} cancelled.
          </p>
        {/if}
      </Panel>
    </div>
  </div>
</div>
