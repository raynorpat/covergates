import axios from 'axios'
import { basePath } from '@/lib/base'

// isStatus reports whether an error is an Axios error with the given HTTP status.
export function isStatus(e: unknown, status: number): boolean {
  return axios.isAxiosError(e) && e.response?.status === status
}

export function is401(e: unknown): boolean {
  return isStatus(e, 401)
}

// loginUrl builds the backend OAuth entry URL that returns to fullPath after login.
export function loginUrl(fullPath: string): string {
  return `${basePath()}/login?redirect=${encodeURIComponent(fullPath)}`
}
