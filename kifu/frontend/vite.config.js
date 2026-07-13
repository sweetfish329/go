import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import path from "path";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [svelte()],
  resolve: {
    alias: {
      "@wasm-kifu": path.resolve(__dirname, "../wasm-kifu"),
    },
  },
  build: {
    chunkSizeWarningLimit: 600,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes("node_modules")) {
            if (id.includes("qrcode")) {
              return "qrcode";
            }
            if (
              id.includes("@sabaki/sgf") ||
              id.includes("safer-buffer") ||
              id.includes("iconv-lite")
            ) {
              return "sgf-parser";
            }
            return "vendor";
          }
        },
      },
    },
  },
  server: {
    port: 5173,
    host: "0.0.0.0",
    fs: {
      allow: [".", "../wasm-kifu"],
    },
    proxy: {
      "/api": {
        target: "http://backend:8080",
        changeOrigin: true,
      },
    },
  },
});
