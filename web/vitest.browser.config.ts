import { mergeConfig } from "vite";
import { defineConfig } from "vitest/config";
import { playwright } from "@vitest/browser-playwright";
import viteConfig from "./vite.config";

// Reuse the production compiler, Tailwind and aliases, but never its API proxy.
const { server: _applicationServer, ...sharedConfig } = viteConfig;

export default mergeConfig(
  sharedConfig,
  defineConfig({
    optimizeDeps: {
      include: ["@xterm/xterm", "@xterm/addon-fit", "@xterm/addon-webgl"],
    },
    test: {
      include: ["test/browser/**/*.test.ts"],
      setupFiles: ["./test/browser/setup.ts"],
      browser: {
        enabled: true,
        headless: true,
        provider: playwright(),
        instances: [{ browser: "chromium" }],
        viewport: { width: 1024, height: 900 },
      },
    },
  }),
);
