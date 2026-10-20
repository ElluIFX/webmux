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
}) => {
  const id = await createShell(page, "Windows native shell", workDir);
  await runCommand(page, "Write-Output ('WINDOWS_' + 'CONPTY_READY')");
  await expectTerminalLine(page, "WINDOWS_CONPTY_READY");
  await page.reload();
  await expect(page.locator(".terminal-header").getByRole("status")).toHaveText(
    "已连接",
  );
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
});
