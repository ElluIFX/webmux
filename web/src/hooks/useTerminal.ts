import { useEffect, useRef, useState } from "preact/hooks";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import { stored, store, type SessionInfo } from "../api";
import { connect, type Connection } from "../ws";
import "@xterm/xterm/css/xterm.css";
const dark = {
  background: "#0d1117",
  foreground: "#c9d1d9",
  cursor: "#58a6ff",
  selectionBackground: "#264f78",
  black: "#161b22",
  red: "#ff7b72",
  green: "#7ee787",
  yellow: "#d29922",
  blue: "#58a6ff",
  magenta: "#bc8cff",
  cyan: "#76e3ea",
  white: "#c9d1d9",
};
const light = {
  background: "#ffffff",
  foreground: "#24292f",
  cursor: "#0969da",
  selectionBackground: "#b6d7ff",
  black: "#24292f",
  red: "#cf222e",
  green: "#116329",
  yellow: "#9a6700",
  blue: "#0969da",
  magenta: "#8250df",
  cyan: "#1b7c83",
  white: "#57606a",
};
export function useTerminal(id: string) {
  const container = useRef<HTMLDivElement>(null),
    terminal = useRef<Terminal>(),
    socket = useRef<ReturnType<typeof connect>>(),
    fit = useRef<FitAddon>();
  const [info, setInfo] = useState<SessionInfo>(),
    [connection, setConnection] = useState<Connection>("connecting"),
    [restoring, setRestoring] = useState(true),
    [exitCode, setExitCode] = useState<number | null>(null),
    [fontSize, setFontSize] = useState(
      Math.max(
        12,
        Math.min(
          20,
          Number(
            stored(
              "fontSize",
              matchMedia("(pointer: coarse)").matches ? "13" : "14",
            ),
          ) || 14,
        ),
      ),
    );
  const encoder = new TextEncoder();
  useEffect(() => {
    const parent = container.current;
    if (!parent) return;
    const colors = matchMedia("(prefers-color-scheme: light)");
    const term = new Terminal({
      fontSize,
      fontFamily: '"SF Mono", Menlo, Consolas, monospace',
      scrollback: 10000,
      cursorBlink: true,
      allowProposedApi: true,
      macOptionIsMeta: true,
      theme: colors.matches ? light : dark,
    });
    terminal.current = term;
    const fitAddon = new FitAddon();
    fit.current = fitAddon;
    term.loadAddon(fitAddon);
    term.open(parent);
    let webgl: WebglAddon | undefined;
    try {
      webgl = new WebglAddon();
      term.loadAddon(webgl);
      webgl.onContextLoss(() => webgl?.dispose());
    } catch {
      webgl?.dispose();
      // xterm 5's built-in DOM renderer is the dependency-free fallback.
    }
    const theme = () => {
      term.options.theme = colors.matches ? light : dark;
    };
    colors.addEventListener("change", theme);
    term.textarea?.setAttribute("autocapitalize", "off");
    term.textarea?.setAttribute("autocorrect", "off");
    term.textarea?.setAttribute("spellcheck", "false");
    let disposed = false;
    const ws = connect(
      "/ws/s/" + id,
      (data) => {
        if (typeof data === "string") return;
        const frame = new Uint8Array(data),
          payload = frame.subarray(1);
        if (frame[0] === 1) {
          setRestoring(true);
          term.reset();
          term.write(payload, () => {
            if (!disposed) {
              term.scrollToBottom();
              setRestoring(false);
            }
          });
        } else if (frame[0] === 0) term.write(payload);
        else if (frame[0] === 2) {
          const value = JSON.parse(
            new TextDecoder().decode(payload),
          ) as SessionInfo;
          setInfo(value);
          document.title = value.title + " · WebMux";
        } else if (frame[0] === 3) {
          setExitCode(
            (
              JSON.parse(new TextDecoder().decode(payload)) as {
                exitCode: number;
              }
            ).exitCode,
          );
        }
      },
      (state) => {
        setConnection(state);
        if (state === "connected") {
          fitAddon.fit();
          ws.send(
            1,
            encoder.encode(
              JSON.stringify({ cols: term.cols, rows: term.rows }),
            ),
          );
        }
        if (state === "connecting" || state === "reconnecting")
          setRestoring(true);
      },
    );
    socket.current = ws;
    const input = term.onData((s) => ws.send(0, encoder.encode(s)));
    const resize = term.onResize((size) =>
      ws.send(1, encoder.encode(JSON.stringify(size))),
    );
    let timer: number | undefined;
    const observer = new ResizeObserver(() => {
      clearTimeout(timer);
      timer = window.setTimeout(() => fitAddon.fit(), 100);
    });
    observer.observe(parent);
    term.attachCustomKeyEventHandler((event) => {
      if (
        event.type === "keydown" &&
        event.ctrlKey &&
        event.shiftKey &&
        (event.key === "C" || event.key === "c")
      ) {
        void navigator.clipboard?.writeText(term.getSelection());
        return false;
      }
      return true;
    });
    fitAddon.fit();
    term.focus();
    return () => {
      disposed = true;
      clearTimeout(timer);
      observer.disconnect();
      input.dispose();
      resize.dispose();
      colors.removeEventListener("change", theme);
      ws.close();
      term.dispose();
      terminal.current = undefined;
      socket.current = undefined;
    };
  }, [id]);
  useEffect(() => {
    if (terminal.current) {
      terminal.current.options.fontSize = fontSize;
      fit.current?.fit();
    }
    store("fontSize", String(fontSize));
  }, [fontSize]);
  return {
    container,
    terminal,
    info,
    connection,
    restoring,
    exitCode,
    fontSize,
    setFontSize,
    send: (s: string) => socket.current?.send(0, encoder.encode(s)),
    clear: () => terminal.current?.clear(),
    resync: () => socket.current?.send(2, new Uint8Array()),
    focus: () => terminal.current?.focus(),
  };
}
