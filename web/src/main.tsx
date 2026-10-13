import { render } from "preact";
import { useEffect, useState } from "preact/hooks";
import { api, store, stored } from "./api";
import { SessionList } from "./pages/SessionList";
import { TerminalPage } from "./pages/Terminal";
import "./styles.css";
function App() {
  const [path, setPath] = useState(location.pathname),
    [ready, setReady] = useState(false),
    [checking, setChecking] = useState(true),
    [token, setToken] = useState(stored("token")),
    [error, setError] = useState("");
  useEffect(() => {
    const change = () => setPath(location.pathname);
    const auth = () => {
      setReady(false);
      setChecking(false);
      setError("Token 无效或已更改");
    };
    window.addEventListener("popstate", change);
    window.addEventListener("webmux:auth", auth);
    void check();
    return () => {
      window.removeEventListener("popstate", change);
      window.removeEventListener("webmux:auth", auth);
    };
  }, []);
  async function check() {
    setChecking(true);
    try {
      await api("/health");
      setReady(true);
      setError("");
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setChecking(false);
    }
  }
  if (!ready)
    return (
      <main class="login">
        <div class="brand">
          WM<span>WebMux</span>
        </div>
        <h1>你的终端，始终在线。</h1>
        <p class="muted">在可信内网中连接这台机器。</p>
        {checking ? (
          <p>正在连接…</p>
        ) : (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              store("token", token);
              void check();
            }}
          >
            <label>
              共享 Token
              <input
                autoFocus
                type="password"
                value={token}
                onInput={(e) => setToken(e.currentTarget.value)}
                autoComplete="current-password"
                required
              />
            </label>
            {error && (
              <p role="alert" class="error">
                {error}
              </p>
            )}
            <button class="primary">连接 WebMux →</button>
          </form>
        )}
      </main>
    );
  const match = path.match(/^\/s\/([a-z2-7]{8})$/);
  return match ? (
    <TerminalPage key={match[1]} id={match[1]} />
  ) : (
    <SessionList />
  );
}
render(<App />, document.getElementById("app")!);
