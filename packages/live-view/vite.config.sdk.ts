import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import dts from "vite-plugin-dts";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react(),
    dts({
      insertTypesEntry: true,
      include: ['sdk/**/*'],
      exclude: ['**/*.test.*', '**/*.spec.*']
    })
  ],
  build: {
    outDir: "dist",
    lib: {
      entry: "sdk/index.ts",
      name: "gbox-live-view",
      fileName: (format) => `index.${format}.js`,
      formats: ['es', 'umd']
    },
    rollupOptions: {
      // 不要把 peerDependencies 打包进去
      external: ['react', 'react-dom'],
      output: {
        globals: {
          react: 'React',
          'react-dom': 'ReactDOM'
        }
      }
    },
    cssCodeSplit: false
    // sourcemap: true
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
