module.exports = {
  root: true,
  extends: ['plugin:vue/vue3-essential', '@vue/eslint-config-typescript'],
  parserOptions: { ecmaVersion: 'latest' },
  rules: {
    // Route views and single-purpose components are intentionally single-word.
    'vue/multi-word-component-names': 'off'
  }
}
