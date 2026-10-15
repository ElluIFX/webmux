import { expect, test } from "vitest";
import { createTerminalDrafts } from "../../src/terminal-draft.svelte";

test("draft edits preserve Unicode, selections and session isolation across mode switches", () => {
  const drafts = createTerminalDrafts();
  const a = drafts.get("session-a");
  const b = drafts.get("session-b");
  drafts.setEnabled(a, true);
  drafts.edit(a, "中文🙂 replace 尾");
  drafts.select(a, 5, 12);
  expect(drafts.insert(a, "引用")).toBe("draft");
  expect(a.text).toBe("中文🙂 引用 尾");
  expect(a.selectionStart).toBe(7);
  expect(a.selectionEnd).toBe(7);
  drafts.setEnabled(a, false);
  drafts.setEnabled(b, true);
  drafts.edit(b, "another session");
  drafts.setEnabled(a, true);
  expect(drafts.get("session-a").text).toBe("中文🙂 引用 尾");
  expect(b.text).toBe("another session");
  drafts.clear(a);
  expect(a.text).toBe("");
  expect(a.enabled).toBe(true);
  expect(b.text).toBe("another session");
});

test("async input cannot downgrade after draft mode was enabled and closed", () => {
  const drafts = createTerminalDrafts();
  const entry = drafts.get("session-a");
  const directOperation = drafts.begin(entry);
  expect(drafts.destination(entry, directOperation)).toBe("terminal");
  drafts.setEnabled(entry, true);
  const draftOperation = drafts.begin(entry);
  drafts.setEnabled(entry, false);
  expect(drafts.destination(entry, directOperation)).toBe("draft");
  expect(drafts.destination(entry, draftOperation)).toBe("draft");
  drafts.insert(entry, "uploaded-path ", directOperation);
  drafts.insert(entry, "clipboard", draftOperation);
  expect(entry.text).toBe("uploaded-path clipboard");
  expect(entry.enabled).toBe(false);
  expect(entry.updatedWhileHidden).toBe(true);
  // A new action after mode is closed is direct; the prior operations stay pinned.
  expect(drafts.destination(entry, drafts.begin(entry))).toBe("terminal");
  drafts.setEnabled(entry, true);
  expect(entry.updatedWhileHidden).toBe(false);
  expect(entry.text).toBe("uploaded-path clipboard");
});

test("late operations cannot resurrect removed drafts or target another session", () => {
  const drafts = createTerminalDrafts();
  const old = drafts.get("session-a");
  drafts.setEnabled(old, true);
  const pending = drafts.begin(old);
  drafts.remove("session-a");
  const fresh = drafts.get("session-a");
  const other = drafts.get("session-b");
  drafts.setEnabled(fresh, true);
  drafts.setEnabled(other, true);
  expect(drafts.insert(old, "late", pending)).toBe("failed");
  expect(drafts.insert(fresh, "late", pending)).toBe("failed");
  expect(drafts.insert(other, "wrong-session", pending)).toBe("failed");
  expect(old.text).toBe("");
  expect(fresh.text).toBe("");
  expect(other.text).toBe("");
});

test("authoritative reconciliation removes only absent sessions and authentication reset invalidates pending input", () => {
  const drafts = createTerminalDrafts();
  const retained = drafts.get("retained");
  const removed = drafts.get("removed");
  drafts.setEnabled(retained, true);
  drafts.edit(retained, "still useful after exit");
  const pending = drafts.begin(retained);
  drafts.reconcile(["retained"]);
  expect(drafts.valid(retained)).toBe(true);
  expect(drafts.valid(removed)).toBe(false);
  expect(retained.text).toBe("still useful after exit");
  drafts.clearAll();
  expect(drafts.insert(retained, "late", pending)).toBe("failed");
  const newAuthentication = drafts.get("retained");
  expect(newAuthentication.text).toBe("");
  expect(drafts.destination(newAuthentication, pending)).toBe("failed");
});

test("external insert appends without an editing position and clamps stale positions", () => {
  const drafts = createTerminalDrafts();
  const entry = drafts.get("a");
  drafts.setEnabled(entry, true);
  drafts.edit(entry, "prefix ");
  drafts.insert(entry, "file:12 ");
  expect(entry.text).toBe("prefix file:12 ");
  drafts.edit(entry, "x", 100, 200);
  drafts.insert(entry, "🙂");
  expect(entry.text).toBe("x🙂");
  expect(entry.selectionStart).toBe(3);
});
