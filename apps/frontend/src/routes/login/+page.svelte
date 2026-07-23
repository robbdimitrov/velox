<script lang="ts">
  import { Mail, Lock, ArrowRight } from '@lucide/svelte';
  import AuthCard from '$lib/components/AuthCard.svelte';
  import { pageTitle } from '$lib/pageTitle';
  import PrimaryButton from '$lib/components/PrimaryButton.svelte';
  import TextField from '$lib/components/TextField.svelte';

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
    } catch (err: unknown) {
      error = err instanceof Error ? err.message : 'Login failed';
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>{pageTitle('Log in')}</title>
</svelte:head>

<div class="flex min-h-[80vh] items-center justify-center">
  <AuthCard
    title="Welcome Back"
    description="Log in to access your reservation tickets."
  >
    {#if error}
      <div
        class="border-urgency/50 bg-urgency/20 text-urgency mb-6 rounded-sm border p-3 text-sm backdrop-blur-sm"
      >
        {error}
      </div>
    {/if}

    <form onsubmit={handleSubmit} class="space-y-6">
      <TextField
        id="email"
        label="Email"
        type="email"
        bind:value={email}
        required
        icon={Mail}
        placeholder="you@example.com"
      />
      <TextField
        id="password"
        label="Password"
        type="password"
        bind:value={password}
        required
        icon={Lock}
        placeholder="••••••••"
      />

      <PrimaryButton type="submit" disabled={loading} flush>
        {#if loading}
          <span class="loading loading-spinner loading-sm"></span>
        {:else}
          Log In <ArrowRight size={18} />
        {/if}
      </PrimaryButton>
    </form>

    <p class="text-inkMuted mt-6 text-center text-sm">
      Don't have an account?
      <a
        href="/register"
        class="text-signal hover:text-signal/80 font-semibold transition-colors"
        >Register here</a
      >
    </p>
  </AuthCard>
</div>
