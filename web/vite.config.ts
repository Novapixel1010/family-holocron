/// <reference types="vitest" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const pluginsDir = new URL("../plugins", import.meta.url).pathname;

// The production build is written into the Go module so it can be embedded with go:embed.
// Plugin UIs live outside web/ (plugins/<name>/web); "@plugins" points there, and shared
// libraries are deduped so plugin code uses this app's single copy of React etc.
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": "/src", "@plugins": pluginsDir },
    dedupe: ["react", "react-dom", "react-router-dom", "@tanstack/react-query", "@xyflow/react"],
  },
  build: {
    outDir: "../server/frontend/dist",
    emptyOutDir: true,
    chunkSizeWarningLimit: 900,
  },
  server: {
    port: 5173,
    fs: { allow: [".", pluginsDir] },
    proxy: {
      "/api": "http://localhost:8080",
      "/healthz": "http://localhost:8080",
    },
  },
  test: { environment: "node", include: ["src/**/*.test.ts", "../plugins/*/web/**/*.test.ts"] },
});
