export interface RepoSetting {
  filters: string[]
  mergePR: boolean
  updateAction: string
  protected: boolean
  coverageMinimum: number
  coverageDecreaseThreshold: number
  disablePRComment: boolean
  emailRecipients: string[]
  slackWebhook: string
  notifyTrigger: string
}
