import { defineConfig } from "vite";
export default defineConfig({
  build: { outDir: "dist", emptyOutDir: false },
  server: {
    proxy: {
      "/api": "http://127.0.0.1:7681",
      "/ws": { target: "ws://127.0.0.1:7681", ws: true },
    },
  },
  esbuild: { jsx: "automatic", jsxImportSource: "preact" },
});
