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
      SCHEDULER_API_KEY: "abc123",
    },
    coverage: {
      enabled: false,
    },
  },
});
