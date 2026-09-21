import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  // Inlined data: fonts would be blocked by the CSP.
  build: { assetsInlineLimit: 0 },
  server: {
    port: 5173,
    proxy: { "/api": "http://localhost:8080" },
  },
  test: { environment: "node" },
});
