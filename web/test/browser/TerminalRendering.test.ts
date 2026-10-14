import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./TerminalHarness.svelte";

// Exercise the real transport framing, hook, xterm buffer and both renderers.
// This socket never opens a network connection or supplies a synthetic echo.
class FixtureSocket extends EventTarget {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  readonly url: string;
  binaryType = "blob";
  readyState = FixtureSocket.CONNECTING;
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  sent: Uint8Array[] = [];
  closeCount = 0;

  constructor(url: string | URL) {
    super();
    this.url = String(url);
    sockets.push(this);
  }

  open() {
    this.readyState = FixtureSocket.OPEN;
    this.onopen?.(new Event("open"));
  }

  receive(kind: number, text: string) {
    const payload = new TextEncoder().encode(text);
    const frame = new Uint8Array(payload.length + 1);
    frame[0] = kind;
    frame.set(payload, 1);
    this.onmessage?.(new MessageEvent("message", { data: frame.buffer }));
  }

  send(data: Uint8Array) {
    if (this.readyState !== FixtureSocket.OPEN) {
      throw new Error("Fixture socket is not open");
    }
    this.sent.push(new Uint8Array(data));
  }

  close() {
    this.closeCount++;
    this.readyState = FixtureSocket.CLOSED;
    this.onclose?.(new CloseEvent("close", { code: 1000 }));
  }
}

let sockets: FixtureSocket[];
let observations: ControlledObserver[];
let resolutionQueries: ResolutionQuery[];
let dpr: number;
let measurement: "consistent" | "css-pixels";
let glyphPixels: WeakMap<HTMLCanvasElement, number>;
let webglCanvases: WeakSet<HTMLCanvasElement>;
let resizeListeners: Set<EventListenerOrEventListenerObject>;
const nativeResizeObserver = window.ResizeObserver;
const nativeMatchMedia = window.matchMedia.bind(window);

// Chromium's device emulator can change DPR without changing the actual
// physical-pixel observations. Only that browser API boundary is controlled;
// native ResizeObserver still determines when real layout changes are delivered.
function measuredEntry(
  target: Element,
  contentRect: DOMRectReadOnly,
  pixels?: ResizeObserverSize,
): ResizeObserverEntry {
  const size = pixels ?? {
    inlineSize: Math.round(
      contentRect.width * (measurement === "consistent" ? dpr : 1),
    ),
    blockSize: Math.round(
      contentRect.height * (measurement === "consistent" ? dpr : 1),
    ),
  };
  return {
    target,
    contentRect,
    contentBoxSize: [
      { inlineSize: contentRect.width, blockSize: contentRect.height },
    ],
    borderBoxSize: [
      { inlineSize: contentRect.width, blockSize: contentRect.height },
    ],
    devicePixelContentBoxSize: [size],
  };
}

class ControlledObserver implements ResizeObserver {
  readonly targets = new Set<Element>();
  readonly native: ResizeObserver;
  readonly callback: ResizeObserverCallback;
  disconnected = false;
  observeCount = 0;
  deliveryCount = 0;

  constructor(callback: ResizeObserverCallback) {
    this.callback = callback;
    this.native = new nativeResizeObserver((entries) => {
      this.deliveryCount++;
      callback(
        entries.map((entry) => measuredEntry(entry.target, entry.contentRect)),
        this,
      );
    });
    observations.push(this);
  }

  observe(target: Element, options?: ResizeObserverOptions) {
    this.disconnected = false;
    this.observeCount++;
    this.targets.add(target);
    this.native.observe(target, options);
  }

  unobserve(target: Element) {
    this.targets.delete(target);
    this.native.unobserve(target);
  }

  disconnect() {
    this.disconnected = true;
    this.targets.clear();
    this.native.disconnect();
  }

  deliver(entry: ResizeObserverEntry) {
    // Deliberately usable after disconnect: a queued browser callback must be safe.
    this.callback([entry], this);
  }
}

class ResolutionQuery extends EventTarget {
  readonly listeners = new Set<EventListenerOrEventListenerObject>();
  readonly legacyListeners = new Set<EventListener>();
  readonly media: string;
  readonly ratio: number;
  onchange: ((event: MediaQueryListEvent) => void) | null = null;

  constructor(media: string, ratio: number) {
    super();
    this.media = media;
    this.ratio = ratio;
  }

  get matches() {
    return this.ratio === dpr;
  }

  override addEventListener(
    type: string,
    callback: EventListenerOrEventListenerObject | null,
    options?: AddEventListenerOptions | boolean,
  ) {
    if (type === "change" && callback) this.listeners.add(callback);
    super.addEventListener(type, callback, options);
  }

  override removeEventListener(
    type: string,
    callback: EventListenerOrEventListenerObject | null,
    options?: EventListenerOptions | boolean,
  ) {
    if (type === "change" && callback) this.listeners.delete(callback);
    super.removeEventListener(type, callback, options);
  }

  addListener(callback: ((event: MediaQueryListEvent) => void) | null) {
    if (!callback) return;
    this.legacyListeners.add(callback as EventListener);
    super.addEventListener("change", callback as EventListener);
  }

  removeListener(callback: ((event: MediaQueryListEvent) => void) | null) {
    if (!callback) return;
    this.legacyListeners.delete(callback as EventListener);
    super.removeEventListener("change", callback as EventListener);
  }
}

function setDpr(value: number) {
  const changed = resolutionQueries.filter(
    (query) => query.matches !== (query.ratio === value),
  );
  dpr = value;
  vi.stubGlobal("devicePixelRatio", value);
  for (const query of changed) {
    const event = new MediaQueryListEvent("change", {
      media: query.media,
      matches: query.matches,
    });
    query.dispatchEvent(event);
    query.onchange?.(event);
  }
}

// Read actual glyph pixels immediately after the real GL draw, before Chromium
// discards a non-preserved drawing buffer. Rendering/context creation are not mocked.
function observeGlyphPixels(unavailable = false) {
  const getContext = HTMLCanvasElement.prototype.getContext;
  const seen = new WeakSet<WebGL2RenderingContext>();
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockImplementation(
    function (this: HTMLCanvasElement, contextId: string, options?: unknown) {
      if (unavailable && contextId === "webgl2") return null;
      const context = getContext.call(this, contextId, options);
      if (contextId === "webgl2" && context) {
        const gl = context as WebGL2RenderingContext;
        webglCanvases.add(this);
        if (!seen.has(gl)) {
          seen.add(gl);
          const canvas = this;
          const draw = gl.drawElementsInstanced.bind(gl);
          vi.spyOn(gl, "drawElementsInstanced").mockImplementation(
            (...args) => {
              draw(...args);
              if (!canvas.isConnected || gl.isContextLost()) return;
              const width = gl.drawingBufferWidth;
              const height = Math.min(
                gl.drawingBufferHeight,
                Math.ceil(100 * dpr),
              );
              if (!width || !height) return;
              const pixels = new Uint8Array(width * height * 4);
              gl.readPixels(
                0,
                gl.drawingBufferHeight - height,
                width,
                height,
                gl.RGBA,
                gl.UNSIGNED_BYTE,
                pixels,
              );
              let red = 0;
              for (let index = 0; index < pixels.length; index += 4) {
                if (
                  pixels[index] > 180 &&
                  pixels[index + 1] < 80 &&
                  pixels[index + 2] < 80
                )
                  red++;
              }
              glyphPixels.set(canvas, red);
            },
          );
        }
      }
      return context;
    } as typeof HTMLCanvasElement.prototype.getContext,
  );
}

function host() {
  const element = document.querySelector<HTMLDivElement>(
    '[data-testid="terminal-host"]',
  );
  if (!element) throw new Error("Missing terminal host");
  return element;
}

function webglCanvas() {
  // Link overlays also use canvases; identify the one with a real GL context.
  return (
    Array.from(
      host().querySelectorAll<HTMLCanvasElement>(".xterm-screen > canvas"),
    ).find((canvas) => webglCanvases.has(canvas)) ?? null
  );
}

function hostObserver() {
  const observer = observations.find((observer) =>
    observer.targets.has(host()),
  );
  if (!observer) throw new Error("Missing terminal host observer");
  return observer;
}

function domText() {
  return host().querySelector(".xterm-rows")?.textContent ?? "";
}

async function expectDomText(text: string) {
  await expect.poll(domText).toContain(text);
  const rows = host().querySelector<HTMLElement>(".xterm-rows")!;
  await expect.element(page.elementLocator(rows)).toBeVisible();
  expect(rows.getBoundingClientRect().width).toBeGreaterThan(0);
  expect(rows.getBoundingClientRect().height).toBeGreaterThan(0);
  expect(webglCanvas()).toBeNull();
}

async function expectGlyphs() {
  await expect
    .poll(() => {
      const canvas = webglCanvas();
      return canvas ? (glyphPixels.get(canvas) ?? 0) : 0;
    })
    .toBeGreaterThan(100);
  const canvas = webglCanvas()!;
  await expect.element(page.elementLocator(canvas)).toBeVisible();
  const bounds = canvas.getBoundingClientRect();
  expect(Math.abs(canvas.width - bounds.width * dpr)).toBeLessThanOrEqual(1);
  expect(Math.abs(canvas.height - bounds.height * dpr)).toBeLessThanOrEqual(1);
  return canvas;
}

const prompt = "fixture@mobile:~/project$ omp";
const snapshot = `\x1b[38;2;255;0;0m${prompt}\x1b[0m\r\n`;

async function openTerminal() {
  const rendered = await render(Harness);
  expect(sockets).toHaveLength(1);
  const socket = sockets[0];
  socket.open();
  socket.receive(1, snapshot);
  await expect
    .element(page.getByRole("status", { name: "Terminal connection" }))
    .toHaveTextContent("connected");
  await expect
    .element(page.getByRole("status", { name: "Terminal restoration" }))
    .toHaveTextContent("ready");
  return { socket, rendered };
}

function expectNoSessionMutation(socket: FixtureSocket) {
  expect(sockets).toEqual([socket]);
  expect(socket.closeCount).toBe(0);
  // Resize frames are legitimate; input and resync frames are not.
  expect(socket.sent.length).toBeGreaterThan(0);
  expect(socket.sent.every((frame) => frame[0] === 1)).toBe(true);
}

beforeEach(async () => {
  sockets = [];
  observations = [];
  resolutionQueries = [];
  resizeListeners = new Set();
  glyphPixels = new WeakMap();
  webglCanvases = new WeakSet();
  dpr = 1;
  measurement = "consistent";
  await page.viewport(412, 924);
  const addWindowListener = window.addEventListener.bind(window);
  vi.spyOn(window, "addEventListener").mockImplementation(
    (type, listener, options) => {
      if (type === "resize" && listener) resizeListeners.add(listener);
      addWindowListener(type, listener, options);
    },
  );
  vi.stubGlobal("WebSocket", FixtureSocket);
  vi.stubGlobal("ResizeObserver", ControlledObserver);
  vi.stubGlobal("devicePixelRatio", dpr);
  vi.stubGlobal("matchMedia", (query: string) => {
    const resolution = /\(resolution:\s*([\d.]+)dppx\)/.exec(query);
    if (!resolution) return nativeMatchMedia(query);
    const result = new ResolutionQuery(query, Number(resolution[1]));
    resolutionQueries.push(result);
    return result as unknown as MediaQueryList;
  });
});

afterEach(async () => {
  try {
    await cleanup();
    await userEvent.cleanup();
    expect(
      observations
        .filter((observer) => observer.targets.size > 0)
        .map((observer) =>
          [...observer.targets].map((target) => target.className),
        ),
    ).toEqual([]);
    // Assert the hook's modern listeners. xterm 5.5's legacy ScreenDprMonitor
    // is not registered for disposal; isolate that dependency in teardown.
    expect(
      resolutionQueries
        .filter((query) => query.listeners.size > 0)
        .map((query) => ({
          media: query.media,
          listeners: query.listeners.size,
        })),
    ).toEqual([]);
    expect(sockets.map((socket) => socket.closeCount)).toEqual(
      sockets.map(() => 1),
    );
  } finally {
    for (const observer of observations) observer.native.disconnect();
    for (const query of resolutionQueries) {
      for (const listener of [...query.listeners])
        query.removeEventListener("change", listener);
      for (const listener of [...query.legacyListeners])
        query.removeListener(listener);
    }
    for (const listener of resizeListeners)
      window.removeEventListener("resize", listener);
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  }
});

test("mobile high-DPR CSS-pixel observations render the restored prompt and retain working input", async () => {
  setDpr(2.625);
  measurement = "css-pixels";
  observeGlyphPixels();
  const { socket } = await openTerminal();
  await expectDomText(prompt);
  expectNoSessionMutation(socket);

  await page.getByRole("button", { name: "Focus terminal input" }).click();
  const textarea = host().querySelector<HTMLTextAreaElement>(
    ".xterm-helper-textarea",
  )!;
  expect(document.activeElement).toBe(textarea);
  expect(textarea.disabled).toBe(false);
  await userEvent.keyboard("x");
  await expect
    .poll(() => socket.sent.filter((frame) => frame[0] === 0).length)
    .toBe(1);
  const input = socket.sent.find((frame) => frame[0] === 0)!;
  expect(new TextDecoder().decode(input.subarray(1))).toBe("x");
  expect(sockets).toEqual([socket]);
  expect(socket.closeCount).toBe(0);
});

test("desktop to incompatible high-DPR fallback preserves rendered history and the same socket", async () => {
  observeGlyphPixels();
  const { socket, rendered } = await openTerminal();
  const canvas = await expectGlyphs();
  const snapshotPixels = glyphPixels.get(canvas)!;
  socket.receive(0, "output before DPR transition\r\n");
  socket.receive(0, "\x1b[38;2;255;0;0mvisible history\x1b[0m\r\n");
  // Additional red glyph pixels prove new output was actually drawn, not just queued.
  await expect
    .poll(() => glyphPixels.get(canvas) ?? 0)
    .toBeGreaterThan(snapshotPixels + 100);

  const observer = hostObserver();
  const observeCount = observer.observeCount;
  measurement = "css-pixels";
  setDpr(2.625);
  expect(observer.observeCount).toBeGreaterThan(observeCount);
  await expectDomText(prompt);
  await expectDomText("output before DPR transition");
  await expectDomText("visible history");
  socket.receive(0, "output after DPR transition\r\n");
  await expectDomText("output after DPR transition");
  expectNoSessionMutation(socket);

  const oldHost = host();
  const lateEntry = measuredEntry(oldHost, oldHost.getBoundingClientRect());
  const oldQueries = [...resolutionQueries];
  await rendered.unmount();
  expect(observer.disconnected).toBe(true);
  expect(socket.closeCount).toBe(1);
  const sentCount = socket.sent.length;
  const unmountedObserveCount = observer.observeCount;
  expect(() => observer.deliver(lateEntry)).not.toThrow();
  for (const query of oldQueries) query.dispatchEvent(new Event("change"));
  expect(socket.sent).toHaveLength(sentCount);
  expect(sockets).toEqual([socket]);
  expect(observer.observeCount).toBe(unmountedObserveCount);
  expect(oldQueries.every((query) => query.listeners.size === 0)).toBe(true);
});

test("fractional DPR, one-pixel rounding, hidden boxes and absent physical measurements keep real WebGL glyphs", async () => {
  setDpr(1.25);
  observeGlyphPixels();
  const { socket } = await openTerminal();
  const canvas = await expectGlyphs();
  const observer = hostObserver();
  const bounds = host().getBoundingClientRect();
  observer.deliver(
    measuredEntry(host(), bounds, {
      inlineSize: bounds.width * dpr + 1,
      blockSize: Math.round(bounds.height * dpr),
    }),
  );
  observer.deliver(
    measuredEntry(host(), new DOMRect(0, 0, 0, 0), {
      inlineSize: 0,
      blockSize: 0,
    }),
  );
  const { devicePixelContentBoxSize: _physical, ...unsupported } =
    measuredEntry(host(), bounds);
  observer.deliver(unsupported as ResizeObserverEntry);
  glyphPixels.delete(canvas);
  socket.receive(
    0,
    "\x1b[38;2;255;0;0mstill rendered at fractional DPR\x1b[0m\r\n",
  );
  expect(await expectGlyphs()).toBe(canvas);

  const deliveries = observer.deliveryCount;
  setDpr(2.625);
  await expect.poll(() => observer.deliveryCount).toBeGreaterThan(deliveries);
  glyphPixels.delete(canvas);
  socket.receive(
    0,
    "\x1b[38;2;255;0;0mconsistent fresh physical pixels\x1b[0m\r\n",
  );
  expect(await expectGlyphs()).toBe(canvas);
  expectNoSessionMutation(socket);
});

test("unavailable WebGL renders snapshot and later output visibly with the DOM renderer", async () => {
  setDpr(2.625);
  observeGlyphPixels(true);
  const { socket } = await openTerminal();
  await expectDomText(prompt);
  socket.receive(0, "output without WebGL\r\n");
  await expectDomText("output without WebGL");
  expectNoSessionMutation(socket);
});

test("real WebGL context loss falls back visibly without discarding existing history", async () => {
  observeGlyphPixels();
  const { socket } = await openTerminal();
  const canvas = await expectGlyphs();
  const gl = canvas.getContext("webgl2")!;
  const extension = gl.getExtension("WEBGL_lose_context");
  expect(extension).not.toBeNull();
  extension!.loseContext();
  // addon-webgl allows three seconds for context restoration before fallback.
  await expect.poll(domText, { timeout: 10000 }).toContain(prompt);
  await expectDomText(prompt);
  socket.receive(0, "output after real context loss\r\n");
  await expectDomText("output after real context loss");
  expectNoSessionMutation(socket);
}, 15000);
