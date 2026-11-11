import { expect, test } from 'vitest'
import { highlightedHtml } from './highlight'

test('search highlighting escapes HTML and restores only mark tags', () => {
  expect(highlightedHtml('a <b> & "c"')).toBe('a &lt;b&gt; &amp; &quot;c&quot;')
  expect(highlightedHtml('see <mark>Atlas</mark> now')).toBe('see <mark>Atlas</mark> now')
  expect(highlightedHtml('<mark><script></mark>')).toBe('<mark>&lt;script&gt;</mark>')
})
