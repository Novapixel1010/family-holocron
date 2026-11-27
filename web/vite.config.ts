/// <reference types="vitest" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The production build is written into the Go module so it can be embedded with go:embed.
export default defineConfig({
  plugins: [react()],
  resolve: { alias: { "@": "/src" } },
  build: {
    outDir: "../server/frontend/dist",
    emptyOutDir: true,
    chunkSizeWarningLimit: 900,
  },
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:8080",
      "/healthz": "http://localhost:8080",
    },
  },
  test: { environment: "node", include: ["src/**/*.test.ts"] },
});
