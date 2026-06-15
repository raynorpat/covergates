// Runtime base path, injected into index.html by the Go template (window.VUE_BASE).
// Empty string when hosted at root. JS bundles are not templated, so the base must
// be read at runtime, never from import.meta.env.BASE_URL.
export function basePath(): string {
  const raw = window.VUE_BASE ?? ''
  // Guard against the un-substituted template token during local `vite preview`.
  if (raw.includes('{{')) return ''
  return raw.replace(/\/$/, '')
}
