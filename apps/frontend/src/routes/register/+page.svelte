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
        body: JSON.stringify({ email, password, name })
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

<div class="flex min-h-[80vh] items-center justify-center">
  <div class="glass-panel app-auth relative overflow-hidden p-8 shadow-glow">
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
          class="bg-urgency/20 border border-urgency/50 text-urgency p-3 rounded mb-6 text-sm backdrop-blur-sm animate-pulse"
        >
          {error}
        </div>
      {/if}

      <form onsubmit={handleSubmit} class="space-y-5">
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
              class="velox-field w-full py-3 pl-10 pr-4 shadow-inner placeholder:text-inkMuted/50"
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
              class="velox-field w-full py-3 pl-10 pr-4 shadow-inner placeholder:text-inkMuted/50"
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
              class="velox-field w-full py-3 pl-10 pr-4 shadow-inner placeholder:text-inkMuted/50"
              placeholder="••••••••"
            />
          </div>
        </div>

        <button
          type="submit"
          disabled={loading}
          class="btn velox-action mt-4 w-full rounded transition-all hover:scale-[1.02]"
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
          >Log in</a
        >
      </p>
    </div>
  </div>
</div>
