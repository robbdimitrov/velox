<script lang="ts">
  import { Mail, Lock, User as UserIcon, ArrowRight } from '@lucide/svelte';
  import AuthCard from '$lib/components/AuthCard.svelte';
  import PrimaryButton from '$lib/components/PrimaryButton.svelte';
  import TextField from '$lib/components/TextField.svelte';
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
  <AuthCard
    title="Join Velox"
    description="Create an account to start reserving seats or hosting events."
  >
    {#if error}
      <div
        class="mb-6 rounded-sm border border-urgency/50 bg-urgency/20 p-3 text-sm text-urgency backdrop-blur-sm"
      >
        {error}
      </div>
    {/if}

    <form onsubmit={handleSubmit} class="space-y-5">
      <TextField
        id="name"
        label="Full Name"
        bind:value={name}
        required
        icon={UserIcon}
        placeholder="John Doe"
      />
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

      <PrimaryButton type="submit" disabled={loading}>
        {#if loading}
          <span class="loading loading-spinner loading-sm"></span>
        {:else}
          Create Account <ArrowRight size={18} />
        {/if}
      </PrimaryButton>
    </form>

    <p class="mt-6 text-center text-sm text-inkMuted">
      Already have an account?
      <a
        href="/login"
        class="font-semibold text-signal transition-colors hover:text-signal/80"
        >Log in</a
      >
    </p>
  </AuthCard>
</div>
