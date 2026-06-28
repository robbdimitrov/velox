<script lang="ts">
  import {
    Mail,
    Lock,
    User as UserIcon,
    ArrowRight,
    BriefcaseBusiness
  } from '@lucide/svelte';
  let email = $state('');
  let password = $state('');
  let name = $state('');
  let role = $state('customer');
  let loading = $state(false);
  let error = $state('');

  async function handleSubmit(e: Event) {
    e.preventDefault();
    loading = true;
    error = '';

    try {
      const res = await fetch('/api/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password, name, role })
      });

      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.message || 'Registration failed');
      }

      window.location.href = '/';
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>Register - Velox</title>
</svelte:head>

<div class="flex items-center justify-center min-h-[80vh] px-4">
  <div
    class="glass-panel w-full max-w-md p-8 rounded-3xl shadow-glow relative overflow-hidden group"
  >
    <div
      class="absolute inset-0 bg-gradient-to-br from-signal/10 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none"
    ></div>
    <div class="relative z-10">
      <h1 class="text-3xl font-black mb-2 tracking-tight uppercase">
        Join Velox
      </h1>
      <p class="text-inkMuted mb-8 text-sm">
        Create an account to start reserving seats or hosting events.
      </p>

      {#if error}
        <div
          class="bg-danger/20 border border-danger/50 text-danger p-3 rounded-xl mb-6 text-sm backdrop-blur-sm animate-pulse"
        >
          {error}
        </div>
      {/if}

      <form onsubmit={handleSubmit} class="space-y-5">
        <div class="grid grid-cols-2 gap-4 mb-6">
          <button
            type="button"
            onclick={() => (role = 'customer')}
            class="flex flex-col items-center justify-center gap-2 p-4 rounded-xl border transition-all duration-300 {role ===
            'customer'
              ? 'bg-signal/20 border-signal text-white shadow-inner shadow-signal/20'
              : 'bg-black/40 border-white/10 text-inkMuted hover:border-white/30'}"
          >
            <UserIcon
              size={24}
              class={role === 'customer' ? 'text-signal' : 'text-current'}
            />
            <span class="text-sm font-semibold">Regular User</span>
          </button>

          <button
            type="button"
            onclick={() => (role = 'vendor')}
            class="flex flex-col items-center justify-center gap-2 p-4 rounded-xl border transition-all duration-300 {role ===
            'vendor'
              ? 'bg-info/20 border-info text-white shadow-inner shadow-info/20'
              : 'bg-black/40 border-white/10 text-inkMuted hover:border-white/30'}"
          >
            <BriefcaseBusiness
              size={24}
              class={role === 'vendor' ? 'text-info' : 'text-current'}
            />
            <span class="text-sm font-semibold">Organizer</span>
          </button>
        </div>

        <div class="space-y-2">
          <label
            class="text-xs font-semibold uppercase tracking-wider text-inkMuted"
            for="name">Full Name</label
          >
          <div class="relative flex items-center">
            <UserIcon size={18} class="absolute left-3 text-inkMuted" />
            <input
              id="name"
              type="text"
              bind:value={name}
              required
              class="w-full bg-black/40 border border-white/10 rounded-xl py-3 pl-10 pr-4 text-ink placeholder:text-inkMuted/50 focus:border-signal focus:ring-1 focus:ring-signal transition-all outline-none shadow-inner"
              placeholder="John Doe"
            />
          </div>
        </div>

        <div class="space-y-2">
          <label
            class="text-xs font-semibold uppercase tracking-wider text-inkMuted"
            for="email">Email</label
          >
          <div class="relative flex items-center">
            <Mail size={18} class="absolute left-3 text-inkMuted" />
            <input
              id="email"
              type="email"
              bind:value={email}
              required
              class="w-full bg-black/40 border border-white/10 rounded-xl py-3 pl-10 pr-4 text-ink placeholder:text-inkMuted/50 focus:border-signal focus:ring-1 focus:ring-signal transition-all outline-none shadow-inner"
              placeholder="you@example.com"
            />
          </div>
        </div>

        <div class="space-y-2">
          <label
            class="text-xs font-semibold uppercase tracking-wider text-inkMuted"
            for="password">Password</label
          >
          <div class="relative flex items-center">
            <Lock size={18} class="absolute left-3 text-inkMuted" />
            <input
              id="password"
              type="password"
              bind:value={password}
              required
              class="w-full bg-black/40 border border-white/10 rounded-xl py-3 pl-10 pr-4 text-ink placeholder:text-inkMuted/50 focus:border-signal focus:ring-1 focus:ring-signal transition-all outline-none shadow-inner"
              placeholder="••••••••"
            />
          </div>
        </div>

        <button
          type="submit"
          disabled={loading}
          class="w-full btn border-none bg-gradient-to-r from-signal to-accent text-white rounded-xl shadow-lg shadow-signal/20 hover:shadow-signal/40 hover:scale-[1.02] transition-all flex items-center justify-center gap-2 mt-4"
        >
          {#if loading}
            <span class="loading loading-spinner loading-sm"></span>
          {:else}
            Create Account <ArrowRight size={18} />
          {/if}
        </button>
      </form>

      <p class="mt-6 text-center text-sm text-inkMuted">
        Already have an account?
        <a
          href="/login"
          class="text-signal hover:text-signal/80 font-semibold transition-colors"
          >Sign in</a
        >
      </p>
    </div>
  </div>
</div>
