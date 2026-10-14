<script lang="ts">
  import WorktreeCreateFields from "../../src/components/WorktreeCreateFields.svelte";
  import type { Repository } from "../../src/schemas";

  let { repository }: { repository: Repository } = $props();
  let path = $state("");
  let mode = $state<"new" | "existing">("new");
  let branch = $state("");
  let base = $state("");
  let ready = $state(false);
</script>

<WorktreeCreateFields
  {repository}
  bind:path
  bind:mode
  bind:branch
  bind:base
  bind:ready
/>
<output aria-label="表单就绪">{ready ? "ready" : "not-ready"}</output>
