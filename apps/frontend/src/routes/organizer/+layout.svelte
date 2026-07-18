<script lang="ts">
  import {
    LayoutDashboard,
    Calendar,
    MapPin,
    Settings,
    LogOut
  } from '@lucide/svelte';
  import { page } from '$app/state';

  let { children } = $props();

  const links = [
    { href: '/organizer', icon: LayoutDashboard, label: 'Dashboard' },
    { href: '/organizer/events/new', icon: Calendar, label: 'Create Event' },
    { href: '/organizer/venues', icon: MapPin, label: 'Venues' },
    { href: '/organizer/settings', icon: Settings, label: 'Settings' }
  ];
</script>

<div
  class="app-screen flex min-h-[calc(100vh-6rem)] flex-col gap-6 lg:flex-row"
>
  <aside class="w-full flex-shrink-0 lg:w-64">
    <div
      class="glass-panel flex flex-col p-4 shadow-glow lg:sticky lg:top-28 lg:h-[calc(100vh-8rem)]"
    >
      <div class="mb-8 px-2">
        <h2 class="text-xl font-black uppercase tracking-tight text-signal">
          Organizer Portal
        </h2>
        <p class="text-xs text-inkMuted mt-1">Manage your events and venues</p>
      </div>

      <nav class="flex-1 space-y-2">
        {#each links as link}
          {@const isActive =
            page.url.pathname === link.href ||
            (link.href !== '/organizer' &&
              page.url.pathname.startsWith(link.href))}
          <a
            href={link.href}
            class="flex items-center gap-3 px-4 py-3 rounded transition-all duration-300 {isActive
              ? 'bg-signal/20 text-signal font-semibold shadow-inner shadow-signal/20'
              : 'text-inkMuted hover:bg-white/5 hover:text-ink'}"
          >
            <link.icon size={20} />
            {link.label}
          </a>
        {/each}
      </nav>

      <div class="pt-4 border-t border-white/10 mt-auto">
        <a
          href="/api/auth/logout"
          class="flex items-center gap-3 px-4 py-3 rounded text-urgency/80 hover:bg-urgency/10 hover:text-urgency transition-all duration-300"
        >
          <LogOut size={20} />
          Logout
        </a>
      </div>
    </div>
  </aside>

  <main class="flex-1 min-w-0 pb-10">
    {@render children()}
  </main>
</div>
