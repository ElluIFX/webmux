import { onMount } from "svelte";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import { apiBuffer, stored, store } from "../api";
import {
  ExitSchema,
  FontSizeSchema,
  SessionInfoSchema,
  type SessionInfo,
} from "../schemas";
import { connect, type Connection, type SocketConnection } from "../ws";
import { modifiedInput, type Modifiers } from "../input";
import "@xterm/xterm/css/xterm.css";

// Read the same theme tokens used by the shadcn surfaces, including live theme changes.
function terminalTheme() {
  const css = getComputedStyle(document.documentElement);
  return {
    background: css.getPropertyValue("--terminal-background").trim(),
    foreground: css.getPropertyValue("--terminal-foreground").trim(),
    cursor: css.getPropertyValue("--terminal-blue").trim(),
    selectionBackground: css.getPropertyValue("--terminal-selection").trim(),
    black: css.getPropertyValue("--terminal-black").trim(),
    red: css.getPropertyValue("--terminal-red").trim(),
    green: css.getPropertyValue("--terminal-green").trim(),
    yellow: css.getPropertyValue("--terminal-yellow").trim(),
    blue: css.getPropertyValue("--terminal-blue").trim(),
    magenta: css.getPropertyValue("--terminal-magenta").trim(),
    cyan: css.getPropertyValue("--terminal-cyan").trim(),
    white: css.getPropertyValue("--terminal-white").trim(),
  };
}

export function useTerminal(
  getContainer: () => HTMLDivElement,
  id: string,
  clipboard: { paste: () => void; copySelection: (text: string) => void },
) {
  let terminal: Terminal | undefined;
  let socket: SocketConnection | undefined;
  let fit: FitAddon | undefined;
  let info = $state<SessionInfo>();
  let connection = $state<Connection>("connecting");
  let restoring = $state(true);
  let exitCode = $state<number | null>(null);
  const defaultFont = matchMedia("(pointer: coarse)").matches ? 13 : 14;
  let fontSize = $state(
    FontSizeSchema.catch(defaultFont).parse(
      stored("fontSize", String(defaultFont)),
    ),
  );
  let modifiers = $state<Modifiers>({ ctrl: false, alt: false });
  let error = $state("");
  const encoder = new TextEncoder();
  const decoder = new TextDecoder();

  function send(input: string) {
    const value = modifiedInput(input, modifiers);
    modifiers = { ctrl: false, alt: false };
    return socket?.send(0, encoder.encode(value));
  }

  function setFontSize(value: number) {
    fontSize = Math.max(12, Math.min(20, Math.round(value)));
    if (terminal) {
      terminal.options.fontSize = fontSize;
      fit?.fit();
    }
    store("fontSize", String(fontSize));
  }

  onMount(() => {
    const parent = getContainer();
    const term = new Terminal({
      fontSize,
      fontFamily: '"SF Mono", Menlo, Consolas, monospace',
      scrollback: 10000,
      cursorBlink: true,
      allowProposedApi: true,
      macOptionIsMeta: true,
      theme: terminalTheme(),
    });
    terminal = term;
    const fitAddon = new FitAddon();
    fit = fitAddon;
    term.loadAddon(fitAddon);
    term.open(parent);
    let webgl: WebglAddon | undefined;
    try {
      webgl = new WebglAddon();
      term.loadAddon(webgl);
      webgl.onContextLoss(() => webgl?.dispose());
    } catch {
      webgl?.dispose();
      // xterm's built-in DOM renderer remains available without WebGL.
    }
    const colors = matchMedia("(prefers-color-scheme: light)");
    const theme = () => {
      term.options.theme = terminalTheme();
    };
    colors.addEventListener("change", theme);
    term.textarea?.setAttribute("autocapitalize", "off");
    term.textarea?.setAttribute("autocorrect", "off");
    term.textarea?.setAttribute("spellcheck", "false");
    let disposed = false;
    let stale = false;
    let exited = false;
    const restore = (payload: Uint8Array) => {
      restoring = true;
      term.reset();
      term.write(payload, () => {
        if (!disposed) {
          term.scrollToBottom();
          restoring = false;
        }
      });
    };
    const ws = connect(
      "/ws/s/" + id,
      (data) => {
        if (typeof data === "string") return;
        const frame = new Uint8Array(data);
        const payload = frame.subarray(1);
        if (frame[0] === 0 || frame[0] === 1) {
          if (document.hidden) stale = true;
          else if (frame[0] === 1) {
            stale = false;
            restore(payload);
          } else term.write(payload);
        } else if (frame[0] === 2) {
          info = SessionInfoSchema.parse(JSON.parse(decoder.decode(payload)));
          document.title = info.title + " · WebMux";
        } else if (frame[0] === 3) {
          exited = true;
          exitCode = ExitSchema.parse(
            JSON.parse(decoder.decode(payload)),
          ).exitCode;
        }
      },
      (state) => {
        connection = state;
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
          restoring = true;
        if (state === "missing") restoring = false;
      },
      (message) => {
        error = message;
      },
    );
    socket = ws;
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
      // Live sessions reattach in connect(); exited sessions cannot reopen that socket.
      if (!document.hidden && stale && exited) {
        void apiBuffer(id)
          .then((payload) => {
            if (!disposed) {
              stale = false;
              restore(payload);
            }
          })
          .catch((err) => {
            if (!disposed) error = (err as Error).message;
          });
      }
    };
    document.addEventListener("visibilitychange", visible);

    let touchY = 0;
    let remainder = 0;
    let moved = false;
    let pinchDistance = 0;
    let pinchFont = fontSize;
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
        setFontSize((pinchFont * distance(event.touches)) / pinchDistance);
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
      if (event.type === "keydown" && event.ctrlKey && event.shiftKey) {
        if (event.key.toLowerCase() === "c") {
          clipboard.copySelection(term.getSelection());
          return false;
        }
        if (event.key.toLowerCase() === "v") {
          clipboard.paste();
          return false;
        }
      }
      return true;
    });
    fitAddon.fit();
    if (matchMedia("(pointer: fine)").matches) term.focus();
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
      terminal = undefined;
      socket = undefined;
      fit = undefined;
    };
  });

  return {
    get info() {
      return info;
    },
    get connection() {
      return connection;
    },
    get restoring() {
      return restoring;
    },
    get exitCode() {
      return exitCode;
    },
    get fontSize() {
      return fontSize;
    },
    get modifiers() {
      return modifiers;
    },
    get error() {
      return error;
    },
    send,
    setFontSize,
    toggleModifier(key: keyof Modifiers) {
      modifiers = { ...modifiers, [key]: !modifiers[key] };
    },
    paste(text: string) {
      modifiers = { ctrl: false, alt: false };
      terminal?.paste(text);
    },
    keyboard() {
      if (document.activeElement === terminal?.textarea) terminal?.blur();
      else terminal?.focus();
    },
    clear() {
      terminal?.clear();
    },
    resync() {
      if (exitCode !== null) {
        void apiBuffer(id)
          .then((payload) => {
            terminal?.reset();
            terminal?.write(payload, () => terminal?.scrollToBottom());
          })
          .catch((err) => {
            error = (err as Error).message;
          });
      } else socket?.send(2, new Uint8Array());
    },
    focus() {
      terminal?.focus();
    },
  };
}
