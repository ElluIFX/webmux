import { randomUUID } from "node:crypto";
import {
  test,
  expect,
  createShell,
  runCommand,
  expectTerminalLine,
} from "./fixtures";

test("real shell output survives reload, then closes and is removed", async ({
  page,
  request,
  workDir,
}) => {
  const marker = `e2e-${randomUUID().slice(0, 8)}`;
  const id = await createShell(page, marker, workDir);
  const sessionURL = page.url();
  const created = await request.get(`/api/sessions/${id}`);
  expect(created.ok()).toBeTruthy();
  expect(await created.json()).toMatchObject({
    id,
    title: marker,
    cwd: workDir,
    preset: "shell",
    cmd: ["/bin/bash", "--noprofile", "--norc", "-i"],
  });

  // The whole marker never occurs in the typed command: only executing printf
  // inside the real PTY can join these halves into the asserted output line.
  await runCommand(
    page,
    `printf '\\n%s%s\\n' '${marker.slice(0, 6)}' '${marker.slice(6)}'`,
  );
  await expectTerminalLine(page, marker);
  const beforeReload = await request.get(`/api/sessions/${id}/buffer`);
  expect(await beforeReload.text()).toContain(`\r\n${marker}\r\n`);

  await page.reload();
  await expect(page).toHaveURL(sessionURL);
  await expect(page.locator(".terminal-header").getByRole("status")).toHaveText(
    "已连接",
  );
  await expect(page.locator(".restore-overlay")).toBeHidden();
  await expectTerminalLine(page, marker);
  const restored = await request.get("/api/sessions");
  expect(
    (await restored.json()).map((session: { id: string }) => session.id),
  ).toEqual([id]);

  await page.getByRole("button", { name: "终端菜单", exact: true }).click();
  await page.getByRole("menuitem", { name: "关闭会话", exact: true }).click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "确认关闭", exact: true })
    .click();
  await expect(page.locator(".terminal-header").getByRole("status")).toHaveText(
    "已退出",
  );
  await expect
    .poll(async () => {
      const response = await request.get(`/api/sessions/${id}`);
      return (await response.json()).status;
    })
    .toBe("exited");
  await expectTerminalLine(page, marker);
  const afterClose = await request.get(`/api/sessions/${id}/buffer`);
  expect(await afterClose.text()).toContain(`\r\n${marker}\r\n`);

  await page.getByRole("button", { name: "移除会话", exact: true }).click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "确认移除", exact: true })
    .click();
  await expect(page).toHaveURL(new URL("/", sessionURL).href);
  await expect(
    page.getByRole("link", { name: marker, exact: true }),
  ).toHaveCount(0);
  const removed = await request.get(`/api/sessions/${id}`);
  expect(removed.status()).toBe(404);
  expect(await (await request.get("/api/sessions")).json()).toEqual([]);
});

test("shell exit reports its real code and remains removable", async ({
  page,
  request,
  workDir,
}) => {
  const id = await createShell(page, "E2E exit code", workDir);
  await runCommand(page, "exit 7");
  await expect(
    page.getByText("进程已退出（code 7）", { exact: true }),
  ).toBeVisible();
  const exited = await request.get(`/api/sessions/${id}`);
  expect(await exited.json()).toMatchObject({ status: "exited", exitCode: 7 });
  await page.getByRole("button", { name: "移除会话", exact: true }).click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "确认移除", exact: true })
    .click();
  await expect(page).toHaveURL(/\/$/);
  expect(await (await request.get("/api/sessions")).json()).toEqual([]);
});
