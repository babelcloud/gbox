import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "static",
    assetsDir: "assets",
    // Generate a single HTML file with all assets inlined for embedding
    rollupOptions: {
      input: {
        main: "index.html",
      },
      output: {
        chunkFileNames: "assets/[name]-[hash].js",
        assetFileNames: "assets/[name]-[hash][extname]",
      },
    },
  },
  server: {
    port: 3000,
    proxy: {
      "/api": {
        target: "http://localhost:29888",
        changeOrigin: true,
      },
      "/ws": {
        target: "ws://localhost:29888",
        ws: true,
        changeOrigin: true,
      },
      "/stream": {
        target: "http://localhost:29888",
        changeOrigin: true,
      },
    },
  },
});
