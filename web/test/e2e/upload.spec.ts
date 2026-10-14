import { readFile } from "node:fs/promises";
import { test, expect, createShell, expectTerminalLine } from "./fixtures";

test("file chooser shows progress, uploads to temp, and pastes a shell-safe path without Enter", async ({
  page,
  workDir,
}) => {
  const id = await createShell(page, "File upload", workDir);
  const contents = Buffer.from("UPLOADED_FILE_CONTENT\n");
  const { promise, resolve } = Promise.withResolvers<void>();
  await page.route(`**/api/sessions/${id}/upload`, async (route) => {
    await promise;
    await route.continue();
  });
  await page.locator(".xterm-helper-textarea").focus();
  await page.keyboard.type("cat ");
  const response = page.waitForResponse((response) =>
    response.url().endsWith(`/api/sessions/${id}/upload`),
  );
  const chooser = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "上传文件", exact: true }).click();
  await (
    await chooser
  ).setFiles({
    name: "notes 'quoted'.txt",
    mimeType: "text/plain",
    buffer: contents,
  });
  try {
    await expect(
      page.getByRole("progressbar", { name: "文件上传进度" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "上传文件", exact: true }),
    ).toBeDisabled();
  } finally {
    resolve();
  }
  const uploaded = await response;
  expect(uploaded.status()).toBe(201);
  const { path } = await uploaded.json();
  expect(path).toContain("webmux-upload-");
  expect(path.startsWith(workDir)).toBe(false);
  expect(await readFile(path)).toEqual(contents);
  await expect(
    page.getByRole("progressbar", { name: "文件上传进度" }),
  ).toBeHidden();
  await expect(page.locator(".xterm-rows")).not.toContainText(
    "UPLOADED_FILE_CONTENT",
  );
  await page.locator(".xterm-helper-textarea").press("Enter");
  await expectTerminalLine(page, "UPLOADED_FILE_CONTENT");
});

test("dragging a file into the terminal uploads actual bytes and pastes the path", async ({
  page,
  workDir,
}) => {
  const id = await createShell(page, "Drop upload", workDir);
  await page.locator(".xterm-helper-textarea").focus();
  await page.keyboard.type("cat ");
  const data = await page.evaluateHandle(() => {
    const transfer = new DataTransfer();
    transfer.items.add(
      new File(["DROPPED_FILE_CONTENT\n"], "dragged.txt", {
        type: "text/plain",
      }),
    );
    return transfer;
  });
  const host = page.locator(".terminal-host");
  await host.dispatchEvent("dragenter", { dataTransfer: data });
  await expect(
    page.getByText("松开以上传文件到临时目录", { exact: true }),
  ).toBeVisible();
  await host.dispatchEvent("dragover", { dataTransfer: data });
  const response = page.waitForResponse((response) =>
    response.url().endsWith(`/api/sessions/${id}/upload`),
  );
  await host.dispatchEvent("drop", { dataTransfer: data });
  const uploaded = await response;
  expect(uploaded.status()).toBe(201);
  const { path } = await uploaded.json();
  expect(await readFile(path, "utf8")).toBe("DROPPED_FILE_CONTENT\n");
  await expect(
    page.getByText("松开以上传文件到临时目录", { exact: true }),
  ).toBeHidden();
  await expect(
    page.getByRole("progressbar", { name: "文件上传进度" }),
  ).toBeHidden();
  await expect(page.locator(".xterm-rows")).toContainText("dragged.txt");
  await page.locator(".xterm-helper-textarea").press("Enter");
  await expectTerminalLine(page, "DROPPED_FILE_CONTENT");
  await data.dispose();
});

test("desktop clipboard image uploads while ordinary text paste remains terminal input", async ({
  page,
  context,
  workDir,
}, testInfo) => {
  test.skip(
    testInfo.project.name !== "chromium-desktop",
    "Desktop clipboard integration",
  );
  const id = await createShell(page, "Clipboard image", workDir);
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  const clipboardBytes = await page.evaluate(async () => {
    const canvas = new OffscreenCanvas(2, 2);
    const context = canvas.getContext("2d")!;
    context.fillStyle = "#00aa66";
    context.fillRect(0, 0, 2, 2);
    const image = await canvas.convertToBlob({ type: "image/png" });
    await navigator.clipboard.write([
      new ClipboardItem({ "image/png": image }),
    ]);
    const [item] = await navigator.clipboard.read();
    return Array.from(
      new Uint8Array(await (await item.getType("image/png")).arrayBuffer()),
    );
  });
  await page.locator(".xterm-helper-textarea").focus();
  const response = page.waitForResponse((response) =>
    response.url().endsWith(`/api/sessions/${id}/upload`),
  );
  await page.keyboard.press("Control+Shift+V");
  const uploaded = await response;
  expect(uploaded.status()).toBe(201);
  const { path } = await uploaded.json();
  expect(path).toMatch(/\.png$/);
  expect(await readFile(path)).toEqual(Buffer.from(clipboardBytes));
  await expect(page.locator(".xterm-rows")).toContainText("clipboard-");
  await expect(
    page.getByRole("progressbar", { name: "文件上传进度" }),
  ).toBeHidden();
  await page.keyboard.press("Control+U");
  await page.evaluate(() =>
    navigator.clipboard.writeText("printf 'NORMAL_PASTE_CONTENT\\n'"),
  );
  await page.keyboard.press(
    process.platform === "darwin" ? "Meta+V" : "Control+V",
  );
  await page.waitForFunction(() =>
    document
      .querySelector(".xterm-rows")
      ?.textContent?.includes("NORMAL_PASTE_CONTENT"),
  );
  await page.keyboard.press("Enter");
  await expectTerminalLine(page, "NORMAL_PASTE_CONTENT");
});

test("desktop native file paste takes precedence over the clipboard's text", async ({
  page,
  workDir,
}, testInfo) => {
  test.skip(
    testInfo.project.name !== "chromium-desktop",
    "Desktop file clipboard event",
  );
  const id = await createShell(page, "Clipboard file", workDir);
  const response = page.waitForResponse((response) =>
    response.url().endsWith(`/api/sessions/${id}/upload`),
  );
  await page.locator(".xterm-helper-textarea").evaluate((input) => {
    const transfer = new DataTransfer();
    transfer.items.add(
      new File(["CLIPBOARD_FILE_CONTENT"], "clipboard-file.txt", {
        type: "text/plain",
      }),
    );
    transfer.setData("text/plain", "DO_NOT_PASTE_FILENAME_TEXT");
    input.dispatchEvent(
      new ClipboardEvent("paste", {
        bubbles: true,
        cancelable: true,
        clipboardData: transfer,
      }),
    );
  });
  const uploaded = await response;
  expect(uploaded.status()).toBe(201);
  const { path } = await uploaded.json();
  expect(await readFile(path, "utf8")).toBe("CLIPBOARD_FILE_CONTENT");
  await expect(page.locator(".xterm-rows")).toContainText("clipboard-file.txt");
  await expect(page.locator(".xterm-rows")).not.toContainText(
    "DO_NOT_PASTE_FILENAME_TEXT",
  );
});
