import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import App from './App.tsx'

describe('App', () => {
  it('サービス名と準備中の案内を表示する', () => {
    const html = renderToStaticMarkup(<App />)
    expect(html).toContain('ポイ得比較')
    expect(html).toContain('準備中')
  })
})
