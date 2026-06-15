import { describe, it, expect } from 'vitest'
import { fileCoverage, lineState, formatPercent, coverageColor } from '@/lib/coverage'
import type { SourceFile } from '@/types/build'

const f = (coverage: (number | null)[]): SourceFile => ({ name: 'x', source_digest: '', coverage })

describe('coverage utils', () => {
  it('fileCoverage counts relevant and covered', () => {
    expect(fileCoverage(f([1, null, 0, 2]))).toEqual({ covered: 2, relevant: 3, ratio: 2 / 3 })
  })
  it('fileCoverage is 0 when no relevant lines', () => {
    expect(fileCoverage(f([null, null]))).toEqual({ covered: 0, relevant: 0, ratio: 0 })
  })
  it('lineState maps hits/miss/none', () => {
    expect(lineState(3)).toBe('hit')
    expect(lineState(0)).toBe('miss')
    expect(lineState(null)).toBe('none')
  })
  it('formatPercent renders one decimal', () => {
    expect(formatPercent(0.824)).toBe('82.4%')
    expect(formatPercent(0)).toBe('0.0%')
  })
  it('coverageColor thresholds', () => {
    expect(coverageColor(0.9)).toBe('success')
    expect(coverageColor(0.6)).toBe('warning')
    expect(coverageColor(0.2)).toBe('error')
  })
})
