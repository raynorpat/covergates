export interface SourceFile {
  name: string
  source_digest: string
  coverage: (number | null)[]
}

export interface Job {
  id: number
  serviceJobID: string
  coverage: number
}

export type BuildStatus = 'processing' | 'done' | 'errored'

export interface Build {
  number: number
  serviceName: string
  serviceNumber: string
  commit: string
  branch: string
  pullRequest: number
  status: BuildStatus
  parallel: boolean
  coverage: number
  coverageChange: number
  baseBuildID: number
  baseBuildNumber: number
  commitMessage: string
  authorName: string
  authorEmail: string
  sourceFiles?: SourceFile[]
  jobs?: Job[]
  createdAt: string
  finishedAt: string
}
