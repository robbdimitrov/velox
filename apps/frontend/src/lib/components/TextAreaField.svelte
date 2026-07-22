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
  <span class="text-inkMuted text-xs font-semibold tracking-wider uppercase">
    {label}
  </span>
  <textarea
    {id}
    bind:value
    {rows}
    {required}
    class="border-line bg-carbon/60 text-ink placeholder:text-inkMuted/50 focus:border-signal focus:ring-signal/50 w-full resize-none rounded-sm border px-4 py-3 shadow-inner transition-colors outline-none focus:ring-1"
    {placeholder}></textarea>
</label>
