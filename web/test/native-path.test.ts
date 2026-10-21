import { describe, expect, it } from "vitest";
import { execFileSync } from "node:child_process";
import { join } from "node:path";
import { parseArgs } from "../src/api";
import { UploadSchema } from "../src/schemas";
import {
  displayPath,
  isWindowsPath,
  parentPath,
  pathBasename,
  pathRoot,
  quotePath,
  shellFlavor,
} from "../src/native-path";

describe("server-native paths", () => {
  it("handles drive and UNC roots independently of the browser OS", () => {
    expect(displayPath("C:\\Users\\me\\work")).toBe("C:/Users/me/work");
    expect(pathBasename("C:\\Users\\my work")).toBe("my work");
    expect(parentPath("C:/Users/")).toBe("C:/");
    expect(parentPath("C:/")).toBe("C:/");
    expect(pathRoot("\\\\server\\share\\dir")).toBe("//server/share/");
    expect(parentPath("//server/share/")).toBe("//server/share/");
    expect(displayPath("/unix/back\\slash")).toBe("/unix/back\\slash");
    expect(isWindowsPath("/unix/path")).toBe(false);
    expect(parentPath("\\\\?\\C:\\Users\\")).toBe("C:/");
    expect(pathRoot("\\\\?\\UNC\\server\\share\\dir")).toBe("//server/share/");
  });
  it("accepts native absolute upload paths and rejects relative paths", () => {
    for (const path of [
      "C:\\Temp\\file.txt",
      "D:/file.txt",
      "\\\\server\\share\\file.txt",
      "/tmp/file.txt",
      "\\\\?\\C:\\Temp\\file.txt",
    ])
      expect(UploadSchema.parse({ path }).path).toBe(path);
    for (const path of [
      "",
      "relative.txt",
      "C:relative.txt",
      "\\rooted.txt",
      "~\\file.txt",
      "C:\\bad\x00path",
    ])
      expect(UploadSchema.safeParse({ path }).success).toBe(false);
  });
  it("preserves native backslashes in Windows argv without invoking a shell", () => {
    expect(
      parseArgs('"C:\\Program Files\\tool.exe" C:\\work\\file.txt ""', true),
    ).toEqual(["C:\\Program Files\\tool.exe", "C:\\work\\file.txt", ""]);
    expect(parseArgs("'C:\\work\\'", true)).toEqual(["C:\\work\\"]);
  });
  it("quotes for the session shell and rejects cmd expansion hazards", () => {
    expect(shellFlavor(["cmd.exe"], "C:\\work")).toBe("cmd");
    expect(
      shellFlavor(["C:\\Program Files\\Git\\bin\\bash.exe"], "C:\\work"),
    ).toBe("posix");
    expect(quotePath("C:\\a'b $(whoami)", "powershell")).toBe(
      "'C:\\a''b $(whoami)'",
    );
    expect(quotePath("C:\\a&b", "cmd")).toBe('"C:\\a&b"');
    for (const path of ["C:\\%PATH%", "C:\\!name!", 'C:\\a"b'])
      expect(() => quotePath(path, "cmd")).toThrow();
    expect(() => quotePath("C:\\x\rwhoami", "powershell")).toThrow();
    expect(() =>
      quotePath("C:\\x’; Write-Output PWN; ‘", "powershell"),
    ).toThrow();
    for (const quote of ["\u2018", "\u2019", "\u201a", "\u201b"])
      expect(() =>
        quotePath("C:\\x" + quote + "; Write-Output PWN", "powershell"),
      ).toThrow();
  });
  it.skipIf(process.platform !== "win32")(
    "passes PowerShell metacharacters literally on Windows",
    () => {
      const powershell = join(
        process.env.SystemRoot || "C:\\Windows",
        "System32",
        "WindowsPowerShell",
        "v1.0",
        "powershell.exe",
      );
      const paths = [
        "C:\\work\\a'b",
        "C:\\work\\$(Write-Output INJECTED)",
        "C:\\work\\a`;whoami",
        "C:\\work\\a & b",
      ];
      expect(
        JSON.parse(
          execFileSync(
            powershell,
            [
              "-NoProfile",
              "-NonInteractive",
              "-Command",
              "ConvertTo-Json -Compress @(" +
                paths.map((path) => quotePath(path, "powershell")).join(",") +
                ")",
            ],
            { encoding: "utf8", timeout: 20_000 },
          ),
        ),
      ).toEqual(paths);
    },
    30_000,
  );
});
