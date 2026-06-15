export type SCM = 'github' | 'gitea' | 'gitlab'

export interface User {
  login: string
  avatar?: string
  email?: string
}

export interface Repository {
  ID: number
  URL: string
  ReportID: string
  NameSpace: string
  Name: string
  Branch: string
  Private: boolean
  SCM: SCM
}
