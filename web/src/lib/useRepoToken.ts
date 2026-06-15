import { ref } from 'vue'
import http, { errorMessage } from '@/plugins/http'

export function useRepoToken(repoPath: string) {
  const token = ref('')
  const revealed = ref(false)
  const busy = ref(false)
  const error = ref('')

  async function reveal() {
    busy.value = true
    error.value = ''
    try {
      const { data } = await http.get<{ token: string }>(`${repoPath}/token`)
      token.value = data.token
      revealed.value = true
    } catch (e) {
      error.value = errorMessage(e)
    } finally {
      busy.value = false
    }
  }

  async function rotate() {
    busy.value = true
    error.value = ''
    try {
      const { data } = await http.patch<{ token: string }>(`${repoPath}/token`)
      token.value = data.token
      revealed.value = true
    } catch (e) {
      error.value = errorMessage(e)
    } finally {
      busy.value = false
    }
  }

  return { token, revealed, busy, error, reveal, rotate }
}
