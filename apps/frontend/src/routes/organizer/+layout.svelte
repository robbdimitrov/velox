<script lang="ts">
  import {
    LayoutDashboard,
    Calendar,
    MapPin,
    Settings,
    LogOut
  } from '@lucide/svelte';
  import { page } from '$app/state';
  import Panel from '$lib/components/Panel.svelte';
  import { themeState } from '$lib/state/theme-state.svelte';

  let { children } = $props();

  const links = [
    { href: '/organizer', icon: LayoutDashboard, label: 'Dashboard' },
    { href: '/organizer/events/new', icon: Calendar, label: 'Create Event' },
    { href: '/organizer/venues', icon: MapPin, label: 'Venues' },
    { href: '/organizer/settings', icon: Settings, label: 'Settings' }
  ];
</script>

<div class="flex min-h-[calc(100vh-6rem)] w-full flex-col gap-6 lg:flex-row">
  <aside class="w-full flex-shrink-0 lg:w-64">
    <Panel padding="sm" flexColumn>
      <div class="mb-8 px-2">
        <h2 class="text-signal text-xl font-black tracking-tight uppercase">
          Organizer Portal
        </h2>
        <p class="text-inkMuted mt-1 text-xs">Manage your events and venues</p>
      </div>

      <nav class="flex-1 space-y-2">
        {#each links as link}
          {@const isActive =
            page.url.pathname === link.href ||
            (link.href !== '/organizer' &&
              page.url.pathname.startsWith(link.href))}
          <a
            href={link.href}
            class="flex items-center gap-3 rounded px-4 py-3 transition-all duration-300 {isActive
              ? 'bg-signal/20 text-signal shadow-signal/20 font-semibold shadow-inner'
              : 'text-inkMuted hover:bg-panelSoft hover:text-ink'}"
          >
            <link.icon size={20} />
            {link.label}
          </a>
        {/each}
      </nav>

      <div class="border-line mt-auto border-t pt-4">
        <a
          href="/api/auth/logout"
          onclick={() => themeState.clearOverride()}
          class="text-urgency/80 hover:bg-urgency/10 hover:text-urgency flex items-center gap-3 rounded px-4 py-3 transition-all duration-300"
        >
          <LogOut size={20} />
          Logout
        </a>
      </div>
    </Panel>
  </aside>

  <main class="min-w-0 flex-1 pb-10">
    {@render children()}
  </main>
</div>
