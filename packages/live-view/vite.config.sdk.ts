import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import dts from "vite-plugin-dts";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react({
      jsxRuntime: 'automatic',
    }),
    dts({
      insertTypesEntry: true,
      include: ['sdk/**/*', 'src/lib/types.ts', 'src/types.ts'],
      exclude: ['**/*.test.*', '**/*.spec.*', '**/*.css'],
      outDir: 'dist',
      rollupTypes: true,
      tsconfigPath: './tsconfig.json',
      bundledPackages: [],
      copyDtsFiles: false,
      compilerOptions: {
        declaration: true,
        declarationMap: false,
      },
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
      external: (id) => {
        // 排除所有 react 相关的包
        return /^react($|\/|$)/.test(id) || /^react-dom($|\/|$)/.test(id);
      },
      output: {
        globals: {
          react: 'React',
          'react-dom': 'ReactDOM',
          'react/jsx-runtime': 'React',
          'react-dom/client': 'ReactDOM'
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
