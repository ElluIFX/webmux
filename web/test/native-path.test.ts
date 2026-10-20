import { describe, expect, it } from "vitest";
import { execFileSync } from "node:child_process";
import { join } from "node:path";
import { parseArgs } from "../src/api";
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
      for (const path of [
        "C:\\work\\a'b",
        "C:\\work\\$(Write-Output INJECTED)",
        "C:\\work\\a`;whoami",
        "C:\\work\\a & b",
      ]) {
        expect(
          execFileSync(
            powershell,
            [
              "-NoProfile",
              "-NonInteractive",
              "-Command",
              "[Console]::Write(" + quotePath(path, "powershell") + ")",
            ],
            { encoding: "utf8" },
          ),
        ).toBe(path);
      }
    },
  );
});
