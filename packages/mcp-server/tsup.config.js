/** @type {import('tsup').Options} */
export default {
    entry: ['src/index.ts'],
    format: ['esm'],
    target: 'node18',
    sourcemap: true,
    clean: true,
    minify: false,
    dts: true,
    splitting: false,
    keepNames: true,
    external: ['node:*'],
    noExternal: [/(.*)/],
    shims: true,
    banner: {
        js: `import { createRequire } from 'module'; const require = createRequire(import.meta.url);`,
    },
    outDir: 'dist',
} 