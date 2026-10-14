<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import { untrack } from "svelte";
  import {
    api,
    APIError,
    navigate,
    parseArgs,
    recentDirs,
    store,
    stored,
  } from "../api";
  import { NoContentSchema, SessionInfoSchema, type Preset } from "../schemas";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as Dialog from "$lib/components/ui/dialog";
  import * as Drawer from "$lib/components/ui/drawer";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import DirectoryPicker from "./DirectoryPicker.svelte";

  let {
    presets,
    defaultCwd,
    onclose,
  }: { presets: Preset[]; defaultCwd: string; onclose: () => void } = $props();
  const desktop = new MediaQuery("(min-width: 640px)");
  const recent = recentDirs();
  let open = $state(true);
  let cwd = $state(untrack(() => recent[0] || defaultCwd));
  let preset = $state(
    untrack(
      () =>
        (stored("lastPreset") === "custom"
          ? "custom"
          : presets.find((p) => p.name === stored("lastPreset"))?.name) ||
        presets[0]?.name ||
        "shell",
    ),
  );
  let args = $state("");
  let command = $state(stored("customCommand"));
  let name = $state("");
  let mkdirOpen = $state(false);
  let error = $state("");
  let busy = $state(false);
  const chosen = $derived(presets.find((p) => p.name === preset));
  const custom = $derived(preset === "custom");

  async function create(makeDirectory = false) {
    busy = true;
    error = "";
    try {
      const cmd = custom ? parseArgs(command) : undefined;
      if (custom && (!cmd?.length || !cmd[0]))
        throw new Error("请输入可执行程序及参数");
      const payload = {
        cwd,
        preset,
        args: !custom && chosen?.allowArgs ? parseArgs(args) : [],
        ...(custom ? { cmd } : {}),
        name,
      };
      if (makeDirectory)
        await api("/fs/mkdir", NoContentSchema, "POST", { path: cwd });
      const session = await api(
        "/sessions",
        SessionInfoSchema,
        "POST",
        payload,
      );
      store(
        "recentDirs",
        JSON.stringify(
          [session.cwd, ...recent.filter((p) => p !== session.cwd)].slice(
            0,
            10,
          ),
        ),
      );
      store("lastPreset", preset);
      if (custom) store("customCommand", command);
      navigate("/s/" + session.id);
    } catch (err) {
      if (
        !makeDirectory &&
        err instanceof APIError &&
        err.status === 400 &&
        err.message.startsWith("directory:") &&
        err.message.includes("no such file or directory")
      ) {
        mkdirOpen = true;
      } else error = (err as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

{#snippet fields()}
  <form
    class="space-y-5"
    onsubmit={(event) => {
      event.preventDefault();
      void create();
    }}
  >
    <div class="space-y-2">
      <Label for="cwd">工作目录</Label>
      <div class="flex gap-2">
        <Input
          id="cwd"
          bind:value={cwd}
          placeholder="~/projects"
          required
          autocomplete="off"
          autocapitalize="off"
          autocorrect="off"
          spellcheck={false}
        />
        <DirectoryPicker bind:value={cwd} />
      </div>
      {#if recent.length}
        <div class="flex flex-wrap gap-1" aria-label="最近目录">
          {#each recent.slice(0, 5) as dir (dir)}<Button
              type="button"
              variant="ghost"
              size="sm"
              class="max-w-full truncate text-xs"
              title={dir}
              onclick={() => {
                cwd = dir;
              }}>{dir.split("/").pop() || "/"}</Button
            >{/each}
        </div>
      {/if}
    </div>
    <fieldset class="space-y-2">
      <legend class="mb-2 text-sm font-medium">启动命令</legend>
      <div class="flex flex-wrap gap-2">
        {#each presets as p (p.name)}
          <Button
            type="button"
            variant={preset === p.name ? "default" : "outline"}
            aria-pressed={preset === p.name}
            onclick={() => {
              preset = p.name;
              args = "";
            }}>{p.label}</Button
          >
        {/each}
        <Button
          type="button"
          variant={custom ? "default" : "outline"}
          aria-pressed={custom}
          onclick={() => {
            preset = "custom";
            args = "";
          }}>自定义</Button
        >
      </div>
      {#if custom}
        <div class="space-y-2">
          <Label for="custom-command">自定义启动命令</Label>
          <Input
            id="custom-command"
            bind:value={command}
            placeholder="omp --resume 或 bash -lc 'npm run dev'"
            required
            autocomplete="off"
            autocapitalize="off"
            autocorrect="off"
            spellcheck={false}
          />
          <p class="text-xs text-muted-foreground">
            支持引号和转义；直接执行程序，不展开 shell 变量或管道。需要 shell
            语法时请使用 bash -lc '命令'。
          </p>
        </div>
      {:else}
        <p class="overflow-x-auto rounded-md bg-muted p-3 font-mono text-xs">
          {chosen?.cmd.join(" ")}{args ? " " + args : ""}
        </p>
      {/if}
    </fieldset>
    {#if chosen?.allowArgs}
      <div class="space-y-2">
        <Label for="args">追加参数</Label><Input
          id="args"
          bind:value={args}
          placeholder="--resume 或带引号的参数"
          autocapitalize="off"
          autocorrect="off"
          spellcheck={false}
        />
      </div>
    {/if}
    <div class="space-y-2">
      <Label for="session-name"
        >会话名称 <span class="text-muted-foreground">可选</span></Label
      ><Input
        id="session-name"
        bind:value={name}
        placeholder="留空则跟随终端标题"
      />
    </div>
    {#if error}<p role="alert" class="text-destructive">{error}</p>{/if}
    <div class="flex justify-end gap-2 pt-2">
      <Button type="button" variant="outline" disabled={busy} onclick={onclose}
        >取消</Button
      >
      <Button type="submit" disabled={busy || (!chosen && !custom)}
        >{busy ? "正在启动…" : "创建并进入 →"}</Button
      >
    </div>
  </form>
{/snippet}

{#if desktop.current}
  <Dialog.Root
    bind:open
    onOpenChange={(value) => {
      if (!value) onclose();
    }}
  >
    <Dialog.Content class="max-h-[90dvh] overflow-y-auto p-6 sm:max-w-lg">
      <Dialog.Header
        ><Dialog.Title>开启一个工作空间</Dialog.Title><Dialog.Description
          >选择目录和命令。浏览器断开后，会话仍会继续。</Dialog.Description
        ></Dialog.Header
      >
      {@render fields()}
    </Dialog.Content>
  </Dialog.Root>
{:else}
  <Drawer.Root
    bind:open
    onOpenChange={(value) => {
      if (!value) onclose();
    }}
  >
    <Drawer.Content class="data-[vaul-drawer-direction=bottom]:max-h-[95dvh]">
      <Drawer.Header
        ><Drawer.Title>开启一个工作空间</Drawer.Title><Drawer.Description
          >选择目录和命令，创建持久终端会话。</Drawer.Description
        ></Drawer.Header
      >
      <div
        class="overflow-y-auto px-4 pt-1 pb-[max(1rem,env(safe-area-inset-bottom))]"
      >
        {@render fields()}
      </div>
    </Drawer.Content>
  </Drawer.Root>
{/if}

<AlertDialog.Root bind:open={mkdirOpen}>
  <AlertDialog.Content>
    <AlertDialog.Header
      ><AlertDialog.Title>目录不存在，是否创建？</AlertDialog.Title
      ><AlertDialog.Description class="break-all"
        >将在 {cwd} 创建目录，然后启动会话。</AlertDialog.Description
      ></AlertDialog.Header
    >
    <AlertDialog.Footer>
      <AlertDialog.Cancel>取消</AlertDialog.Cancel>
      <AlertDialog.Action
        onclick={() => {
          void create(true);
        }}>创建目录并启动</AlertDialog.Action
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
