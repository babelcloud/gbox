import js from '@eslint/js';
import typescript from '@typescript-eslint/eslint-plugin';
import typescriptParser from '@typescript-eslint/parser';

export default [
  js.configs.recommended,
  {
    files: ['**/*.{js,jsx,ts,tsx}'],
    ignores: ['**/*.d.ts', '**/node_modules/**', '**/dist/**', '**/vendor/**'],
    languageOptions: {
      parser: typescriptParser,
      parserOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module',
        ecmaFeatures: {
          jsx: true,
        },
      },
      globals: {
        // Browser globals
        window: 'readonly',
        document: 'readonly',
        navigator: 'readonly',
        console: 'readonly',
        setTimeout: 'readonly',
        clearTimeout: 'readonly',
        setInterval: 'readonly',
        clearInterval: 'readonly',
        requestAnimationFrame: 'readonly',
        cancelAnimationFrame: 'readonly',
        fetch: 'readonly',
        URL: 'readonly',
        performance: 'readonly',
        Buffer: 'readonly',
        global: 'readonly',

        // DOM types
        HTMLElement: 'readonly',
        HTMLDivElement: 'readonly',
        HTMLVideoElement: 'readonly',
        HTMLCanvasElement: 'readonly',
        HTMLAudioElement: 'readonly',
        CanvasRenderingContext2D: 'readonly',
        DOMRect: 'readonly',
        DOMMatrix: 'readonly',
        AbortController: 'readonly',
        AbortSignal: 'readonly',
        DOMException: 'readonly',

        // Web APIs
        WebSocket: 'readonly',
        RTCPeerConnection: 'readonly',
        RTCDataChannel: 'readonly',
        RTCIceCandidate: 'readonly',
        RTCOfferOptions: 'readonly',
        RTCSessionDescription: 'readonly',
        MediaStream: 'readonly',
        MediaStreamTrack: 'readonly',
        ResizeObserver: 'readonly',
        IntersectionObserver: 'readonly',
        VideoDecoder: 'readonly',
        EncodedVideoChunk: 'readonly',
        VideoFrame: 'readonly',
        VideoDecoderConfig: 'readonly',
        HardwareAcceleration: 'readonly',
        MediaSource: 'readonly',
        SourceBuffer: 'readonly',
        ReadableStreamDefaultReader: 'readonly',
        BufferSource: 'readonly',
        TextEncoder: 'readonly',
        URLSearchParams: 'readonly',

        // Event types
        Event: 'readonly',
        MouseEvent: 'readonly',
        TouchEvent: 'readonly',
        WheelEvent: 'readonly',
        KeyboardEvent: 'readonly',
        CloseEvent: 'readonly',
        MessageEvent: 'readonly',

        // React
        React: 'readonly',

        // Jest globals
        jest: 'readonly',
        describe: 'readonly',
        it: 'readonly',
        test: 'readonly',
        expect: 'readonly',
        beforeEach: 'readonly',
        afterEach: 'readonly',
        beforeAll: 'readonly',
        afterAll: 'readonly',

        // Node.js globals for CommonJS
        module: 'readonly',
        require: 'readonly',
        exports: 'readonly',
        __dirname: 'readonly',
        __filename: 'readonly',
        process: 'readonly',
      },
    },
    plugins: {
      '@typescript-eslint': typescript,
    },
    rules: {
      // TypeScript rules
      '@typescript-eslint/no-unused-vars': ['error', {
        argsIgnorePattern: '^_',
        varsIgnorePattern: '^_',
        caughtErrorsIgnorePattern: '^_'
      }],
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/explicit-function-return-type': 'off',
      '@typescript-eslint/explicit-module-boundary-types': 'off',
      '@typescript-eslint/no-non-null-assertion': 'warn',

      // Basic rules
      'no-console': 'off', // Allow console for debugging
      'no-debugger': 'error',
      'no-unused-vars': 'off', // Using TypeScript version instead
      'prefer-const': 'error',
      'no-var': 'error',
      'no-undef': 'error',
      'no-prototype-builtins': 'error',
    },
  },
  {
    files: ['**/*.test.{js,jsx,ts,tsx}', '**/__tests__/**/*'],
    rules: {
      'no-console': 'off', // Allow console in tests
    },
  },
  {
    files: ['**/*.config.{js,ts}', '**/vite.config.*', '**/rollup.config.*'],
    rules: {
      'no-console': 'off', // Allow console in config files
    },
  },
];