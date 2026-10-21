import {
  test,
  expect,
  createShell,
  runCommand,
  expectTerminalLine,
} from "./fixtures";
import { join } from "node:path";

// Actual native server + PowerShell + ConPTY + browser coverage, not a cross-build.
test.skip(process.platform !== "win32", "Native Windows runtime smoke");

test("Windows PowerShell input, replay, upload, and exit", async ({
  page,
  request,
  workDir,
}, testInfo) => {
  // Capture only this isolated test's synthetic terminal traffic. This makes
  // Enter and DSR/CPR round trips observable without changing input semantics.
  const frames: {
    connection: number;
    time: number;
    direction: string;
    payload: string;
  }[] = [];
  const connections: {
    connection: number;
    time: number;
    event: string;
    error?: string;
  }[] = [];
  let nextConnection = 0;
  page.on("websocket", (socket) => {
    if (!socket.url().includes("/ws/s/")) return;
    const connection = ++nextConnection;
    connections.push({ connection, time: Date.now(), event: "open" });
    socket.on("close", () =>
      connections.push({ connection, time: Date.now(), event: "close" }),
    );
    socket.on("socketerror", (error) =>
      connections.push({ connection, time: Date.now(), event: "error", error }),
    );
    socket.on("framesent", ({ payload }) => {
      frames.push({
        connection,
        time: Date.now(),
        direction: "sent",
        payload: Buffer.from(payload).toString("base64"),
      });
    });
    socket.on("framereceived", ({ payload }) => {
      frames.push({
        connection,
        time: Date.now(),
        direction: "received",
        payload: Buffer.from(payload).toString("base64"),
      });
    });
  });
  try {
    const id = await createShell(page, "Windows native shell", workDir);
    // A WebSocket can attach before PowerShell/PSReadLine has finished startup.
    // Wait for the real prompt, not merely an open transport, before typing.
    await expect
      .poll(() =>
        page
          .locator(".xterm-rows > div")
          .evaluateAll((rows) =>
            rows
              .map((row) =>
                (row.textContent ?? "").replaceAll("\u00a0", " ").trimEnd(),
              )
              .join(""),
          ),
      )
      .toContain(`PS ${workDir}>`);
    await runCommand(page, "Write-Output ('WINDOWS_' + 'CONPTY_READY')");
    await expect
      .poll(() =>
        frames.some(
          (frame) =>
            frame.direction === "sent" &&
            Buffer.from(frame.payload, "base64").equals(Buffer.from([0, 13])),
        ),
      )
      .toBe(true);
    // Keep transport output and rendered-line assertions separate: a command
    // lost during a queue-overflow reconnect is not a renderer line-wrap bug.
    await expect
      .poll(() =>
        Buffer.concat(
          frames
            .filter((frame) => frame.direction === "received")
            .map((frame) => Buffer.from(frame.payload, "base64"))
            .filter((frame) => frame[0] === 0 || frame[0] === 1)
            .map((frame) => frame.subarray(1)),
        ).toString("utf8"),
      )
      .toContain("WINDOWS_CONPTY_READY");
    await expectTerminalLine(page, "WINDOWS_CONPTY_READY");
    await page.reload();
    await expect(
      page.locator(".terminal-header").getByRole("status"),
    ).toHaveText("已连接");
    await expectTerminalLine(page, "WINDOWS_CONPTY_READY");
    await page.locator(".xterm-helper-textarea").focus();
    await page.keyboard.type("Get-Content -LiteralPath ");
    const data = await page.evaluateHandle(() => {
      const transfer = new DataTransfer();
      transfer.items.add(
        new File(["WINDOWS_UPLOAD_OK"], "native upload.txt", {
          type: "text/plain",
        }),
      );
      return transfer;
    });
    const response = page.waitForResponse((response) =>
      response.url().endsWith(`/api/sessions/${id}/upload`),
    );
    await page
      .locator(".terminal-host")
      .dispatchEvent("drop", { dataTransfer: data });
    const uploaded = await response;
    expect(uploaded.status()).toBe(201);
    const { path } = await uploaded.json();
    expect(path).toMatch(/^[A-Za-z]:\\/);
    await expect(
      page.getByRole("progressbar", { name: "文件上传进度" }),
    ).toBeHidden();
    await expect(page.locator(".xterm-rows")).toContainText(
      "native upload.txt",
    );
    await page.locator(".xterm-helper-textarea").press("Enter");
    await expectTerminalLine(page, "WINDOWS_UPLOAD_OK");
    await data.dispose();
    await runCommand(page, "exit 7");
    await expect
      .poll(
        async () =>
          (await (await request.get(`/api/sessions/${id}`)).json()).exitCode,
      )
      .toBe(7);
  } finally {
    await testInfo.attach("native-terminal-frames.json", {
      body: JSON.stringify({ connections, frames }, null, 2),
      contentType: "application/json",
    });
  }
});

test("Windows direct launch offers creation of a missing directory", async ({
  page,
  request,
  workDir,
}) => {
  const path = join(workDir, "new directory 中文");
  await page.goto("/");
  await page
    .locator("header")
    .getByRole("button", { name: "＋ 新建会话", exact: true })
    .click();
  await page.getByLabel("工作目录", { exact: true }).fill(path);
  await page.getByRole("button", { name: "Shell", exact: true }).click();
  await page.getByRole("button", { name: "创建并进入 →", exact: true }).click();
  await expect(page.getByRole("alertdialog")).toBeVisible();
  await page
    .getByRole("button", { name: "创建目录并启动", exact: true })
    .click();
  await expect(page).toHaveURL(/\/s\/[a-z0-9]+$/);
  const sessions = await (await request.get("/api/sessions")).json();
  expect(sessions).toHaveLength(1);
  expect(sessions[0]).toMatchObject({ cwd: path, workspace: null });
});
