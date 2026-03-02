import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [],
  test: {
    fileParallelism: false,
    pool: "forks",
    testTimeout: 20000,
    retry: 0,
    setupFiles: [],
    includeTaskLocation: true,
    env: {
      SCHEDULER_URL: "http://localhost:8080",
      SCHEDULER_API_KEY: "your-secret-api-key-here",
    },
    coverage: {
      enabled: false,
    },
    cache: false,
    hideSkippedTests: false,
    allowOnly: false,
  },
});
