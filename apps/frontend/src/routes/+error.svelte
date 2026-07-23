<script lang="ts">
  import { page } from '$app/state';
  import AuthCard from '$lib/components/AuthCard.svelte';
  import ActionLink from '$lib/components/ActionLink.svelte';
  import { pageTitle } from '$lib/pageTitle';

  const notFoundMessage = "This page doesn't exist or may have moved.";
</script>

<svelte:head>
  <title>{pageTitle(`${page.status}`)}</title>
</svelte:head>

<div class="flex min-h-[70vh] items-center justify-center">
  <AuthCard
    title={page.status === 404 ? 'Page not found' : `Error ${page.status}`}
    description={page.status === 404
      ? notFoundMessage
      : (page.error?.message ?? 'Something went wrong loading this page.')}
  >
    <ActionLink href="/" size="lg">Return home</ActionLink>
  </AuthCard>
</div>
