<script lang="ts">
  import { onMount } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as Card from "$lib/components/ui/card";
  import { PreviewSettingsSchema } from "../schemas";
  import {
    defaultPreviewSettings,
    previewSettings,
    updatePreviewSettings,
  } from "../previewSettings.svelte";
  import { selectPreview } from "../preview";

  let lines = $state<number | undefined>(previewSettings.lines);
  let skip = $state<number | undefined>(previewSettings.skip);
  const validation = $derived(PreviewSettingsSchema.safeParse({ lines, skip }));
  const example = [
    ...Array.from({ length: 12 }, (_, index) => `输出第 ${index + 1} 行`),
    "模型 · 工作目录 · 状态栏",
    "❯ 输入提示符",
  ].join("\n");
  const preview = $derived(selectPreview(example, previewSettings));

  $effect(() => {
    if (validation.success) updatePreviewSettings(validation.data);
  });
  onMount(() => {
    document.title = "WebMux · 设置";
  });
</script>

<div class="min-h-dvh">
  <header class="border-b bg-card/60">
    <div
      class="mx-auto flex min-h-20 max-w-6xl items-center justify-between gap-3 px-4 py-3 sm:px-8"
    >
      <a class="brand" href="/"><span class="brand-mark">WM</span> WebMux</a>
      <Button href="/" variant="outline">返回工作区</Button>
    </div>
  </header>
  <main class="mx-auto max-w-3xl space-y-6 px-4 py-8 sm:px-8 sm:py-10">
    <div>
      <h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">设置</h1>
      <p class="mt-2 text-sm text-muted-foreground">
        设置仅保存在当前浏览器，修改后自动保存。不会改变终端内容或其他设备的显示。
      </p>
    </div>
    <Card.Root>
      <Card.Header>
        <Card.Title>会话输出预览</Card.Title>
        <Card.Description
          >从当前终端屏幕的底部向上选取非空行，可隐藏 agent
          的状态栏和输入提示符。</Card.Description
        >
      </Card.Header>
      <Card.Content class="space-y-6">
        <div class="grid gap-5 sm:grid-cols-2">
          <div class="space-y-2">
            <Label for="preview-lines">显示行数</Label>
            <Input
              id="preview-lines"
              type="number"
              min={1}
              max={50}
              step={1}
              bind:value={lines}
              aria-invalid={!validation.success}
              aria-describedby="preview-lines-help"
            />
            <p id="preview-lines-help" class="text-xs text-muted-foreground">
              1–50 行；内容不足时保留空余高度。
            </p>
          </div>
          <div class="space-y-2">
            <Label for="preview-skip">跳过末尾行数</Label>
            <Input
              id="preview-skip"
              type="number"
              min={0}
              max={50}
              step={1}
              bind:value={skip}
              aria-invalid={!validation.success}
              aria-describedby="preview-skip-help"
            />
            <p id="preview-skip-help" class="text-xs text-muted-foreground">
              0–50 行；0 表示不隐藏末尾内容。
            </p>
          </div>
        </div>
        {#if !validation.success}
          <p role="alert" class="text-sm text-destructive">
            请输入有效整数：显示 1–50 行，跳过 0–50
            行。无效输入不会覆盖已保存设置。
          </p>
        {/if}
        <div class="space-y-2">
          <p class="text-sm">
            跳过末尾 {previewSettings.skip} 行，再向上显示 {previewSettings.lines}
            行。
          </p>
          <p class="text-xs text-muted-foreground">
            示例预览（12 行输出 + 2 行底栏，不是实际会话）
          </p>
          <pre
            aria-label="预览效果示例"
            style:height={`calc(${previewSettings.lines}lh + 1rem)`}
            class="min-w-0 overflow-hidden bg-muted p-2 font-mono text-xs leading-5 whitespace-pre text-foreground dark:bg-background">{preview ||
              "末尾行已隐藏，暂无可显示内容"}</pre>
        </div>
        <Button
          variant="outline"
          onclick={() => {
            lines = defaultPreviewSettings.lines;
            skip = defaultPreviewSettings.skip;
          }}
          >恢复默认（{defaultPreviewSettings.lines} 行，跳过 {defaultPreviewSettings.skip}
          行）</Button
        >
      </Card.Content>
    </Card.Root>
  </main>
</div>
