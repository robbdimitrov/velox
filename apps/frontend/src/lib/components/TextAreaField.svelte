<script lang="ts">
  let {
    id,
    label,
    placeholder = '',
    rows = 3,
    value = $bindable(''),
    required = false
  }: {
    id: string;
    label: string;
    placeholder?: string;
    rows?: number;
    value: string;
    required?: boolean;
  } = $props();
</script>

<label class="form-control space-y-2">
  <span class="text-xs font-semibold uppercase tracking-wider text-inkMuted">
    {label}
  </span>
  <textarea
    {id}
    bind:value
    {rows}
    {required}
    class="w-full resize-none rounded-sm border border-line bg-carbon/60 px-4 py-3 text-ink shadow-inner outline-none transition-colors placeholder:text-inkMuted/50 focus:border-signal focus:ring-1 focus:ring-signal/50"
    {placeholder}></textarea>
</label>
