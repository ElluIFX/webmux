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
    const terminalElement = term.element!;
    const screen = terminalElement.querySelector<HTMLElement>(".xterm-screen")!;
    let webgl: WebglAddon | undefined;
    const useDomRenderer = () => {
      if (!webgl) return;
      const addon = webgl;
      webgl = undefined;
      addon.dispose();
      term.refresh(0, term.rows - 1);
    };
    try {
      webgl = new WebglAddon();
      term.loadAddon(webgl);
      webgl.onContextLoss(useDomRenderer);
    } catch {
      useDomRenderer();
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
    const observer = new ResizeObserver(([entry]) => {
      if (disposed) return;
      if (webgl) {
        const pixels = entry?.devicePixelContentBoxSize?.[0];
        const css = entry?.contentRect;
        const dpr = window.devicePixelRatio || 1;
        // Chromium emulation can report physical pixels at the host DPR while
        // xterm renders at the emulated DPR. The WebGL addon then shrinks its
        // backing canvas without updating its render dimensions. DOM rendering
        // avoids that incompatible measurement; allow normal pixel rounding.
        if (
          pixels &&
          css.width > 0 &&
          css.height > 0 &&
          pixels.inlineSize > 0 &&
          pixels.blockSize > 0 &&
          (Math.abs(pixels.inlineSize - css.width * dpr) > 1 ||
            Math.abs(pixels.blockSize - css.height * dpr) > 1)
        ) {
          useDomRenderer();
        }
      }
      clearTimeout(timer);
      timer = window.setTimeout(() => fitAddon.fit(), 100);
    });
    const observeSize = () => {
      try {
        observer.observe(parent, { box: "device-pixel-content-box" });
      } catch {
        observer.observe(parent);
      }
    };
    let resolution = matchMedia(
      `(resolution: ${window.devicePixelRatio || 1}dppx)`,
    );
    const resolutionChanged = () => {
      if (disposed) return;
      resolution.removeEventListener("change", resolutionChanged);
      resolution = matchMedia(
        `(resolution: ${window.devicePixelRatio || 1}dppx)`,
      );
      resolution.addEventListener("change", resolutionChanged);
      // A DPR change need not change the CSS box. Request a fresh observation
      // rather than comparing the new DPR against stale physical dimensions.
      observer.unobserve(parent);
      observeSize();
    };
    resolution.addEventListener("change", resolutionChanged);
    observeSize();
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

    type TouchPoint = { x: number; y: number };
    const pointers = new Map<number, TouchPoint>();
    let firstPoint: TouchPoint | undefined;
    let secondPoint: TouchPoint | undefined;
    let touchX = 0;
    let touchY = 0;
    let wheelY = 0;
    let remainder = 0;
    let moved = false;
    let pinchDistance = 0;
    let pinchFont = fontSize;
    const distance = () =>
      Math.hypot(
        firstPoint!.x - secondPoint!.x,
        firstPoint!.y - secondPoint!.y,
      );
    const rebasePointers = () => {
      const points = pointers.values();
      firstPoint = points.next().value;
      secondPoint = points.next().value;
      touchX = firstPoint?.x ?? 0;
      touchY = firstPoint?.y ?? 0;
      wheelY = touchY;
      remainder = 0;
      pinchDistance = pointers.size === 2 ? distance() : 0;
      pinchFont = fontSize;
    };
    const clearPointers = () => {
      for (const id of pointers.keys()) {
        pointers.delete(id);
        if (parent.hasPointerCapture(id)) parent.releasePointerCapture(id);
      }
      firstPoint = undefined;
      secondPoint = undefined;
      remainder = 0;
      pinchDistance = 0;
    };
    const pointerDown = (event: PointerEvent) => {
      if (disposed || event.pointerType !== "touch") return;
      event.stopPropagation();
      if (!pointers.size) moved = false;
      pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
      if (pointers.size > 1) moved = true;
      // DOM terminal rows are replaced while scrolling or repainting a TUI.
      // Capture to the stable host, not the row originally touched.
      parent.setPointerCapture(event.pointerId);
      rebasePointers();
    };
    const pointerMove = (event: PointerEvent) => {
      if (disposed || event.pointerType !== "touch") return;
      const point = pointers.get(event.pointerId);
      if (!point) return;
      event.preventDefault();
      event.stopPropagation();
      if (point.x === event.clientX && point.y === event.clientY) return;
      point.x = event.clientX;
      point.y = event.clientY;
      moved = true;
      if (pointers.size === 2 && pinchDistance > 0) {
        setFontSize((pinchFont * distance()) / pinchDistance);
      } else if (pointers.size === 1) {
        const rowHeight = screen.getBoundingClientRect().height / term.rows;
        if (rowHeight <= 0) return;
        remainder += (touchY - point.y) / rowHeight;
        const lines = Math.trunc(remainder);
        // Let xterm choose normal history, mouse reports or alternate-buffer
        // cursor keys, exactly as for a desktop wheel.
        for (let count = Math.abs(lines); count > 0; count--) {
          terminalElement.dispatchEvent(
            new WheelEvent("wheel", {
              bubbles: true,
              cancelable: true,
              deltaMode: WheelEvent.DOM_DELTA_LINE,
              deltaY: lines > 0 ? 1 : -1,
              clientX: touchX,
              clientY: wheelY,
            }),
          );
        }
        remainder -= lines;
        touchY = point.y;
      }
    };
    const pointerUp = (event: PointerEvent) => {
      if (disposed || !pointers.has(event.pointerId)) return;
      event.stopPropagation();
      if (moved) event.preventDefault();
      pointers.delete(event.pointerId);
      if (parent.hasPointerCapture(event.pointerId))
        parent.releasePointerCapture(event.pointerId);
      if (pointers.size) rebasePointers();
      else {
        clearPointers();
        if (!moved) term.focus();
      }
    };
    const pointerCancel = (event: PointerEvent) => {
      if (disposed || !pointers.has(event.pointerId)) return;
      event.stopPropagation();
      moved = true;
      clearPointers();
    };
    const blockNativeTouch = (event: TouchEvent) => {
      event.stopPropagation();
      if (event.type === "touchmove" || (event.type === "touchend" && moved))
        event.preventDefault();
    };
    parent.addEventListener("pointerdown", pointerDown, true);
    parent.addEventListener("pointermove", pointerMove, true);
    parent.addEventListener("pointerup", pointerUp, true);
    parent.addEventListener("pointercancel", pointerCancel, true);
    parent.addEventListener("lostpointercapture", pointerCancel, true);
    parent.addEventListener("touchstart", blockNativeTouch, {
      capture: true,
      passive: true,
    });
    parent.addEventListener("touchmove", blockNativeTouch, {
      capture: true,
      passive: false,
    });
    parent.addEventListener("touchend", blockNativeTouch, {
      capture: true,
      passive: false,
    });
    parent.addEventListener("touchcancel", blockNativeTouch, {
      capture: true,
      passive: true,
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
      resolution.removeEventListener("change", resolutionChanged);
      document.removeEventListener("visibilitychange", visible);
      clearPointers();
      parent.removeEventListener("pointerdown", pointerDown, true);
      parent.removeEventListener("pointermove", pointerMove, true);
      parent.removeEventListener("pointerup", pointerUp, true);
      parent.removeEventListener("pointercancel", pointerCancel, true);
      parent.removeEventListener("lostpointercapture", pointerCancel, true);
      parent.removeEventListener("touchstart", blockNativeTouch, true);
      parent.removeEventListener("touchmove", blockNativeTouch, true);
      parent.removeEventListener("touchend", blockNativeTouch, true);
      parent.removeEventListener("touchcancel", blockNativeTouch, true);
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
