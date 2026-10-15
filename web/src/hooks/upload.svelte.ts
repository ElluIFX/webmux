import { onMount } from "svelte";
import { uploadFile } from "../api";
import type { Connection } from "../ws";

// The terminal state and input the upload flow waits on and types into.
interface UploadTerminal {
  readonly exitCode: number | null;
  readonly connection: Connection;
  readonly restoring: boolean;
  paste(text: string): boolean;
  focus(): void;
}

// Uploads dropped, pasted or picked files to the session's temporary directory
// and types each quoted path into the terminal once it is live again.
export function useUpload(
  getHost: () => HTMLElement,
  id: string,
  terminal: UploadTerminal,
  setError: (message: string) => void,
) {
  let progress = $state<{
    name: string;
    percent: number;
    index: number;
    total: number;
    waiting: boolean;
  } | null>(null);
  let dragging = $state(false);
  let foreground = $state(!document.hidden);
  let controller: AbortController | undefined;
  let waiter = $state<{
    resolve: () => void;
    reject: (error: Error) => void;
  } | null>(null);
  let mounted = false;

  $effect(() => {
    const current = waiter;
    if (!current) return;
    if (
      terminal.exitCode !== null ||
      terminal.connection === "exited" ||
      terminal.connection === "missing"
    ) {
      waiter = null;
      current.reject(new Error("终端会话已退出或不存在"));
    } else if (
      foreground &&
      terminal.connection === "connected" &&
      !terminal.restoring
    ) {
      waiter = null;
      current.resolve();
    }
  });

  async function waitForTerminal(signal: AbortSignal) {
    if (signal.aborted) throw new DOMException("操作已取消", "AbortError");
    const { promise, resolve, reject } = Promise.withResolvers<void>();
    const abort = () => reject(new DOMException("操作已取消", "AbortError"));
    const timeout = window.setTimeout(
      () => reject(new Error("等待终端恢复超时")),
      30_000,
    );
    signal.addEventListener("abort", abort, { once: true });
    waiter = { resolve, reject };
    try {
      await promise;
    } finally {
      clearTimeout(timeout);
      signal.removeEventListener("abort", abort);
      waiter = null;
    }
  }

  async function uploadFiles(files: File[]) {
    if (!files.length) return;
    if (progress) {
      setError("正在上传，请等待当前上传完成。");
      return;
    }
    if (
      terminal.exitCode !== null ||
      terminal.connection === "exited" ||
      terminal.connection === "missing"
    ) {
      setError("终端会话已退出或不存在，无法上传文件。");
      return;
    }
    setError("");
    const current = new AbortController();
    controller = current;
    try {
      for (const [index, file] of files.entries()) {
        progress = {
          name: file.name,
          percent: 0,
          index: index + 1,
          total: files.length,
          waiting: false,
        };
        const { path } = await uploadFile(
          id,
          file,
          (percent) => {
            if (progress) progress.percent = percent;
          },
          current.signal,
        );
        if (!mounted || current.signal.aborted) return;
        progress.waiting = true;
        progress.percent = 100;
        try {
          await waitForTerminal(current.signal);
          // Quote arbitrary filenames safely. Uploading never runs a command.
          if (!terminal.paste("'" + path.replaceAll("'", "'\\''") + "' "))
            throw new Error("终端未就绪");
        } catch (err) {
          throw new Error(
            `文件已上传至 ${path}，但路径未发送：${(err as Error).message}`,
          );
        }
      }
      terminal.focus();
    } catch (err) {
      if (mounted) setError((err as Error).message);
    } finally {
      controller = undefined;
      if (mounted) progress = null;
    }
  }

  // Returns whether the paste carried files and was taken over as an upload.
  function filePaste(event: ClipboardEvent) {
    const files = Array.from(event.clipboardData?.files ?? []);
    if (!files.length) return false;
    event.preventDefault();
    event.stopImmediatePropagation();
    void uploadFiles(files);
    return true;
  }

  onMount(() => {
    mounted = true;
    // bind:this is cleared before onMount teardown; retain the listener owner.
    const host = getHost();
    let foregroundFrame = 0;
    const visibilityChanged = () => {
      cancelAnimationFrame(foregroundFrame);
      foreground = false;
      // Let foreground reconnect handlers invalidate the old socket first.
      if (!document.hidden)
        foregroundFrame = requestAnimationFrame(() => {
          foreground = !document.hidden;
        });
    };
    document.addEventListener("visibilitychange", visibilityChanged);
    // Capture before xterm consumes paste/drop events; text still uses xterm.
    let dragDepth = 0;
    const dragEnter = (event: DragEvent) => {
      if (!event.dataTransfer?.types.includes("Files")) return;
      event.preventDefault();
      dragging = true;
      dragDepth++;
    };
    const dragOver = (event: DragEvent) => {
      if (!event.dataTransfer?.types.includes("Files")) return;
      event.preventDefault();
      event.dataTransfer.dropEffect = progress ? "none" : "copy";
    };
    const dragLeave = () => {
      if (--dragDepth <= 0) {
        dragDepth = 0;
        dragging = false;
      }
    };
    const drop = (event: DragEvent) => {
      if (!event.dataTransfer?.types.includes("Files")) return;
      event.preventDefault();
      event.stopImmediatePropagation();
      dragDepth = 0;
      dragging = false;
      void uploadFiles(Array.from(event.dataTransfer.files));
    };
    host.addEventListener("paste", filePaste, true);
    host.addEventListener("dragenter", dragEnter, true);
    host.addEventListener("dragover", dragOver, true);
    host.addEventListener("dragleave", dragLeave, true);
    host.addEventListener("drop", drop, true);
    return () => {
      document.removeEventListener("visibilitychange", visibilityChanged);
      cancelAnimationFrame(foregroundFrame);
      mounted = false;
      controller?.abort();
      host.removeEventListener("paste", filePaste, true);
      host.removeEventListener("dragenter", dragEnter, true);
      host.removeEventListener("dragover", dragOver, true);
      host.removeEventListener("dragleave", dragLeave, true);
      host.removeEventListener("drop", drop, true);
    };
  });

  return {
    get progress() {
      return progress;
    },
    get dragging() {
      return dragging;
    },
    uploadFiles,
    filePaste,
    cancel() {
      controller?.abort();
    },
  };
}
