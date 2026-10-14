<script lang="ts">
  import { onMount } from "svelte";
  import RiSunLine from "remixicon-svelte/icons/sun-line";
  import RiMoonLine from "remixicon-svelte/icons/moon-line";
  import { toggleTheme } from "../theme";
  import { api } from "../api";
  import {
    PresetsSchema,
    ConfigSchema,
    SessionEventSchema,
    type Preset,
    type SessionInfo,
  } from "../schemas";
  import { connect, type Connection } from "../ws";
  import { Button } from "$lib/components/ui/button";
  import { Badge } from "$lib/components/ui/badge";
  import { Skeleton } from "$lib/components/ui/skeleton";
  import SessionCard from "../components/SessionCard.svelte";
  import NewSessionSheet from "../components/NewSessionSheet.svelte";
  import CloseSessionDialog from "../components/CloseSessionDialog.svelte";

  let sessions = $state<SessionInfo[]>([]);
  let presets = $state<Preset[]>([]);
  let cwd = $state("~");
  let loading = $state(true);
  let connection = $state<Connection>("connecting");
  let creating = $state(false);
  let closing = $state<SessionInfo | null>(null);
  let error = $state("");
  let now = $state(Date.now());
  let dark = $state(document.documentElement.classList.contains("dark"));
  const list = $derived(
    [...sessions].sort(
      (a, b) => Date.parse(b.lastActiveAt) - Date.parse(a.lastActiveAt),
    ),
  );

  onMount(() => {
    document.title = "WebMux · 会话";
    const theme = new MutationObserver(() => {
      dark = document.documentElement.classList.contains("dark");
    });
    theme.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    });
    let active = true;
    void Promise.all([
      api("/presets", PresetsSchema),
      api("/config", ConfigSchema),
    ])
      .then(([p, c]) => {
        if (active) {
          presets = p;
          cwd = c.defaultCwd;
        }
      })
      .catch((err) => {
        if (active) error = (err as Error).message;
      });
    const ws = connect(
      "/ws/sessions",
      (data) => {
        if (typeof data !== "string")
          throw new Error("会话列表帧必须是 JSON 文本");
        const event = SessionEventSchema.parse(JSON.parse(data));
        if (event.type === "snapshot") {
          sessions = event.sessions;
          loading = false;
        } else if (event.type === "created" || event.type === "updated") {
          sessions = [
            ...sessions.filter((s) => s.id !== event.session.id),
            event.session,
          ];
        } else if (event.type === "removed") {
          sessions = sessions.filter((s) => s.id !== event.id);
        }
        now = Date.now();
      },
      (state) => {
        connection = state;
      },
      (message) => {
        error = message;
      },
    );
    const clock = setInterval(() => {
      now = Date.now();
    }, 30000);
    return () => {
      active = false;
      theme.disconnect();
      ws.close();
      clearInterval(clock);
    };
  });
</script>

<div class="min-h-dvh">
  <header class="border-b bg-card/60">
    <div
      class="mx-auto flex min-h-20 max-w-6xl flex-wrap items-center justify-between gap-4 px-4 py-3 sm:px-8"
    >
      <a class="brand" href="/"><span class="brand-mark">WM</span> WebMux</a>
      <div class="ml-auto flex items-center gap-2 sm:gap-4">
        <span
          class="hidden items-center gap-2 text-xs text-muted-foreground sm:flex"
          ><span
            class={`status-dot ${connection === "connected" ? "status-running" : "status-waiting"}`}
          ></span>{connection === "connected" ? "本机在线" : "正在连接"}</span
        >
        <Button
          disabled={!presets.length}
          onclick={() => {
            creating = true;
          }}>＋ 新建会话</Button
        >
        <Button
          variant="outline"
          size="sm"
          class="shrink-0"
          aria-label={dark ? "切换为白色主题" : "切换为黑色主题"}
          title={dark ? "切换为白色主题" : "切换为黑色主题"}
          onclick={toggleTheme}
        >
          {#if dark}<RiSunLine class="size-4" />白色
          {:else}<RiMoonLine class="size-4" />黑色{/if}
        </Button>
      </div>
    </div>
  </header>
  {#if connection === "reconnecting"}<div role="status" class="notice-warning">
      连接已断开，重连中…
    </div>{/if}
  <main class="mx-auto max-w-6xl px-4 py-9 sm:px-8 sm:py-12">
    <div class="mb-8 flex items-end justify-between gap-4">
      <div>
        <p class="mb-3 font-mono text-[10px] tracking-[0.18em] text-primary">
          YOUR WORKSPACES
        </p>
        <h1
          class="flex items-center gap-3 text-2xl font-semibold tracking-tight sm:text-3xl"
        >
          继续你的工作 <Badge variant="secondary">{list.length}</Badge>
        </h1>
        <p class="mt-2 text-sm text-muted-foreground">
          连接可以断开，工作不会停下。
        </p>
      </div>
      <span class="hidden text-xs text-muted-foreground md:block"
        >持久会话 · 内存缓冲</span
      >
    </div>
    {#if error}<p role="alert" class="mb-4 text-destructive">{error}</p>{/if}
    {#if loading}
      <div
        class="grid gap-4 md:grid-cols-2 lg:grid-cols-3"
        aria-label="正在加载会话"
      >
        {#each [0, 1, 2] as item (item)}<Skeleton
            class="h-56 w-full rounded-xl"
          />{/each}
      </div>
    {:else if list.length}
      <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {#each list as session (session.id)}<SessionCard
            {session}
            {now}
            onclose={(value) => {
              closing = value;
            }}
          />{/each}
      </div>
    {:else}
      <section
        class="flex min-h-80 flex-col items-center justify-center rounded-xl border border-dashed bg-card/40 p-6 text-center"
      >
        <span class="mb-5 font-mono text-4xl text-primary">›_</span>
        <h2 class="text-xl font-medium">还没有会话</h2>
        <p class="mt-2 mb-6 text-sm text-muted-foreground">
          创建一个终端，让你的 agent 在后台继续工作。
        </p>
        <Button
          disabled={!presets.length}
          onclick={() => {
            creating = true;
          }}>＋ 新建会话</Button
        >
      </section>
    {/if}
    <footer
      class="mt-10 flex flex-wrap justify-between gap-2 border-t pt-5 text-xs text-muted-foreground"
    >
      <span>WebMux</span><span>仅在可信内网使用 · 服务重启会结束所有会话</span>
    </footer>
  </main>
</div>
{#if creating}<NewSessionSheet
    {presets}
    defaultCwd={cwd}
    onclose={() => {
      creating = false;
    }}
  />{/if}
{#if closing}<CloseSessionDialog
    session={closing}
    onclose={() => {
      closing = null;
    }}
  />{/if}
