<script lang="ts">
  import '../app.css';
  import {
    BriefcaseBusiness,
    LogIn,
    Search,
    ShieldCheck,
    Ticket,
    User,
    UserPlus
  } from '@lucide/svelte';
  import { authState } from '$lib/state/auth-state.svelte';
  import { filterState } from '$lib/state/filter-state.svelte';
  import { themeState } from '$lib/state/theme-state.svelte';
  import type { Snippet } from 'svelte';
  import type { LayoutData } from './$types';

  let { data, children }: { data: LayoutData; children: Snippet } = $props();

  $effect(() => {
    authState.user = data.user || null;
  });

  $effect(() => themeState.init());

  function clearLocalPreferences() {
    themeState.clearOverride();
  }
</script>

<div
  class="text-ink min-h-screen bg-[linear-gradient(135deg,color-mix(in_oklab,var(--color-primary)_10%,transparent),transparent_30%),linear-gradient(225deg,color-mix(in_oklab,var(--color-info)_9%,transparent),transparent_36%),var(--color-base-100)] pb-10 font-sans antialiased"
  data-theme={themeState.resolvedTheme}
>
  <div
    class="pointer-events-none fixed inset-0 z-0 bg-[linear-gradient(color-mix(in_oklab,var(--color-base-300)_28%,transparent)_1px,transparent_1px),linear-gradient(90deg,color-mix(in_oklab,var(--color-base-300)_28%,transparent)_1px,transparent_1px)] [mask-image:linear-gradient(to_bottom,rgba(0,0,0,0.72),transparent_72%)] bg-[size:36px_36px]"
  ></div>
  <div
    class="relative z-10 mx-auto flex w-[min(calc(100%_-_2rem),80rem)] flex-col gap-6 pt-6 pb-8"
  >
    <header class="sticky top-4 z-30">
      <nav
        class="border-line bg-panel/90 flex h-16 items-center justify-between rounded-sm border px-3 shadow-[0_0_0_1px_rgba(242,184,75,0.24),0_20px_70px_rgba(0,0,0,0.45)] backdrop-blur-xl sm:px-4 lg:px-5"
      >
        <a
          href="/"
          class="hover:text-signal flex min-w-0 items-center gap-3 text-xl font-black tracking-tight uppercase transition-colors sm:text-2xl"
        >
          <div
            class="bg-signal text-primary-content shadow-signal/20 grid h-10 w-10 place-items-center rounded-sm shadow-lg"
          >
            <Ticket size={22} />
          </div>
          <span>Velox</span>
        </a>

        <label
          class="border-line bg-carbon/60 focus-within:border-signal focus-within:ring-signal/50 hover:border-signal/50 hidden w-[min(38vw,30rem)] items-center gap-2 rounded-sm border px-4 py-2 shadow-inner transition-colors outline-none focus-within:ring-1 md:flex"
        >
          <Search size={18} class="text-inkMuted" />
          <input
            bind:value={filterState.query}
            aria-label="Search events, venues, and cities"
            class="placeholder:text-inkMuted w-full bg-transparent text-sm outline-none"
            maxlength="120"
            placeholder="Search events, venues, cities..."
          />
        </label>

        <div class="flex items-center gap-2 sm:gap-3">
          {#if authState.user}
            <a
              class="btn btn-sm border-line bg-panelSoft text-ink hover:border-signal hover:bg-panel rounded-sm"
              href="/wallet"
            >
              <ShieldCheck size={16} class="text-ok" /> Wallet
            </a>
            <div class="dropdown dropdown-end">
              <div
                tabindex="0"
                role="button"
                class="btn btn-circle btn-sm btn-ghost border-line bg-panelSoft text-ink hover:bg-panel"
              >
                <User size={18} />
              </div>
              <ul
                tabindex="-1"
                class="dropdown-content menu border-line bg-panel z-[1] mt-4 w-52 rounded-sm border p-2 shadow-xl"
              >
                <li
                  class="menu-title border-line text-inkMuted mb-2 border-b px-4 py-2 text-xs font-semibold"
                >
                  {authState.user.email}
                </li>
                <li>
                  <a
                    href="/organizer"
                    class="hover:bg-panelSoft hover:text-signal flex items-center gap-2 rounded-sm transition-colors"
                    ><BriefcaseBusiness size={14} /> Host Events</a
                  >
                </li>
                <li>
                  <a
                    href="/profile"
                    class="hover:bg-panelSoft hover:text-signal rounded-sm transition-colors"
                    >Profile</a
                  >
                </li>
                <li>
                  <a
                    href="/api/auth/logout"
                    onclick={clearLocalPreferences}
                    class="hover:bg-panelSoft hover:text-urgency rounded-sm transition-colors"
                    >Logout</a
                  >
                </li>
              </ul>
            </div>
          {:else}
            <a
              class="btn btn-sm btn-ghost text-ink hover:bg-panelSoft rounded-sm px-2 sm:px-3"
              href="/login"
            >
              <LogIn size={16} /> <span class="hidden sm:inline">Login</span>
            </a>
            <a
              class="btn btn-primary btn-sm text-primary-content rounded-sm px-2 sm:px-3"
              href="/register"
            >
              <UserPlus size={16} />
              <span class="hidden sm:inline">Register</span>
            </a>
          {/if}
        </div>
      </nav>
    </header>

    {@render children()}
  </div>
</div>
