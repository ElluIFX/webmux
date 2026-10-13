import { useState } from "preact/hooks";
import { api, apiBuffer, navigate, statusText } from "../api";
import { closeSession } from "../components/SessionCard";
import { useTerminal } from "../hooks/useTerminal";
import { SoftKeys } from "../components/SoftKeys";
import { useViewport } from "../hooks/useViewport";
export function TerminalPage({ id }: { id: string }) {
  const t = useTerminal(id);
  const height = useViewport();
  const [menu, setMenu] = useState(false),
    [error, setError] = useState(""),
    [copyText, setCopyText] = useState<string | null>(null),
    [pasteText, setPasteText] = useState<string | null>(null);
  async function copy() {
    try {
      const text = new TextDecoder().decode(await apiBuffer(id));
      try {
        await navigator.clipboard.writeText(text);
      } catch {
        setCopyText(text);
      }
    } catch (err) {
      setError((err as Error).message);
    }
  }
  async function paste() {
    try {
      t.paste(await navigator.clipboard.readText());
    } catch {
      setPasteText("");
    }
  }
  return (
    <div class="terminal-page" style={{ height: `${height}px` }}>
      <header class="terminal-header">
        <button
          class="back"
          onClick={() => navigate("/")}
          aria-label="返回会话列表"
        >
          ←
        </button>
        <span class="terminal-title">{t.info?.title || "终端"}</span>
        {t.info && (
          <span
            class={"status-dot status-" + t.info.status}
            title={statusText[t.info.status]}
          />
        )}
        <span class="connection" title={t.connection}>
          {t.connection === "connected"
            ? "已连接"
            : t.connection === "exited"
              ? "已退出"
              : "重连中"}
        </span>
        <button
          aria-label="终端菜单"
          aria-expanded={menu}
          onClick={() => setMenu(!menu)}
        >
          ⋯
        </button>
      </header>
      {menu && (
        <div class="terminal-menu">
          <button
            onClick={() => {
              void copy();
              setMenu(false);
            }}
          >
            复制全部缓冲
          </button>
          <button
            onClick={() => {
              t.clear();
              setMenu(false);
            }}
          >
            清屏
          </button>
          <button
            onClick={() => {
              t.resync();
              setMenu(false);
            }}
          >
            重新同步
          </button>
          <label>
            字体大小
            <input
              type="range"
              min="12"
              max="20"
              value={t.fontSize}
              onInput={(e) => t.setFontSize(Number(e.currentTarget.value))}
            />
            <span>{t.fontSize}px</span>
          </label>
          <button
            class="danger"
            onClick={() => {
              if (t.info)
                void closeSession(t.info).catch((e) =>
                  setError((e as Error).message),
                );
              setMenu(false);
            }}
          >
            关闭会话
          </button>
        </div>
      )}
      {(error || t.error) && (
        <div class="error" role="alert">
          {error || t.error}
        </div>
      )}
      {t.connection === "missing" && (
        <div class="error">
          会话不存在或已被移除。
          <button onClick={() => navigate("/")}>返回列表</button>
        </div>
      )}
      {t.exitCode !== null && (
        <div class="exit-banner">
          进程已退出（code {t.exitCode}）
          <button onClick={() => navigate("/")}>返回列表</button>
          <button
            onClick={() => {
              if (confirm("移除会话及其缓冲？"))
                void api(`/sessions/${id}?remove=1`, "DELETE")
                  .then(() => navigate("/"))
                  .catch((e) => setError((e as Error).message));
            }}
          >
            移除会话
          </button>
        </div>
      )}
      <div class="terminal-area">
        <div class="terminal-host" ref={t.container} />
        {t.restoring && t.connection !== "missing" && (
          <div class="restore-overlay" role="status">
            {t.connection === "connected" ? "正在恢复…" : "正在连接…"}
          </div>
        )}
      </div>
      <SoftKeys
        send={t.send}
        modifiers={t.modifiers}
        toggle={t.toggleModifier}
        paste={() => void paste()}
        keyboard={t.keyboard}
      />
      {pasteText !== null && (
        <div class="modal-backdrop">
          <section
            class="sheet"
            role="dialog"
            aria-modal="true"
            aria-labelledby="paste-title"
          >
            <h2 id="paste-title">粘贴到终端</h2>
            <p class="muted">
              当前环境无法直接读取剪贴板，请在下方粘贴。多行文本遵循终端的
              bracketed paste 模式。
            </p>
            <textarea
              autoFocus
              aria-label="粘贴内容"
              value={pasteText}
              onInput={(e) => setPasteText(e.currentTarget.value)}
              autocapitalize="off"
              autocorrect="off"
              spellcheck={false}
            />
            <footer>
              <button
                onClick={() => {
                  setPasteText(null);
                  t.focus();
                }}
              >
                取消
              </button>
              <button
                class="primary"
                onClick={() => {
                  t.paste(pasteText);
                  setPasteText(null);
                  t.focus();
                }}
              >
                发送
              </button>
            </footer>
          </section>
        </div>
      )}
      {copyText !== null && (
        <div class="modal-backdrop">
          <section class="sheet">
            <h2>复制缓冲</h2>
            <p>当前环境不支持剪贴板访问，请长按或全选复制。</p>
            <textarea readOnly value={copyText} />
            <button onClick={() => setCopyText(null)}>关闭</button>
          </section>
        </div>
      )}
    </div>
  );
}
