<script lang="ts">
  import '../app.css';
  import {
    BriefcaseBusiness,
    Search,
    ShieldCheck,
    Ticket
  } from '@lucide/svelte';
  import { authState } from '$lib/state/auth-state.svelte';
  import { filterState } from '$lib/state/filter-state.svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  let { children } = $props();
</script>

<div class="min-h-screen text-ink pb-10" data-theme="velox">
  <div
    class="fixed inset-0 z-[-1] bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-signal/20 via-carbon to-carbon pointer-events-none"
  ></div>
  <header class="sticky top-4 z-30 mx-auto max-w-7xl px-4 pt-2 pb-6">
    <nav
      class="glass-panel flex h-16 items-center justify-between px-6 shadow-glow"
    >
      <a
        href="/"
        class="flex items-center gap-2 text-2xl font-black uppercase tracking-tight hover:scale-105 transition-transform"
      >
        <div
          class="p-2 bg-gradient-to-br from-signal to-accent rounded-xl shadow-lg shadow-signal/30"
        >
          <Ticket class="text-white" size={24} />
        </div>
        Velox
      </a>

      <label
        class="hidden md:flex items-center gap-2 bg-black/40 border border-white/10 rounded-full px-4 py-2 w-1/3 hover:border-signal/50 transition-colors shadow-inner"
      >
        <Search size={18} class="text-inkMuted" />
        <input
          bind:value={filterState.query}
          class="w-full bg-transparent text-sm outline-none placeholder:text-inkMuted"
          placeholder="Search events, venues, cities..."
        />
      </label>

      <div class="flex items-center gap-4">
        <select
          class="select select-bordered select-sm border-white/10 bg-black/50 text-ink rounded-lg focus:border-signal hover:border-signal/50 transition-colors cursor-pointer"
          bind:value={authState.role}
          onchange={() => {
            if (
              authState.role === 'vendor' &&
              page.url.pathname.startsWith('/wallet')
            ) {
              goto('/vendor');
            } else if (
              authState.role === 'reserver' &&
              page.url.pathname.startsWith('/vendor')
            ) {
              goto('/');
            }
          }}
        >
          <option value="reserver">Reserver View</option>
          <option value="vendor">Vendor View</option>
        </select>

        {#if authState.role === 'reserver'}
          <a
            class="btn btn-sm border-white/10 bg-black/40 text-ink hover:border-signal hover:bg-signal/20 rounded-lg shadow-sm backdrop-blur-md transition-all duration-300"
            href="/wallet"
          >
            <ShieldCheck size={16} class="text-ok" /> Wallet
          </a>
        {:else}
          <a
            class="btn btn-sm border-white/10 bg-black/40 text-ink hover:border-signal hover:bg-signal/20 rounded-lg shadow-sm backdrop-blur-md transition-all duration-300"
            href="/vendor"
          >
            <BriefcaseBusiness size={16} class="text-info" /> Dashboard
          </a>
        {/if}
      </div>
    </nav>
  </header>

  <div class="mx-auto max-w-7xl pt-4">
    {@render children()}
  </div>
</div>
