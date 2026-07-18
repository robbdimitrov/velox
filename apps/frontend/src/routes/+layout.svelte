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
  import type { LayoutData } from './$types';
  import { User, LogIn, UserPlus } from '@lucide/svelte';

  let { data, children }: { data: LayoutData; children: any } = $props();

  $effect(() => {
    authState.user = data.user || null;
  });
</script>

<div class="min-h-screen text-ink pb-10" data-theme="velox">
  <div
    class="fixed inset-0 z-[-1] bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-signal/20 via-carbon to-carbon pointer-events-none"
  ></div>
  <header class="app-shell sticky top-4 z-30 pt-2 pb-6">
    <nav
      class="glass-panel flex h-16 items-center justify-between px-6 shadow-glow"
    >
      <a
        href="/"
        class="flex items-center gap-2 text-2xl font-black uppercase tracking-tight hover:scale-105 transition-transform"
      >
        <div class="rounded bg-signal p-2 shadow-lg shadow-signal/30">
          <Ticket class="text-carbon" size={24} />
        </div>
        Velox
      </a>

      <label
        class="velox-field hidden w-1/3 items-center gap-2 px-4 py-2 shadow-inner hover:border-signal/50 md:flex"
      >
        <Search size={18} class="text-inkMuted" />
        <input
          bind:value={filterState.query}
          aria-label="Search events, venues, and cities"
          class="w-full bg-transparent text-sm outline-none placeholder:text-inkMuted"
          maxlength="120"
          placeholder="Search events, venues, cities..."
        />
      </label>

      <div class="flex items-center gap-4">
        {#if authState.user}
          <a
            class="btn btn-sm border-white/10 bg-black/40 text-ink hover:border-signal hover:bg-signal/20 rounded shadow-sm backdrop-blur-md transition-all duration-300"
            href="/wallet"
          >
            <ShieldCheck size={16} class="text-ok" /> Wallet
          </a>
          <div class="dropdown dropdown-end">
            <div
              tabindex="0"
              role="button"
              class="btn btn-circle btn-sm btn-ghost border-white/10 bg-black/40 text-ink hover:bg-signal/20"
            >
              <User size={18} />
            </div>
            <ul
              tabindex="-1"
              class="dropdown-content z-[1] menu p-2 shadow bg-black/90 border border-white/10 rounded-box w-52 mt-4 backdrop-blur-xl"
            >
              <li
                class="menu-title text-inkMuted text-xs font-semibold px-4 py-2 border-b border-white/10 mb-2"
              >
                {authState.user.email}
              </li>
              <li>
                <a
                  href="/organizer"
                  class="hover:bg-white/10 hover:text-signal rounded transition-colors flex items-center gap-2"
                  ><BriefcaseBusiness size={14} /> Host Events</a
                >
              </li>
              <li>
                <a
                  href="/profile"
                  class="hover:bg-white/10 hover:text-signal rounded transition-colors"
                  >Profile</a
                >
              </li>
              <li>
                <a
                  href="/api/auth/logout"
                  class="hover:bg-white/10 hover:text-urgency rounded transition-colors"
                  >Logout</a
                >
              </li>
            </ul>
          </div>
        {:else}
          <a
            class="btn btn-sm btn-ghost text-ink hover:bg-white/5 rounded transition-colors"
            href="/login"
          >
            <LogIn size={16} /> Login
          </a>
          <a
            class="btn btn-sm velox-action rounded transition-all hover:scale-105"
            href="/register"
          >
            <UserPlus size={16} /> Register
          </a>
        {/if}
      </div>
    </nav>
  </header>

  <div class="pt-4">
    {@render children()}
  </div>
</div>
