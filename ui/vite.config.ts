import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: { proxy: { "/api": "http://127.0.0.1:8080" } },
  build: {
    outDir: "../backend/web/dist",
    // ship empties the dir but for its .gitkeep before the build; vite's own
    // emptying would take the .gitkeep with it.
    emptyOutDir: false,
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    coverage: {
      provider: "v8",
      // lcov is what ship check reads (the UI floor and the patch gate);
      // text-summary is for humans.
      reporter: ["text-summary", "lcov"],
      reportsDirectory: "../coverage/ui",
      include: ["src/**/*.{ts,tsx}"],
      exclude: [
        "src/**/*.test.{ts,tsx}",
        "src/**/*.d.ts",
        "src/main.tsx",
        "src/test/**",
      ],
    },
  },
});
