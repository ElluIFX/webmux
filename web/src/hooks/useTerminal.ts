import { useEffect, useRef, useState } from "preact/hooks";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import { apiBuffer, stored, store, type SessionInfo } from "../api";
import { connect, type Connection } from "../ws";
import "@xterm/xterm/css/xterm.css";
import { modifiedInput, type Modifiers } from "../input";
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
  const modifierRef = useRef<Modifiers>({ ctrl: false, alt: false });
  const [modifiers, setModifiers] = useState<Modifiers>(modifierRef.current);
  const [error, setError] = useState("");
  function resetModifiers() {
    modifierRef.current = { ctrl: false, alt: false };
    setModifiers(modifierRef.current);
  }
  function send(input: string) {
    const value = modifiedInput(input, modifierRef.current);
    resetModifiers();
    return socket.current?.send(0, encoder.encode(value));
  }
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
    let stale = false,
      exited = false,
      connected = false;
    const restore = (payload: Uint8Array) => {
      setRestoring(true);
      term.reset();
      term.write(payload, () => {
        if (!disposed) {
          term.scrollToBottom();
          setRestoring(false);
        }
      });
    };
    const ws = connect(
      "/ws/s/" + id,
      (data) => {
        if (typeof data === "string") return;
        const frame = new Uint8Array(data),
          payload = frame.subarray(1);
        if (frame[0] === 0 || frame[0] === 1) {
          if (document.hidden) {
            stale = true;
          } else if (frame[0] === 1) {
            stale = false;
            restore(payload);
          } else {
            term.write(payload);
          }
        } else if (frame[0] === 2) {
          const value = JSON.parse(
            new TextDecoder().decode(payload),
          ) as SessionInfo;
          setInfo(value);
          document.title = value.title + " · WebMux";
        } else if (frame[0] === 3) {
          exited = true;
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
        connected = state === "connected";
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
    const input = term.onData(send);
    const resize = term.onResize((size) =>
      ws.send(1, encoder.encode(JSON.stringify(size))),
    );
    let timer: number | undefined;
    const observer = new ResizeObserver(() => {
      clearTimeout(timer);
      timer = window.setTimeout(() => fitAddon.fit(), 100);
    });
    observer.observe(parent);
    const visible = () => {
      if (document.hidden || !stale) return;
      if (exited) {
        void apiBuffer(id)
          .then((payload) => {
            if (!disposed) {
              stale = false;
              restore(payload);
            }
          })
          .catch((err) => setError((err as Error).message));
      } else if (connected) {
        ws.send(2, new Uint8Array());
      }
    };
    document.addEventListener("visibilitychange", visible);
    let touchY = 0,
      remainder = 0,
      moved = false,
      pinchDistance = 0,
      pinchFont = fontSize;
    const distance = (touches: TouchList) =>
      Math.hypot(
        touches[0].clientX - touches[1].clientX,
        touches[0].clientY - touches[1].clientY,
      );
    const touchStart = (event: TouchEvent) => {
      event.stopPropagation();
      moved = false;
      remainder = 0;
      touchY = event.touches[0]?.clientY ?? 0;
      if (event.touches.length === 2) {
        pinchDistance = distance(event.touches);
        pinchFont = term.options.fontSize || 14;
      }
    };
    const touchMove = (event: TouchEvent) => {
      event.preventDefault();
      event.stopPropagation();
      moved = true;
      if (event.touches.length === 2 && pinchDistance > 0) {
        setFontSize(
          Math.max(
            12,
            Math.min(
              20,
              Math.round((pinchFont * distance(event.touches)) / pinchDistance),
            ),
          ),
        );
      } else if (event.touches.length === 1) {
        const nextY = event.touches[0].clientY;
        const rowHeight = parent.clientHeight / term.rows;
        remainder += (touchY - nextY) / rowHeight;
        const lines = Math.trunc(remainder);
        if (lines) term.scrollLines(lines);
        remainder -= lines;
        touchY = nextY;
      }
    };
    const touchEnd = (event: TouchEvent) => {
      event.stopPropagation();
      if (moved) event.preventDefault();
      else term.focus();
      if (!event.touches.length) {
        pinchDistance = 0;
        fitAddon.fit();
      }
    };
    parent.addEventListener("touchstart", touchStart, {
      capture: true,
      passive: true,
    });
    parent.addEventListener("touchmove", touchMove, {
      capture: true,
      passive: false,
    });
    parent.addEventListener("touchend", touchEnd, {
      capture: true,
      passive: false,
    });
    term.attachCustomKeyEventHandler((event) => {
      if (
        event.type === "keydown" &&
        event.ctrlKey &&
        event.shiftKey &&
        (event.key === "C" || event.key === "c")
      ) {
        void navigator.clipboard
          ?.writeText(term.getSelection())
          .catch((err) => setError((err as Error).message));
        return false;
      }
      if (
        event.type === "keydown" &&
        event.ctrlKey &&
        event.shiftKey &&
        event.key.toLowerCase() === "v"
      ) {
        void navigator.clipboard
          ?.readText()
          .then((text) => term.paste(text))
          .catch((err) => setError((err as Error).message));
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
      document.removeEventListener("visibilitychange", visible);
      parent.removeEventListener("touchstart", touchStart, true);
      parent.removeEventListener("touchmove", touchMove, true);
      parent.removeEventListener("touchend", touchEnd, true);
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
    modifiers,
    error,
    send,
    toggleModifier: (key: keyof Modifiers) => {
      modifierRef.current = {
        ...modifierRef.current,
        [key]: !modifierRef.current[key],
      };
      setModifiers(modifierRef.current);
    },
    paste: (text: string) => {
      resetModifiers();
      terminal.current?.paste(text);
    },
    keyboard: () => {
      const term = terminal.current;
      if (document.activeElement === term?.textarea) term?.blur();
      else term?.focus();
    },
    clear: () => terminal.current?.clear(),
    resync: () => socket.current?.send(2, new Uint8Array()),
    focus: () => terminal.current?.focus(),
  };
}
