<script lang="ts">
  import type { Snippet } from 'svelte';

  type Padding = 'none' | 'sm' | 'md' | 'lg' | 'xl';
  type Accent = 'none' | 'signal' | 'ok' | 'urgency';

  let {
    padding = 'md',
    accent = 'none',
    sticky = false,
    hMax = false,
    overflowHidden = false,
    flexColumn = false,
    children
  }: {
    padding?: Padding;
    accent?: Accent;
    sticky?: boolean;
    hMax?: boolean;
    overflowHidden?: boolean;
    flexColumn?: boolean;
    children: Snippet;
  } = $props();
</script>

<div
  class="group border-line bg-panel/90 hover:border-signal/40 relative rounded-sm border shadow-xl backdrop-blur-xl transition-colors"
  class:p-4={padding === 'sm'}
  class:p-5={padding === 'md'}
  class:p-6={padding === 'lg'}
  class:p-8={padding === 'xl'}
  class:border-l-4={accent !== 'none'}
  class:border-l-signal={accent === 'signal'}
  class:border-l-ok={accent === 'ok'}
  class:border-l-urgency={accent === 'urgency'}
  class:sticky
  class:top-28={sticky}
  class:h-max={hMax}
  class:overflow-hidden={overflowHidden}
  class:flex={flexColumn}
  class:flex-col={flexColumn}
>
  {@render children()}
</div>
