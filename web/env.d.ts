/// <reference types="vite/client" />
/// <reference types="vuetify" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

declare global {
  interface Window {
    VUE_BASE?: string
  }
}
export {}
