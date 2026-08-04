import { fileURLToPath, URL } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  build: { outDir: "dist", emptyOutDir: true, chunkSizeWarningLimit: Number.POSITIVE_INFINITY },
  server: { port: 34115 },
  test: { setupFiles: ["./src/test-setup.ts"] },
});
