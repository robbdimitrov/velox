module.exports = {
  root: true,
  extends: ['eslint:recommended', 'plugin:svelte/recommended'],
  parserOptions: {
    sourceType: 'module',
    ecmaVersion: 2022
  },
  env: {
    browser: true,
    es2022: true,
    node: true
  },
  overrides: [
    {
      files: ['*.svelte'],
      parser: 'svelte-eslint-parser'
    }
  ]
};
