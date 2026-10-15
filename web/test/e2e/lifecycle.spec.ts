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

test("replay and the header action return to latest output without typing", async ({
  page,
  request,
  workDir,
}, info) => {
  const marker = "scroll-latest-output";
  const id = await createShell(page, "Scroll restoration", workDir);
  await runCommand(
    page,
    "for ((i=1;i<=200;i++)); do printf 'history-%03d\\n' \"$i\"; done; printf 'scroll-latest-%s\\n' output",
  );
  await expectTerminalLine(page, marker);
  await expectTerminalLine(page, "webmux-e2e$");
  const latest = page.getByRole("button", {
    name: "滚动到最新位置",
    exact: true,
  });
  const scrollBack = async () => {
    await expect(latest).toBeEnabled();
    if (info.project.use.isMobile) {
      const bounds = (await page.locator(".xterm-screen").boundingBox())!;
      const cdp = await page.context().newCDPSession(page);
      try {
        const x = bounds.x + bounds.width / 2;
        const y = bounds.y + bounds.height / 4;
        await cdp.send("Input.dispatchTouchEvent", {
          type: "touchStart",
          touchPoints: [{ x, y, id: 1 }],
        });
        for (let step = 1; step <= 6; step++)
          await cdp.send("Input.dispatchTouchEvent", {
            type: "touchMove",
            touchPoints: [{ x, y: y + (bounds.height * step) / 12, id: 1 }],
          });
        await cdp.send("Input.dispatchTouchEvent", {
          type: "touchEnd",
          touchPoints: [],
        });
      } finally {
        await cdp.detach();
      }
    } else {
      await page.locator(".xterm-screen").hover();
      await page.mouse.wheel(0, -10000);
    }
    await expect(page.locator(".xterm-rows")).not.toContainText(marker);
  };
  await scrollBack();
  await page
    .locator(".xterm-helper-textarea")
    .evaluate((element) => element.blur());
  const before = await (await request.get(`/api/sessions/${id}/buffer`)).text();
  await latest.click();
  await expectTerminalLine(page, marker);
  await expect(page.locator(".xterm-helper-textarea")).not.toBeFocused();
  expect(await (await request.get(`/api/sessions/${id}/buffer`)).text()).toBe(
    before,
  );

  await scrollBack();
  await page.reload();
  await expect(page.locator(".restore-overlay")).toBeHidden();
  await expectTerminalLine(page, marker);

  await scrollBack();
  await page.getByRole("button", { name: "终端菜单", exact: true }).click();
  await page.getByRole("menuitem", { name: "重新同步", exact: true }).click();
  await expectTerminalLine(page, marker);

  await scrollBack();
  if (info.project.use.isMobile) {
    await page
      .locator(".terminal-header")
      .getByRole("button", { name: "代码", exact: true })
      .click();
    await expect(
      page.getByRole("region", { name: "代码工作区" }),
    ).toBeVisible();
  }
  // A controlled visibility transition exercises the real reattached WebSocket
  // and PTY snapshot; it is not a real-device suspend/resume test.
  const replay = page.waitForEvent("websocket").then((socket) =>
    socket.waitForEvent("framereceived", {
      predicate: ({ payload }) =>
        typeof payload !== "string" && payload[0] === 1,
    }),
  );
  await page.evaluate(() => {
    Object.defineProperty(document, "hidden", {
      configurable: true,
      value: true,
    });
    document.dispatchEvent(new Event("visibilitychange"));
    Reflect.deleteProperty(document, "hidden");
    document.dispatchEvent(new Event("visibilitychange"));
  });
  await replay;
  await expect(latest).toBeEnabled();
  if (info.project.use.isMobile)
    await page
      .locator(".terminal-header")
      .getByRole("button", { name: "终端", exact: true })
      .click();
  await expectTerminalLine(page, marker);

  if (info.project.use.isMobile) {
    await scrollBack();
    await page
      .locator(".terminal-header")
      .getByRole("button", { name: "代码", exact: true })
      .click();
    await latest.tap();
    await expect(page.locator(".terminal-host")).toBeVisible();
    await expectTerminalLine(page, marker);
  }
  await runCommand(page, "exit");
  await expect(page.locator(".terminal-header").getByRole("status")).toHaveText(
    "已退出",
  );
  await scrollBack();
  await page.getByRole("button", { name: "终端菜单", exact: true }).click();
  await page.getByRole("menuitem", { name: "重新同步", exact: true }).click();
  await expectTerminalLine(page, marker);
  await expect(latest).toBeEnabled();
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
