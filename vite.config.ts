import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";
import { viteSingleFile } from "vite-plugin-singlefile";

export default defineConfig({
  base: "./",
  plugins: [vue(), viteSingleFile()],
  build: {
    outDir: "ui",
    emptyOutDir: true,
    target: "es2022",
    assetsInlineLimit: 1024 * 1024,
    chunkSizeWarningLimit: 2048,
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
    // 宿主 FS shim 偶发拦截并行 worker 的临时 SSR 文件，串行执行保证稳定。
    fileParallelism: false,
  },
});
