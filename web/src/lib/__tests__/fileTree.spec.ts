import { describe, it, expect } from 'vitest'
import { buildTree } from '@/lib/fileTree'

describe('buildTree', () => {
  it('nests files into folders and aggregates coverage', () => {
    const root = buildTree([
      { path: 'a/b.go', covered: 1, relevant: 2 },
      { path: 'a/c.go', covered: 2, relevant: 2 },
      { path: 'd.go', covered: 0, relevant: 1 }
    ])
    expect(root.covered).toBe(3)
    expect(root.relevant).toBe(5)
    const names = root.children.map((c) => c.name)
    expect(names).toEqual(['a', 'd.go'])
    const a = root.children.find((c) => c.name === 'a')!
    expect(a.isDir).toBe(true)
    expect(a.covered).toBe(3)
    expect(a.relevant).toBe(4)
    expect(a.children.map((c) => c.name)).toEqual(['b.go', 'c.go'])
    const d = root.children.find((c) => c.name === 'd.go')!
    expect(d.isDir).toBe(false)
    expect(d.path).toBe('d.go')
  })
})
