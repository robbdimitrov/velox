import js from '@eslint/js';
import ts from '@typescript-eslint/eslint-plugin';
import tsParser from '@typescript-eslint/parser';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';

export default [
  js.configs.recommended,
  ...svelte.configs['flat/recommended'],
  {
    ignores: ['.svelte-kit/**', 'build/**', 'node_modules/**']
  },
  {
    files: ['**/*.ts', '**/*.svelte'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        extraFileExtensions: ['.svelte'],
        parser: tsParser,
        sourceType: 'module'
      },
      globals: {
        ...globals.browser,
        ...globals.node
      }
    },
    plugins: {
      '@typescript-eslint': ts
    },
    rules: {
      ...ts.configs.recommended.rules
    }
  }
];
