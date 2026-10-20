import {
  test,
  expect,
  createShell,
  runCommand,
  expectTerminalLine,
} from "./fixtures";

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
    const uploaded = await request.post(`/api/sessions/${id}/upload`, {
      multipart: {
        file: {
          name: "native upload.txt",
          mimeType: "text/plain",
          buffer: Buffer.from("WINDOWS_UPLOAD_OK"),
        },
      },
    });
    expect(uploaded.status()).toBe(201);
    const { path } = await uploaded.json();
    expect(path).toMatch(/^[A-Za-z]:\\/);
    await runCommand(
      page,
      "Get-Content -LiteralPath '" + path.replaceAll("'", "''") + "'",
    );
    await expectTerminalLine(page, "WINDOWS_UPLOAD_OK");
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
