import { test, expect, createShell } from "./fixtures";

test("mobile soft keys stay in one row with paste visible and all keys reachable", async ({
  page,
  workDir,
}, testInfo) => {
  test.skip(
    !testInfo.project.use.isMobile,
    "Soft keys are shown on touch devices",
  );
  await page.addInitScript(() => {
    Object.defineProperty(navigator.clipboard, "readText", {
      value: () =>
        Promise.reject(
          new DOMException("Permission denied", "NotAllowedError"),
        ),
    });
  });
  await createShell(page, "Single-row soft keys", workDir);
  const bar = page.getByRole("complementary", { name: "终端软键" });
  const paste = bar.getByRole("button", { name: "粘贴", exact: true });
  const keyboard = bar.getByRole("button", { name: "唤起或收起键盘" });
  for (const width of [320, 390, 844]) {
    await page.setViewportSize({ width, height: 844 });
    await expect(paste).toBeInViewport({ ratio: 1 });
    await expect(keyboard).toBeInViewport({ ratio: 1 });
    const layout = await bar.evaluate((element) => {
      const buttons = [...element.querySelectorAll("button")].map((button) =>
        button.getBoundingClientRect(),
      );
      return {
        height: element.getBoundingClientRect().height,
        verticalSpread:
          Math.max(...buttons.map((bounds) => bounds.top)) -
          Math.min(...buttons.map((bounds) => bounds.top)),
        pageOverflow: document.documentElement.scrollWidth - innerWidth,
      };
    });
    expect(
      layout.verticalSpread,
      "All soft keys share one row",
    ).toBeLessThanOrEqual(1);
    expect(
      layout.height,
      "The toolbar must not consume two button rows",
    ).toBeLessThan(64);
    expect(layout.pageOverflow).toBe(0);
  }
  await page.setViewportSize({ width: 320, height: 844 });
  await bar
    .getByRole("button", { name: "Ctrl+R", exact: true })
    .scrollIntoViewIfNeeded();
  // Scroll offsets snap to device pixels (DPR 2.625 here), so the last key can
  // stay clipped by a fraction of a CSS pixel at maximum scroll.
  await expect(
    bar.getByRole("button", { name: "Ctrl+R", exact: true }),
  ).toBeInViewport({ ratio: 0.99 });
  await paste.scrollIntoViewIfNeeded();
  await expect(paste).toBeInViewport({ ratio: 1 });
  await keyboard.click();
  await expect(page.locator(".xterm-helper-textarea")).toBeFocused();
  await paste.click();
  await expect(page.getByRole("dialog", { name: "粘贴到终端" })).toBeVisible();
});
