import pluginVue from 'eslint-plugin-vue'
import tsParser from '@typescript-eslint/parser'
import tsPlugin from '@typescript-eslint/eslint-plugin'
import prettierPlugin from 'eslint-plugin-prettier'
import prettierConfig from 'eslint-config-prettier'

export default [
  // Global ignores
  {
    ignores: ['dist/**', 'node_modules/**', 'screenshots/**', 'log/**'],
  },

  // Base JS/TS config
  {
    files: ['src/**/*.{js,ts,vue}'],
    languageOptions: {
      parser: tsParser,
      ecmaVersion: 'latest',
      sourceType: 'module',
    },
    plugins: {
      '@typescript-eslint': tsPlugin,
      prettier: prettierPlugin,
    },
    rules: {
      ...tsPlugin.configs.recommended.rules,
      ...prettierConfig.rules,
      'prettier/prettier': 'warn',
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
      '@typescript-eslint/ban-ts-comment': ['error', { 'ts-nocheck': false }],
      '@typescript-eslint/no-empty-object-type': 'off',
      '@typescript-eslint/no-unused-expressions': ['error', { allowShortCircuit: true, allowTernary: true }],
      'no-console': ['warn', { allow: ['warn', 'error'] }],
    },
  },

  // Vue files
  ...pluginVue.configs['flat/recommended'].map((config) => ({
    ...config,
    files: ['src/**/*.vue'],
    languageOptions: {
      ...config.languageOptions,
      parserOptions: {
        parser: tsParser,
      },
    },
  })),

  // Vue-specific overrides
  {
    files: ['src/**/*.vue'],
    rules: {
      'vue/multi-word-component-names': 'off',
      'vue/require-default-prop': 'off',
      'prettier/prettier': 'off', // prettier breaks Vue inline expressions
      'vue/html-self-closing': ['warn', {
        html: { void: 'always', normal: 'never' },
        svg: 'always',
        math: 'always',
      }],
    },
  },
]
