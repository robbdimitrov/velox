<script lang="ts">
  import { BriefcaseBusiness, Mail, Shield, User } from '@lucide/svelte';
  import Panel from '$lib/components/Panel.svelte';
  import { pageTitle } from '$lib/pageTitle';
  import ThemePreferencePicker from '$lib/components/ThemePreferencePicker.svelte';

  let { data } = $props();

  const roleLabel: Record<string, string> = {
    reserver: 'Reserver',
    organizer: 'Organizer',
    admin: 'Admin'
  };
</script>

<svelte:head>
  <title>{pageTitle('Profile')}</title>
</svelte:head>

<div class="mb-8 flex items-end justify-between">
  <div>
    <h1 class="text-3xl font-black tracking-tight uppercase">Profile</h1>
    <p class="text-inkMuted mt-1 text-sm">
      Manage your account and preferences.
    </p>
  </div>
</div>

<div class="grid gap-6">
  <Panel padding="lg">
    <div class="border-line mb-6 border-b pb-4">
      <h2 class="text-ink text-sm font-black tracking-wider uppercase">
        Account
      </h2>
    </div>

    <dl class="grid gap-4 sm:grid-cols-2">
      <div class="flex items-center gap-3">
        <div
          class="border-line bg-panelSoft text-signal grid h-9 w-9 shrink-0 place-items-center rounded-sm border"
        >
          <User size={16} />
        </div>
        <div class="min-w-0">
          <dt class="text-inkMuted text-[10px] font-semibold uppercase">
            Name
          </dt>
          <dd class="text-ink truncate font-semibold">
            {data.user.name ?? 'Not set'}
          </dd>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <div
          class="border-line bg-panelSoft text-signal grid h-9 w-9 shrink-0 place-items-center rounded-sm border"
        >
          <Mail size={16} />
        </div>
        <div class="min-w-0">
          <dt class="text-inkMuted text-[10px] font-semibold uppercase">
            Email
          </dt>
          <dd class="text-ink truncate font-semibold">{data.user.email}</dd>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <div
          class="border-line bg-panelSoft text-signal grid h-9 w-9 shrink-0 place-items-center rounded-sm border"
        >
          <Shield size={16} />
        </div>
        <div class="min-w-0">
          <dt class="text-inkMuted text-[10px] font-semibold uppercase">
            Role
          </dt>
          <dd class="text-ink truncate font-semibold">
            {roleLabel[data.user.role] ?? data.user.role}
          </dd>
        </div>
      </div>

      {#if data.user.role === 'organizer' || data.user.role === 'admin'}
        <div class="flex items-center gap-3">
          <div
            class="border-line bg-panelSoft text-signal grid h-9 w-9 shrink-0 place-items-center rounded-sm border"
          >
            <BriefcaseBusiness size={16} />
          </div>
          <div class="min-w-0">
            <dt class="text-inkMuted text-[10px] font-semibold uppercase">
              Organizer tools
            </dt>
            <dd>
              <a
                href="/organizer"
                class="text-signal hover:text-signal/80 font-semibold"
                >Go to dashboard</a
              >
            </dd>
          </div>
        </div>
      {/if}
    </dl>
  </Panel>

  <Panel padding="lg">
    <div class="border-line mb-6 border-b pb-4">
      <h2 class="text-ink text-sm font-black tracking-wider uppercase">
        Theme Preference
      </h2>
      <p class="text-inkMuted mt-1 text-sm">
        Choose how Velox should render on this device.
      </p>
    </div>

    <ThemePreferencePicker />
  </Panel>
</div>
