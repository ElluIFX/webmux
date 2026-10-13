import { useEffect, useState } from "preact/hooks";
import { api, type Preset, type SessionInfo } from "../api";
import { connect, type Connection } from "../ws";
import { SessionCard } from "../components/SessionCard";
import { NewSessionSheet } from "../components/NewSessionSheet";
export function SessionList() {
  const [sessions, setSessions] = useState(new Map<string, SessionInfo>()),
    [presets, setPresets] = useState<Preset[]>([]),
    [cwd, setCwd] = useState("~"),
    [loading, setLoading] = useState(true),
    [connection, setConnection] = useState<Connection>("connecting"),
    [newSession, setNewSession] = useState(false),
    [error, setError] = useState("");
  const [, setClock] = useState(0);
  useEffect(() => {
    document.title = "WebMux · 会话";
    void Promise.all([
      api<Preset[]>("/presets"),
      api<{ defaultCwd: string }>("/config"),
    ])
      .then(([p, c]) => {
        setPresets(p);
        setCwd(c.defaultCwd);
      })
      .catch((e) => setError((e as Error).message));
    const ws = connect(
      "/ws/sessions",
      (data) => {
        if (typeof data !== "string") return;
        const event = JSON.parse(data) as {
          type: string;
          sessions?: SessionInfo[];
          session?: SessionInfo;
          id?: string;
        };
        setSessions((previous) => {
          if (event.type === "snapshot") {
            setLoading(false);
            return new Map((event.sessions || []).map((s) => [s.id, s]));
          }
          const next = new Map(previous);
          if (event.session) next.set(event.session.id, event.session);
          if (event.type === "removed" && event.id) next.delete(event.id);
          return next;
        });
      },
      setConnection,
    );
    const clock = setInterval(() => setClock((v) => v + 1), 30000);
    return () => {
      ws.close();
      clearInterval(clock);
    };
  }, []);
  const list = [...sessions.values()].sort(
    (a, b) => Date.parse(b.lastActiveAt) - Date.parse(a.lastActiveAt),
  );
  return (
    <div class="list-page">
      <header class="page-header">
        <a class="brand" href="/">
          WM<span>WebMux</span>
        </a>
        <div class="header-right">
          <span class="online-label">
            <span
              class={
                "status-dot " +
                (connection === "connected"
                  ? "status-running"
                  : "status-waiting")
              }
            />
            {connection === "connected" ? "本机在线" : "正在连接"}
          </span>
          <button
            class="primary"
            disabled={!presets.length}
            onClick={() => setNewSession(true)}
          >
            ＋ 新建会话
          </button>
        </div>
      </header>
      {connection === "reconnecting" && (
        <div class="warning" role="status">
          连接已断开，重连中…
        </div>
      )}
      <main class="list-content">
        <div class="section-heading">
          <div>
            <span class="eyebrow">YOUR WORKSPACES</span>
            <h1>
              继续你的工作<span class="count">{list.length}</span>
            </h1>
            <p class="muted">连接可以断开，工作不会停下。</p>
          </div>
          <span class="memory-note">持久会话 · 内存缓冲</span>
        </div>
        {error && (
          <p class="error" role="alert">
            {error}
          </p>
        )}
        {loading ? (
          <div class="card-grid">
            {[1, 2, 3].map((i) => (
              <div class="skeleton" key={i} />
            ))}
          </div>
        ) : list.length ? (
          <div class="card-grid">
            {list.map((session) => (
              <SessionCard
                key={session.id}
                session={session}
                onError={setError}
              />
            ))}
          </div>
        ) : (
          <div class="empty">
            <div class="terminal-mark">›_</div>
            <h2>还没有会话</h2>
            <p class="muted">创建一个终端，让你的 agent 在后台继续工作。</p>
            <button
              class="primary"
              disabled={!presets.length}
              onClick={() => setNewSession(true)}
            >
              ＋ 新建会话
            </button>
          </div>
        )}
        <footer class="list-footer">
          <span>WebMux</span>
          <span>仅在可信内网使用 · 服务重启会结束所有会话</span>
        </footer>
      </main>
      {newSession && (
        <NewSessionSheet
          presets={presets}
          defaultCwd={cwd}
          onClose={() => setNewSession(false)}
        />
      )}
    </div>
  );
}
