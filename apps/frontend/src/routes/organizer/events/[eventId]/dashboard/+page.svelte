<script lang="ts">
  import SystemHealthPanel from '$lib/components/SystemHealthPanel.svelte';
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
      'Cancel this event? All outstanding orders will be cancelled and this cannot be undone.'
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

<div class="space-y-8">
  <div class="mb-8 flex justify-between items-end">
    <div>
      <h1 class="text-3xl font-black uppercase text-white tracking-tight">
        Live Analytics
      </h1>
      <p class="text-signal uppercase tracking-widest text-sm mt-1">
        Event: {eventId}
      </p>
    </div>
  </div>

  <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
    <SystemHealthPanel
      cpu={metrics.cpu}
      memory={metrics.memory}
      activeUsers={metrics.activeUsers}
      requestsPerSecond={metrics.requestsPerSecond}
    />

    <div class="glass-panel p-6">
      <div class="flex items-center gap-2 mb-6 border-b border-white/10 pb-4">
        <Megaphone class="text-signal" size={20} />
        <h3 class="text-sm font-black uppercase tracking-wider text-white">
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
        <input
          bind:value={announcementTitle}
          placeholder="Title"
          class="input input-sm w-full border-white/10 bg-black/40 text-ink rounded focus:border-signal"
          maxlength="200"
        />
        <textarea
          bind:value={announcementBody}
          placeholder="What do fans need to know?"
          rows="3"
          class="textarea textarea-sm w-full border-white/10 bg-black/40 text-ink rounded focus:border-signal"
          maxlength="2000"></textarea>
        <select
          bind:value={announcementSeverity}
          class="select select-sm w-full border-white/10 bg-black/40 text-ink rounded focus:border-signal"
        >
          <option value="INFO">Info</option>
          <option value="SCHEDULE_CHANGE">Schedule Change</option>
          <option value="CANCELLATION">Cancellation</option>
        </select>

        {#if announcementError}
          <p class="text-xs font-medium text-urgency">{announcementError}</p>
        {/if}

        <button
          type="submit"
          class="btn btn-sm velox-action w-full rounded disabled:bg-inkMuted/30 disabled:text-white/40"
          disabled={postingAnnouncement ||
            !announcementTitle.trim() ||
            !announcementBody.trim()}
        >
          <Send size={14} />
          {postingAnnouncement ? 'Posting...' : 'Post Update'}
        </button>
      </form>

      {#if announcements.length}
        <div class="mt-6 space-y-2 border-t border-white/10 pt-4">
          {#each announcements as announcement (announcement.id)}
            <div class="rounded border border-white/5 bg-black/30 p-3 text-xs">
              <p class="font-bold text-white">{announcement.title}</p>
              <p class="text-inkMuted mt-1">{announcement.body}</p>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <div class="glass-panel p-6 border border-urgency/30 lg:col-span-2">
      <div class="flex items-center gap-2 mb-6 border-b border-urgency/20 pb-4">
        <OctagonAlert class="text-urgency" size={20} />
        <h3 class="text-sm font-black uppercase tracking-wider text-urgency">
          Danger Zone
        </h3>
      </div>

      <div
        class="flex flex-col sm:flex-row sm:items-center justify-between gap-4"
      >
        <p class="text-sm text-inkMuted max-w-lg">
          Cancelling this event notifies ticket holders, cancels outstanding
          orders, and marks the event as unavailable for new sales. This action
          cannot be undone.
        </p>
        <button
          class="btn btn-sm border-0 bg-urgency hover:bg-urgency/80 text-white font-bold rounded shrink-0 disabled:opacity-40"
          onclick={cancelEvent}
          disabled={cancelling || cancelResult?.status === 'CANCELLED'}
        >
          {cancelling ? 'Cancelling...' : 'Cancel Event'}
        </button>
      </div>

      {#if cancelError}
        <p class="mt-4 text-xs font-medium text-urgency">{cancelError}</p>
      {/if}

      {#if cancelResult}
        <p
          class="mt-4 rounded border border-urgency/40 bg-urgency/10 p-3 text-xs font-medium text-urgency"
        >
          Event marked {cancelResult.status}. {cancelResult.cancelled_orders} order{cancelResult.cancelled_orders ===
          1
            ? ''
            : 's'} cancelled.
        </p>
      {/if}
    </div>
  </div>
</div>
