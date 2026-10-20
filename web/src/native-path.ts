// Paths come from the server, not the browser OS (a Windows client may use a
// Linux server and vice versa). Keep Unix filenames containing backslashes intact.
export function isWindowsPath(path: string): boolean {
  return (
    /^[A-Za-z]:[\\/]/.test(path) ||
    /^\\\\[^\\]+\\[^\\]+/.test(path) ||
    /^\/\/[^/]+\/[^/]+/.test(path)
  );
}
export function displayPath(path: string): string {
  return isWindowsPath(path) ? path.replaceAll("\\", "/") : path;
}
export function pathBasename(path: string): string {
  return displayPath(path).replace(/\/+$/, "").split("/").pop() || "/";
}
export function pathRoot(path: string): string {
  const normalized = displayPath(path);
  return (
    normalized.match(/^[A-Za-z]:\//)?.[0] ||
    normalized.match(/^\/\/[^/]+\/[^/]+\/?/)?.[0].replace(/\/?$/, "/") ||
    "/"
  );
}
export function parentPath(path: string): string {
  const normalized = displayPath(path);
  const root = pathRoot(normalized);
  const parent = normalized.replace(/\/+$/, "").replace(/\/[^/]*$/, "");
  return parent.length < root.length ? root : parent;
}
export type ShellFlavor = "posix" | "powershell" | "cmd";
export function shellFlavor(
  command: string[] | undefined,
  path: string,
): ShellFlavor {
  const name = (command?.[0] || "")
    .split(/[\\/]/)
    .pop()
    ?.toLowerCase()
    .replace(/\.exe$/, "");
  if (name === "cmd") return "cmd";
  if (name === "powershell" || name === "pwsh") return "powershell";
  if (["bash", "sh", "zsh", "fish"].includes(name || "")) return "posix";
  return isWindowsPath(path) ? "powershell" : "posix";
}
export function quotePath(path: string, shell: ShellFlavor): string {
  if (/[\x00-\x1f\x7f-\x9f\u2028\u2029]/.test(path))
    throw new Error("路径含控制字符，无法安全插入终端");
  if (shell === "powershell" && /[\u2018-\u201e]/.test(path))
    throw new Error("路径含 PowerShell 智能引号，无法安全插入终端");
  if (shell === "powershell") return "'" + path.replaceAll("'", "''") + "'";
  if (shell === "cmd") {
    // cmd expands percent variables and delayed !variables! even inside quotes.
    // Refuse ambiguous insertion rather than execute attacker-controlled text.
    if (/[%!\"]/.test(path))
      throw new Error(
        "此路径无法安全插入 cmd.exe，请使用 PowerShell 或手动复制路径",
      );
    return '"' + path + '"';
  }
  return "'" + path.replaceAll("'", "'\\''") + "'";
}
