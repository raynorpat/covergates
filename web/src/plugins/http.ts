import axios from 'axios'
import { basePath } from '@/lib/base'

const http = axios.create({ baseURL: basePath() })

export default http

// Extract a human-readable message from an Axios error.
export function errorMessage(err: unknown): string {
  if (axios.isAxiosError(err)) {
    const data = err.response?.data as { message?: string } | string | undefined
    if (typeof data === 'string' && data) return data
    if (data && typeof data === 'object' && data.message) return data.message
    return err.message
  }
  return String(err)
}
