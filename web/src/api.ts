export interface SessionInfo {
  id: string;
  title: string;
  cwd: string;
  preset: string;
  cmd: string[];
  status: "running" | "waiting" | "idle" | "exited";
  exitCode: number | null;
  lastLine: string;
  createdAt: string;
  lastActiveAt: string;
  cols: number;
  rows: number;
  clients: number;
}
export interface Preset {
  name: string;
  label: string;
  cmd: string[];
  allowArgs: boolean;
}
export class APIError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}
export const stored = (key: string, fallback = "") => {
  try {
    return localStorage.getItem("webmux." + key) ?? fallback;
  } catch {
    return fallback;
  }
};
export function store(key: string, value: string) {
  try {
    localStorage.setItem("webmux." + key, value);
  } catch {
    /* private-mode storage is optional */
  }
}
async function request(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<Response> {
  const response = await fetch("/api" + path, {
    method,
    headers: {
      Authorization: "Bearer " + stored("token"),
      "Content-Type": "application/json",
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!response.ok) {
    const detail = await response
      .json()
      .catch(() => ({ error: response.statusText }));
    if (response.status === 401) window.dispatchEvent(new Event("webmux:auth"));
    throw new APIError(response.status, detail.error);
  }
  return response;
}
export async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  const response = await request(path, method, body);
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}
export async function apiBuffer(id: string): Promise<Uint8Array> {
  return new Uint8Array(
    await (await request(`/sessions/${id}/buffer`)).arrayBuffer(),
  );
}
export function navigate(path: string) {
  history.pushState(null, "", path);
  window.dispatchEvent(new PopStateEvent("popstate"));
}
export const statusText = {
  running: "运行中",
  waiting: "可能在等你",
  idle: "空闲",
  exited: "已退出",
};
export function recentDirs(): string[] {
  try {
    const value: unknown = JSON.parse(stored("recentDirs", "[]"));
    return Array.isArray(value)
      ? value.filter((x): x is string => typeof x === "string").slice(0, 10)
      : [];
  } catch {
    return [];
  }
}
// Split extra argv without executing a shell. Quoted paths and empty arguments are preserved.
export function parseArgs(input: string): string[] {
  const result: string[] = [];
  let token = "",
    quote = "",
    escaped = false,
    started = false;
  for (const c of input) {
    if (escaped) {
      token += c;
      escaped = false;
      started = true;
    } else if (c === "\\" && quote !== "'") {
      escaped = true;
      started = true;
    } else if (quote) {
      if (c === quote) quote = "";
      else token += c;
    } else if (c === '"' || c === "'") {
      quote = c;
      started = true;
    } else if (/\s/.test(c)) {
      if (started) {
        result.push(token);
        token = "";
        started = false;
      }
    } else {
      token += c;
      started = true;
    }
  }
  if (escaped || quote) throw new Error("参数引号或反斜杠未闭合");
  if (started) result.push(token);
  return result;
}
