import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

const host = process.env.TAURI_DEV_HOST;

export default defineConfig({
  clearScreen: false,
  plugins: [react()],
  server: {
    strictPort: true,
    host: host || false,
    port: 5173,
  },
  envPrefix: ['VITE_', 'TAURI_ENV_*'],
  build: {
    target: process.env.TAURI_ENV_PLATFORM === 'windows' ? 'chrome105' : 'safari13',
    minify: !process.env.TAURI_ENV_DEBUG ? 'oxc' : false,
    sourcemap: !!process.env.TAURI_ENV_DEBUG,
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/setupTests.ts',
  },
});
