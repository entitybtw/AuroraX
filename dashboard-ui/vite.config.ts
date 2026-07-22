import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

const backendTarget = process.env.VITE_AURORA_BACKEND_URL ?? "http://localhost:8080";

// Vite configuration for the Aurora admin dashboard SPA.
//
// Build output is written into ../internal/admin/dashboard/dist so that
// the Go //go:embed directive in dashboard.go picks it up at compile
// time. The asset base "/admin/static/" matches the existing Go route
// GET /admin/static/*, so deep-mounted deployments (BASE_PATH=/g) work via
// the per-request base-path substitution performed by the Go handler.
export default defineConfig(({ command }) => ({
  base: command === "build" ? "/admin/static/" : "/",
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    outDir: path.resolve(__dirname, "../internal/admin/dashboard/dist"),
    emptyOutDir: true,
    sourcemap: false,
    target: "es2022",
    cssCodeSplit: true,
    chunkSizeWarningLimit: 1400,
    rollupOptions: {
      onLog(level, log, handler) {
        if (
          log.code === "MODULE_LEVEL_DIRECTIVE" ||
          log.message.includes("is dynamically imported by")
        ) {
          return;
        }
        handler(level, log);
      },
      output: {
        manualChunks: {
          vendor: [
            "@tanstack/react-query",
            "@tanstack/react-router",
            "lucide-react",
            "react",
            "react-dom",
            "recharts",
            "zod",
          ],
        },
      },
    },
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      "/admin/api": backendTarget,
      "/v1": backendTarget,
      "/p": backendTarget,
      "/health": backendTarget,
      "/metrics": backendTarget,
    },
  },
  preview: {
    port: 4173,
    strictPort: true,
  },
  test: {
    environment: "happy-dom",
    globals: true,
    css: false,
    setupFiles: ["./src/test/setup.ts"],
  },
}));
