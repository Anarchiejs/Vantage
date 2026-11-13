import { expect, test } from 'vitest'
import { highlightedHtml } from './highlight'

test('search highlighting escapes HTML and restores only server sentinels', () => {
  expect(highlightedHtml('a <b> & "c"')).toBe('a &lt;b&gt; &amp; &quot;c&quot;')
  expect(highlightedHtml('see \uE000Atlas\uE001 now')).toBe('see <mark>Atlas</mark> now')
  expect(highlightedHtml('\uE000<script>\uE001')).toBe('<mark>&lt;script&gt;</mark>')
  expect(highlightedHtml('<mark>Atlas</mark>')).toBe('&lt;mark&gt;Atlas&lt;/mark&gt;')
})
