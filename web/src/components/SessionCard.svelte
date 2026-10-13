<script lang="ts">
  import { navigate, statusText } from "../api";
  import type { SessionInfo } from "../schemas";
  import { Button } from "$lib/components/ui/button";
  import { Badge } from "$lib/components/ui/badge";
  import * as Card from "$lib/components/ui/card";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";

  let {
    session,
    now,
    onclose,
  }: {
    session: SessionInfo;
    now: number;
    onclose: (session: SessionInfo) => void;
  } = $props();
  const seconds = $derived(
    Math.max(0, Math.floor((now - Date.parse(session.lastActiveAt)) / 1000)),
  );
  const relative = $derived(
    seconds < 60
      ? "刚刚"
      : seconds < 3600
        ? `${Math.floor(seconds / 60)} 分钟前`
        : `${Math.floor(seconds / 3600)} 小时前`,
  );
  const cwd = $derived(
    session.cwd.length > 58
      ? session.cwd.slice(0, 25) + "…" + session.cwd.slice(-28)
      : session.cwd,
  );
</script>

<Card.Root
  class="session-card group relative gap-0 overflow-hidden py-0 transition-colors hover:border-primary/50"
>
  <Card.Header class="flex flex-row items-start gap-3 px-5 pt-5 pb-3">
    <div class="min-w-0 flex-1">
      <a
        class="session-link block truncate text-base font-semibold after:absolute after:inset-0 after:content-['']"
        href={`/s/${session.id}`}
        onclick={(event) => {
          if (
            event.button === 0 &&
            !event.ctrlKey &&
            !event.metaKey &&
            !event.shiftKey &&
            !event.altKey
          ) {
            event.preventDefault();
            navigate(`/s/${session.id}`);
          }
        }}>{session.title}</a
      >
      <div class="mt-2 flex items-center gap-2 text-xs text-muted-foreground">
        <span class={`status-dot status-${session.status}`}></span>
        <span
          >{statusText[session.status]}{session.exitCode !== null
            ? ` · code ${session.exitCode}`
            : ""}</span
        >
        <span class="ml-auto">{relative}</span>
      </div>
    </div>
    <DropdownMenu.Root>
      <DropdownMenu.Trigger>
        {#snippet child({ props })}
          <Button
            {...props}
            variant="ghost"
            size="icon-sm"
            class="relative z-10 -mt-1 -mr-2"
            aria-label={`会话菜单 ${session.title}`}>⋯</Button
          >
        {/snippet}
      </DropdownMenu.Trigger>
      <DropdownMenu.Content align="end">
        <DropdownMenu.Item
          variant="destructive"
          onSelect={() => onclose(session)}
          >{session.status === "exited"
            ? "移除会话"
            : "关闭会话"}</DropdownMenu.Item
        >
      </DropdownMenu.Content>
    </DropdownMenu.Root>
  </Card.Header>
  <Card.Content class="space-y-4 px-5 pb-4">
    <p
      class="truncate font-mono text-xs text-muted-foreground"
      title={session.cwd}
    >
      {cwd}
    </p>
    <div class="flex min-w-0 items-center gap-2">
      <Badge variant="secondary">{session.preset}</Badge>
      <code
        class="truncate text-xs text-muted-foreground"
        title={session.cmd.join(" ")}>{session.cmd.join(" ")}</code
      >
    </div>
    <p
      class="truncate rounded-md border bg-background px-3 py-2 font-mono text-xs text-muted-foreground"
    >
      {session.lastLine || "等待终端输出…"}
    </p>
  </Card.Content>
  <Card.Footer
    class="justify-between border-t px-5 py-3 text-xs text-muted-foreground"
  >
    <span
      >{session.clients ? `${session.clients} 个设备已连接` : "后台会话"}</span
    >
    <span class="text-primary">打开终端 →</span>
  </Card.Footer>
</Card.Root>
