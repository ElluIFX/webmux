import { api, navigate, statusText, type SessionInfo } from "../api";
export async function closeSession(session: SessionInfo) {
  if (
    !confirm(
      session.status === "exited"
        ? "移除此会话及其缓冲？"
        : `关闭“${session.title}”？正在运行的进程将终止。`,
    )
  )
    return;
  await api(
    `/sessions/${session.id}${session.status === "exited" ? "?remove=1" : ""}`,
    "DELETE",
  );
}
function relative(time: string) {
  const seconds = Math.max(
    0,
    Math.floor((Date.now() - new Date(time).getTime()) / 1000),
  );
  return seconds < 60
    ? "刚刚"
    : seconds < 3600
      ? `${Math.floor(seconds / 60)} 分钟前`
      : `${Math.floor(seconds / 3600)} 小时前`;
}
export function SessionCard({
  session,
  onError,
}: {
  session: SessionInfo;
  onError: (error: string) => void;
}) {
  const parts = session.cwd.split("/");
  const cwd =
    session.cwd.length > 58
      ? parts.slice(0, 3).join("/") + "/…/" + parts.slice(-2).join("/")
      : session.cwd;
  return (
    <article class="session-card">
      <button class="card-open" onClick={() => navigate("/s/" + session.id)}>
        <div class="card-top">
          <span class={"status-dot status-" + session.status} />
          <span class="session-title">{session.title}</span>
          <span class="chevron">↗</span>
        </div>
        <div class="session-status">
          {statusText[session.status]}
          {session.exitCode !== null ? ` · code ${session.exitCode}` : ""}
          <span>{relative(session.lastActiveAt)}</span>
        </div>
        <div class="cwd" title={session.cwd}>
          {cwd}
        </div>
        <div class="command" title={session.cmd.join(" ")}>
          <span class="preset">{session.preset}</span>
          <code>{session.cmd.join(" ")}</code>
        </div>
        <p class="preview">{session.lastLine || "等待终端输出…"}</p>
        <div class="card-bottom">
          <span>
            {session.clients ? `${session.clients} 个设备已连接` : "后台会话"}
          </span>
          <span>打开终端 →</span>
        </div>
      </button>
      <button
        class="card-close"
        aria-label={
          session.status === "exited"
            ? "移除 " + session.title
            : "关闭 " + session.title
        }
        title="关闭 / 移除会话"
        onClick={() =>
          void closeSession(session).catch((e) => onError((e as Error).message))
        }
      >
        ×
      </button>
    </article>
  );
}
