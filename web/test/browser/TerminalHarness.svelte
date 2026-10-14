<script lang="ts">
  import { useTerminal } from "../../src/hooks/terminal.svelte";

  let container: HTMLDivElement;
  const terminal = useTerminal(() => container, "terminal-render-fixture", {
    paste: () => {
      throw new Error(
        "Unexpected clipboard paste in terminal rendering fixture",
      );
    },
    copySelection: () => {
      throw new Error(
        "Unexpected clipboard copy in terminal rendering fixture",
      );
    },
  });
</script>

<div
  bind:this={container}
  data-testid="terminal-host"
  style="position: fixed; left: 0; top: 0; width: 400px; height: 750px"
></div>
<div style="position: fixed; left: 0; top: 760px">
  <output aria-label="Terminal connection">{terminal.connection}</output>
  <output aria-label="Terminal restoration"
    >{terminal.restoring ? "restoring" : "ready"}</output
  >
  <output aria-label="Terminal error">{terminal.error}</output>
  <button onclick={() => terminal.focus()}>Focus terminal input</button>
</div>
