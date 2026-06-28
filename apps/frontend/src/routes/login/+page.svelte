<script lang="ts">
  import { Mail, Lock, ArrowRight } from '@lucide/svelte';
  let email = $state('');
  let password = $state('');
  let loading = $state(false);
  let error = $state('');

  async function handleSubmit(e: Event) {
    e.preventDefault();
    loading = true;
    error = '';

    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password })
      });

      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.message || 'Login failed');
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
  <title>Login - Velox</title>
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
        Welcome Back
      </h1>
      <p class="text-inkMuted mb-8 text-sm">
        Sign in to access your tickets and reservations.
      </p>

      {#if error}
        <div
          class="bg-danger/20 border border-danger/50 text-danger p-3 rounded-xl mb-6 text-sm backdrop-blur-sm animate-pulse"
        >
          {error}
        </div>
      {/if}

      <form onsubmit={handleSubmit} class="space-y-6">
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
          class="w-full btn border-none bg-gradient-to-r from-signal to-accent text-white rounded-xl shadow-lg shadow-signal/20 hover:shadow-signal/40 hover:scale-[1.02] transition-all flex items-center justify-center gap-2"
        >
          {#if loading}
            <span class="loading loading-spinner loading-sm"></span>
          {:else}
            Sign In <ArrowRight size={18} />
          {/if}
        </button>
      </form>

      <p class="mt-6 text-center text-sm text-inkMuted">
        Don't have an account?
        <a
          href="/register"
          class="text-signal hover:text-signal/80 font-semibold transition-colors"
          >Register here</a
        >
      </p>
    </div>
  </div>
</div>
