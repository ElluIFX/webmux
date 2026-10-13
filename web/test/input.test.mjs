import { test } from "node:test";
import assert from "node:assert/strict";
import { modifiedInput } from "../src/input.ts";
test("sticky Ctrl and Alt yield actual terminal control bytes", () => {
  assert.equal(modifiedInput("c", { ctrl: true, alt: false }), "\x03");
  assert.equal(modifiedInput("C", { ctrl: true, alt: true }), "\x1b\x03");
  assert.equal(modifiedInput("[", { ctrl: true, alt: false }), "\x1b");
  assert.equal(modifiedInput("中文", { ctrl: true, alt: false }), "中文");
  assert.equal(modifiedInput("\x1b[A", { ctrl: true, alt: false }), "\x1b[A");
  assert.equal(modifiedInput("x", { ctrl: false, alt: true }), "\x1bx");
});
