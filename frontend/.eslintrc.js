module.exports = {
  extends: ['eslint:recommended', 'airbnb', 'airbnb-typescript'],
  overrides: [
    {
      files: ['**/*.js', '**/*.jsx', '**/*.ts', '**/*.tsx'],
    }
  ],
  rules: {
    '@typescript-eslint/no-shadow': 'off',
    'max-len': ["error", { "code": 160 }]
  },
  parserOptions: {
    'project': ['tsconfig.json']
  }
};
