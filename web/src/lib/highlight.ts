import hljs from 'highlight.js/lib/common'
import 'highlight.js/styles/github.css'

const EXT_LANG: Record<string, string> = {
  go: 'go', js: 'javascript', mjs: 'javascript', cjs: 'javascript', jsx: 'javascript',
  ts: 'typescript', tsx: 'typescript', py: 'python', rb: 'ruby', java: 'java',
  c: 'c', h: 'c', cpp: 'cpp', cc: 'cpp', hpp: 'cpp', cs: 'csharp', php: 'php',
  rs: 'rust', kt: 'kotlin', swift: 'swift', scala: 'scala', sh: 'bash', bash: 'bash',
  json: 'json', yaml: 'yaml', yml: 'yaml', xml: 'xml', html: 'xml', css: 'css',
  scss: 'scss', sql: 'sql', md: 'markdown'
}

// languageFor returns the highlight.js language id for a file path, or undefined.
export function languageFor(path: string): string | undefined {
  const dot = path.lastIndexOf('.')
  if (dot < 0) return undefined
  const ext = path.slice(dot + 1).toLowerCase()
  const lang = EXT_LANG[ext]
  if (!lang) return undefined
  return hljs.getLanguage(lang) ? lang : undefined
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

// highlightLine returns HTML for a single source line. Highlights with the given
// language (balanced per-line markup); falls back to HTML-escaped plain text.
export function highlightLine(line: string, language: string | undefined): string {
  if (!language) return escapeHtml(line)
  try {
    return hljs.highlight(line, { language, ignoreIllegals: true }).value
  } catch {
    return escapeHtml(line)
  }
}
