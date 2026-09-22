import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// https://vite.dev/config/ , https://vitest.dev/config/
export default defineConfig({
  plugins: [react()],
  test: {
    // DOM が必要になったら jsdom を追加する。現状は react-dom/server で描画して検証する
    environment: 'node',
  },
})
