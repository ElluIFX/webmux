<script lang="ts">
  import {
    IconArrowUp,
    IconArrowDown,
    IconArrowsVertical,
  } from "@tabler/icons-svelte";
  import { Button } from "$lib/components/ui/button";
  import type { CodeBlock } from "../code-diff-model";

  let {
    gap,
    up,
    down,
    canExpand,
    onexpand,
  }: {
    gap: CodeBlock;
    up: boolean;
    down: boolean;
    canExpand: () => boolean;
    onexpand: (direction: "up" | "down" | "all") => void;
  } = $props();
</script>

<div
  class="flex h-9 items-center gap-1 border-y border-border/60 bg-muted/60 px-1 text-muted-foreground"
  contenteditable="false"
>
  {#if gap.count <= 60}
    <Button
      variant="ghost"
      size="icon"
      aria-label={`展开隐藏的 ${gap.count} 行`}
      title="展开此段"
      disabled={!canExpand()}
      onclick={() => onexpand("all")}
    >
      <IconArrowsVertical class="size-4" aria-hidden="true" />
    </Button>
  {:else}
    {#if up}
      <Button
        variant="ghost"
        size="icon"
        aria-label="向上展开 30 行"
        title="向上展开 30 行"
        disabled={!canExpand()}
        onclick={() => onexpand("up")}
      >
        <IconArrowUp class="size-4" aria-hidden="true" />
      </Button>
    {/if}
    {#if down}
      <Button
        variant="ghost"
        size="icon"
        aria-label="向下展开 30 行"
        title="向下展开 30 行"
        disabled={!canExpand()}
        onclick={() => onexpand("down")}
      >
        <IconArrowDown class="size-4" aria-hidden="true" />
      </Button>
    {/if}
  {/if}
  <span class="select-none px-2 text-xs">隐藏 {gap.count} 行</span>
</div>
