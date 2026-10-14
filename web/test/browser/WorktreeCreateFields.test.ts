import { afterEach, expect, test, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import { suggestWorktreeBranch } from "../../src/branchName";
import type { Repository } from "../../src/schemas";
import Harness from "./WorktreeCreateFieldsHarness.svelte";

const head = "a".repeat(40);
const repository = {
  id: "repo-1",
  name: "project",
  commonDir: "/repos/project/.git",
  anchorPath: "/repos/project",
  availability: "available",
  error: "",
  checkedAt: "2026-10-14T00:00:00Z",
  worktrees: [
    {
      id: "main",
      repositoryId: "repo-1",
      path: "/repos/project",
      kind: "main",
      branch: "main",
      head,
      detached: false,
      locked: false,
      lockReason: "",
      prunable: false,
      available: true,
      dirty: false,
      hasUntracked: false,
      hasIgnored: false,
      error: "",
    },
    {
      id: "linked",
      repositoryId: "repo-1",
      path: "/repos/project.worktrees/calm-river",
      kind: "linked",
      branch: "feature/calm-river",
      head,
      detached: false,
      locked: false,
      lockReason: "",
      prunable: false,
      available: true,
      dirty: false,
      hasUntracked: false,
      hasIgnored: false,
      error: "",
    },
  ],
} satisfies Repository;

const branches = {
  branches: [
    { name: "main", head, worktreePaths: ["/repos/project"] },
    { name: "wt/taken-branch", head, worktreePaths: [] },
    {
      name: "feature/calm-river",
      head,
      worktreePaths: ["/repos/project.worktrees/calm-river"],
    },
  ],
  bases: [{ name: "refs/heads/main", head }],
  defaultBase: head,
  defaultParent: "/repos/project.worktrees",
};

function serve(status = 200) {
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input) => {
      const url = new URL(String(input), location.href);
      if (url.pathname !== "/api/repositories/repo-1/branches") {
        throw new Error(`Unexpected request: ${url.pathname}`);
      }
      if (status !== 200) {
        return Response.json({ error: "无法读取分支" }, { status });
      }
      return Response.json(branches);
    }),
  );
}

const branchInput = () =>
  page.getByRole("textbox", { name: "新分支名称", exact: true });
const pathInput = () =>
  page.getByRole("textbox", { name: "新工作树完整路径", exact: true });
const ready = () => page.getByRole("status", { name: "表单就绪" });

afterEach(async () => {
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  }
});

test("prefills an editable suggestion and does not refill after a manual clear", async () => {
  vi.spyOn(Math, "random").mockReturnValue(0);
  serve();
  const expected = suggestWorktreeBranch(
    {
      branchNames: branches.branches.map((item) => item.name),
      worktreePaths: repository.worktrees.map((tree) => tree.path),
    },
    { random: () => 0 },
  );
  render(Harness, { repository });

  await expect.element(branchInput()).toHaveValue(expected);
  await expect
    .element(pathInput())
    .toHaveValue(`/repos/project.worktrees/${expected.split("/").at(-1)}`);
  await expect.element(ready()).toHaveTextContent("ready");
  expect(expected.split("/").at(-1)).not.toBe("calm-river");

  await branchInput().fill("feature/my-task");
  await expect.element(branchInput()).toHaveValue("feature/my-task");
  await expect
    .element(pathInput())
    .toHaveValue("/repos/project.worktrees/my-task");

  await branchInput().fill("");
  await expect.element(branchInput()).toHaveValue("");
  await expect.element(ready()).toHaveTextContent("not-ready");

  await page.getByRole("button", { name: "换一个", exact: true }).click();
  await expect.element(branchInput()).toHaveValue(expected);
  await expect.element(ready()).toHaveTextContent("ready");
  await page.getByRole("button", { name: "换一个", exact: true }).click();
  await expect.element(branchInput()).not.toHaveValue(expected);
  await expect
    .poll(() => branchInput().element().value)
    .toMatch(/^wt\/[a-z]+-[a-z]+$/);
});

test("mode switch regenerates a new branch and a failed list stays empty", async () => {
  vi.spyOn(Math, "random").mockReturnValue(0);
  serve();
  render(Harness, { repository });
  await expect
    .poll(() => branchInput().element().value)
    .toMatch(/^wt\/[a-z]+-[a-z]+$/);

  await page.getByRole("button", { name: "使用已有分支", exact: true }).click();
  await expect
    .element(page.getByRole("combobox", { name: "已有本地分支" }))
    .toHaveValue("");
  await page.getByRole("button", { name: "创建新分支", exact: true }).click();
  await expect
    .poll(() => branchInput().element().value)
    .toMatch(/^wt\/[a-z]+-[a-z]+$/);

  await cleanup();
  serve(503);
  render(Harness, { repository });
  await expect
    .element(page.getByRole("alert"))
    .toHaveTextContent("无法读取分支");
  await expect.element(branchInput()).toHaveValue("");
  await expect
    .element(page.getByRole("button", { name: "换一个", exact: true }))
    .toBeDisabled();
});
