import { useEffect, useState } from "preact/hooks";
import {
  api,
  APIError,
  navigate,
  parseArgs,
  recentDirs,
  store,
  stored,
  type Preset,
  type SessionInfo,
} from "../api";
export function NewSessionSheet({
  presets,
  defaultCwd,
  onClose,
}: {
  presets: Preset[];
  defaultCwd: string;
  onClose: () => void;
}) {
  const recent = recentDirs();
  const [cwd, setCwd] = useState(recent[0] || defaultCwd),
    [preset, setPreset] = useState(
      presets.find((p) => p.name === stored("lastPreset"))?.name ||
        presets[0]?.name ||
        "shell",
    ),
    [args, setArgs] = useState(""),
    [name, setName] = useState(""),
    [dirs, setDirs] = useState<string[]>([]),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const chosen = presets.find((p) => p.name === preset);
  useEffect(() => {
    let active = true;
    const timer = setTimeout(
      () =>
        void api<string[]>("/fs/dirs?prefix=" + encodeURIComponent(cwd))
          .then((value) => {
            if (active) setDirs(value);
          })
          .catch(() => {
            if (active) setDirs([]);
          }),
      200,
    );
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [cwd]);
  useEffect(() => {
    const escape = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", escape);
    return () => window.removeEventListener("keydown", escape);
  }, []);
  async function create() {
    setBusy(true);
    setError("");
    try {
      const payload = {
        cwd,
        preset,
        args: chosen?.allowArgs ? parseArgs(args) : [],
        name,
      };
      let session: SessionInfo;
      try {
        session = await api<SessionInfo>("/sessions", "POST", payload);
      } catch (err) {
        if (
          err instanceof APIError &&
          err.status === 400 &&
          err.message.startsWith("directory:") &&
          err.message.includes("no such file or directory") &&
          confirm("目录不存在，是否创建？")
        ) {
          await api("/fs/mkdir", "POST", { path: cwd });
          session = await api<SessionInfo>("/sessions", "POST", payload);
        } else throw err;
      }
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
      navigate("/s/" + session.id);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div
      class="modal-backdrop"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <section
        class="sheet"
        role="dialog"
        aria-modal="true"
        aria-labelledby="new-title"
      >
        <header>
          <div>
            <span class="eyebrow">NEW SESSION</span>
            <h2 id="new-title">开启一个工作空间</h2>
          </div>
          <button onClick={onClose} aria-label="取消">
            ×
          </button>
        </header>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void create();
          }}
        >
          <label>
            工作目录
            <input
              autoFocus
              value={cwd}
              onInput={(e) => setCwd(e.currentTarget.value)}
              placeholder="~/projects"
              required
              list="directory-options"
            />
            <datalist id="directory-options">
              {dirs.map((dir) => (
                <option value={dir} />
              ))}
            </datalist>
          </label>
          {recent.length > 0 && (
            <div class="recent">
              {recent.slice(0, 5).map((dir) => (
                <button type="button" title={dir} onClick={() => setCwd(dir)}>
                  {dir.split("/").pop() || "/"}
                </button>
              ))}
            </div>
          )}
          <label>启动命令</label>
          <div class="presets">
            {presets.map((p) => (
              <button
                type="button"
                class={p.name === preset ? "selected" : ""}
                aria-pressed={p.name === preset}
                onClick={() => {
                  setPreset(p.name);
                  setArgs("");
                }}
              >
                {p.label}
              </button>
            ))}
          </div>
          <code class="actual-command">
            {chosen?.cmd.join(" ")}
            {args ? " " + args : ""}
          </code>
          {chosen?.allowArgs && (
            <label>
              追加参数
              <input
                value={args}
                onInput={(e) => setArgs(e.currentTarget.value)}
                placeholder="--resume 或带引号的参数"
              />
            </label>
          )}
          <label>
            会话名称 <span class="muted">可选</span>
            <input
              value={name}
              onInput={(e) => setName(e.currentTarget.value)}
              placeholder="留空则跟随终端标题"
            />
          </label>
          {error && (
            <p role="alert" class="error">
              {error}
            </p>
          )}
          <footer>
            <button type="button" onClick={onClose}>
              取消
            </button>
            <button class="primary" disabled={busy || !chosen}>
              {busy ? "正在启动…" : "创建并进入 →"}
            </button>
          </footer>
        </form>
      </section>
    </div>
  );
}
