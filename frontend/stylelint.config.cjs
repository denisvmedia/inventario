module.exports = {
  extends: ['stylelint-config-standard-scss'],
  rules: {
    'at-rule-disallowed-list': ['import'],
    'at-rule-no-unknown': null,
    'no-descending-specificity': true,
    'selector-class-pattern': null,
    'selector-pseudo-class-no-unknown': [
      true,
      {
        ignorePseudoClasses: ['deep']
      }
    ],
    'scss/at-rule-no-unknown': true
  },
  overrides: [
    {
      files: ['**/*.vue'],
      customSyntax: 'postcss-html',
    },
    {
      files: ['**/*.scss'],
      customSyntax: 'postcss-scss'
    }
  ]
};
