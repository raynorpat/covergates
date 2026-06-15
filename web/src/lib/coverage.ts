import type { SourceFile } from '@/types/build'

export interface FileCoverage {
  covered: number
  relevant: number
  ratio: number
}

// fileCoverage computes covered/relevant statement counts and their ratio.
// relevant = non-null entries; covered = entries > 0; ratio 0 when none relevant.
export function fileCoverage(file: SourceFile): FileCoverage {
  let covered = 0
  let relevant = 0
  for (const hit of file.coverage) {
    if (hit === null) continue
    relevant++
    if (hit > 0) covered++
  }
  return { covered, relevant, ratio: relevant === 0 ? 0 : covered / relevant }
}

export type LineState = 'hit' | 'miss' | 'none'

export function lineState(hits: number | null): LineState {
  if (hits === null) return 'none'
  return hits > 0 ? 'hit' : 'miss'
}

export function formatPercent(ratio: number): string {
  return `${(ratio * 100).toFixed(1)}%`
}

// coverageColor maps a ratio to a Vuetify theme color name.
export function coverageColor(ratio: number): 'success' | 'warning' | 'error' {
  if (ratio >= 0.8) return 'success'
  if (ratio >= 0.5) return 'warning'
  return 'error'
}
