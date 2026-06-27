<script lang="ts">
  import '../app.css';
  import {
    BriefcaseBusiness,
    Search,
    ShieldCheck,
    Ticket
  } from '@lucide/svelte';
  import { authState } from '$lib/state/auth-state.svelte';

  let { children } = $props();
</script>

<div class="min-h-screen bg-carbon text-ink" data-theme="velox">
  <header class="sticky top-0 z-30 border-b border-line bg-carbon/95">
    <nav class="mx-auto flex h-16 max-w-7xl items-center gap-3 px-4">
      <a href="/" class="flex items-center gap-2 text-xl font-black uppercase">
        <Ticket class="text-signal" size={24} />
        Velox
      </a>
      <label
        class="hidden min-w-0 flex-1 items-center gap-2 border border-line bg-panel px-3 py-2 md:flex"
      >
        <Search size={16} class="text-ink/50" />
        <input
          class="min-w-0 flex-1 bg-transparent text-sm outline-none"
          placeholder="Search events, venues, cities"
        />
      </label>

      <div class="flex items-center gap-4">
        <select
          class="select select-bordered select-sm border-line bg-carbon"
          bind:value={authState.role}
        >
          <option value="reserver">Reserver View</option>
          <option value="vendor">Vendor View</option>
        </select>

        {#if authState.role === 'reserver'}
          <a
            class="btn btn-sm border-line bg-transparent text-ink hover:border-signal hover:bg-panel"
            href="/wallet"
          >
            <ShieldCheck size={16} /> Wallet
          </a>
        {:else}
          <a
            class="btn btn-sm border-line bg-transparent text-ink hover:border-signal hover:bg-panel"
            href="/vendor"
          >
            <BriefcaseBusiness size={16} /> Dashboard
          </a>
        {/if}
      </div>
    </nav>
  </header>
  {@render children()}
</div>
