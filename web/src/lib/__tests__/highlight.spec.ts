import { describe, it, expect } from 'vitest'
import { languageFor, highlightLine } from '@/lib/highlight'

describe('highlight utils', () => {
  it('languageFor maps known extensions', () => {
    expect(languageFor('a/b/main.go')).toBe('go')
    expect(languageFor('x.ts')).toBe('typescript')
    expect(languageFor('y.py')).toBe('python')
    expect(languageFor('noext')).toBeUndefined()
    expect(languageFor('weird.xyz')).toBeUndefined()
  })
  it('highlightLine escapes HTML for unknown language', () => {
    expect(highlightLine('a < b && c', undefined)).toBe('a &lt; b &amp;&amp; c')
  })
  it('highlightLine returns markup for a known language', () => {
    const html = highlightLine('const x = 1', 'typescript')
    expect(html).toContain('hljs-keyword')
    expect(html).toContain('x')
  })
})
