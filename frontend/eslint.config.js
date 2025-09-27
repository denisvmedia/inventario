import js from '@eslint/js';
import globals from 'globals';
import eslintPluginVue from 'eslint-plugin-vue';
import tseslint from 'typescript-eslint';
import vueParser from 'vue-eslint-parser';
import prettier from 'eslint-config-prettier';

const sharedGlobals = {
  ...globals.browser,
  ...globals.node,
  ...globals.es2021,
  process: 'readonly',
  console: 'readonly',
  module: 'readonly',
  __dirname: 'readonly'
};

export default [
  {
    ignores: [
      'node_modules/**',
      'dist/**',
      '.vite/**',
      '**/*.d.ts',
      'public/**',
      '**/*.worker.min.js'
    ]
  },

  js.configs.recommended,
  prettier, // must come after all configs to disable formatting rules

  {
    plugins: {
      vue: eslintPluginVue,
      '@typescript-eslint': tseslint.plugin
    }
  },

  // All .vue files, with TypeScript support
  {
    files: ['**/*.vue'],
    languageOptions: {
      parser: vueParser,
      parserOptions: {
        ecmaVersion: 2021,
        sourceType: 'module',
        parser: tseslint.parser // for <script lang="ts">
      },
      globals: sharedGlobals
    },
    rules: {
      ...eslintPluginVue.configs.recommended.rules,
      ...tseslint.configs.recommended.rules,
      'vue/multi-word-component-names': 'off',
      'vue/require-default-prop': 'off',
      'vue/no-v-html': 'warn',
      'vue/no-reserved-component-names': 'off',
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/explicit-module-boundary-types': 'off',
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
      'no-unused-vars': ['error', { argsIgnorePattern: '^_' }]
    }
  },

  // Vitest configuration files
  {
    files: ['vitest.config.ts'],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: {
        ecmaVersion: 2021,
        sourceType: 'module',
        project: './tsconfig.vitest.json'
      },
      globals: sharedGlobals
    },
    rules: {
      ...tseslint.configs.recommended.rules,
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/explicit-module-boundary-types': 'off',
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
      'no-unused-vars': ['error', { argsIgnorePattern: '^_' }]
    }
  },

  // Standalone TypeScript files
  {
    files: ['**/*.ts', '**/*.tsx'],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: {
        ecmaVersion: 2021,
        sourceType: 'module',
        project: './tsconfig.json'
      },
      globals: sharedGlobals
    },
    rules: {
      ...tseslint.configs.recommended.rules,
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/explicit-module-boundary-types': 'off',
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
      'no-unused-vars': ['error', { argsIgnorePattern: '^_' }]
    }
  },

  // Standalone JavaScript files
  {
    files: ['**/*.js', '**/*.jsx', '**/*.mjs', '**/*.cjs'],
    languageOptions: {
      parserOptions: {
        ecmaVersion: 2021,
        sourceType: 'module'
      },
      globals: sharedGlobals
    }
  },

  // Global overrides
  {
    rules: {
      'no-console': 'off',
      'no-debugger': 'off',
      'max-len': 'off',
      'no-prototype-builtins': 'off',
      'no-self-assign': 'off'
    }
  }
];
