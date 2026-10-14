import { describe, expect, test } from "vitest";
import { modifiedInput } from "../src/input";

describe("one-shot terminal modifiers", () => {
  test.each([
    ["c", "\x03"],
    ["C", "\x03"],
    ["@", "\x00"],
    ["[", "\x1b"],
    ["\\", "\x1c"],
    ["]", "\x1d"],
    ["^", "\x1e"],
    ["_", "\x1f"],
    ["z", "\x1a"],
  ])("Ctrl+%s produces the PTY control byte", (input, output) => {
    expect(modifiedInput(input, { ctrl: true, alt: false })).toBe(output);
  });

  test.each(["中文", "😀", "\x1b[A", "paste\ncontent", "1", ""])(
    "Ctrl preserves non-control input %j",
    (input) => {
      expect(modifiedInput(input, { ctrl: true, alt: false })).toBe(input);
    },
  );

  test("Alt prefixes Escape after Ctrl translation without modifying sticky state", () => {
    const modifiers = { ctrl: true, alt: true };
    expect(modifiedInput("C", modifiers)).toBe("\x1b\x03");
    expect(modifiers).toEqual({ ctrl: true, alt: true });
    expect(modifiedInput("x", { ctrl: false, alt: true })).toBe("\x1bx");
    expect(modifiedInput("中文", { ctrl: false, alt: true })).toBe("\x1b中文");
  });
});
