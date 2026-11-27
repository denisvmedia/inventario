import js from "@eslint/js"
import globals from "globals"
import tseslint from "typescript-eslint"
import reactPlugin from "eslint-plugin-react"
import reactHooks from "eslint-plugin-react-hooks"
import jsxA11y from "eslint-plugin-jsx-a11y"
import prettier from "eslint-config-prettier"

export default [
  {
    ignores: ["node_modules/**", "dist/**", "coverage/**", ".vite/**", "**/*.d.ts"],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["**/*.{ts,tsx}"],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "module",
      globals: { ...globals.browser, ...globals.es2022 },
      parserOptions: {
        ecmaFeatures: { jsx: true },
      },
    },
    plugins: {
      react: reactPlugin,
      "react-hooks": reactHooks,
      "jsx-a11y": jsxA11y,
    },
    settings: {
      // Pin the React version explicitly. With "detect", eslint-plugin-react
      // calls resolveBasedir(context) which assumes context.getFilename() —
      // a method removed in ESLint 10.x. eslint-plugin-react ≤ 7.37.5 has
      // not adopted the property-style `context.filename` yet, so any
      // "detect" config crashes the linter immediately. Pinning skips the
      // detect path entirely; bump this number when react upgrades.
      react: { version: "19.2" },
    },
    rules: {
      ...reactPlugin.configs.recommended.rules,
      ...reactPlugin.configs["jsx-runtime"].rules,
      ...reactHooks.configs.recommended.rules,
      ...jsxA11y.configs.recommended.rules,
      "react/prop-types": "off",
      "@typescript-eslint/no-unused-vars": ["warn", { argsIgnorePattern: "^_" }],
      "@typescript-eslint/no-explicit-any": "warn",
      // Design-system guard (#1264): the blessed dropdown for form &
      // settings fields is the shadcn <Select> primitive
      // (@/components/ui/select). A raw native <select> drifts from the
      // single visual contract. It stays permissible for utility / compact
      // / mobile selectors (bulk-move bars, mobile category collapse, list
      // sort) — annotate those deliberately with
      // `eslint-disable-next-line no-restricted-syntax -- <reason>`. See
      // devdocs/frontend/forms.md → "Dropdowns".
      "no-restricted-syntax": [
        "error",
        {
          selector: "JSXOpeningElement[name.name='select']",
          message:
            "Use the shadcn <Select> (@/components/ui/select) for a form or settings dropdown, and <NativeSelect> (@/components/ui/native-select) for a utility select that keeps the native element (a sort picker, a bulk bar, a per-row picker). Never a raw <select>. See devdocs/frontend/forms.md → Dropdowns.",
        },
        {
          // A field error rendered by hand: `{t(errors.name.message)}` or
          // `{t(form.formState.errors.name.message ?? "")}` inside JSX.
          selector:
            "JSXExpressionContainer > CallExpression[callee.name='t'] MemberExpression[property.name='message']:matches([object.object.name='errors'], [object.object.property.name='errors'])",
          message:
            "Render a field error with <FieldError message={errors.<field>?.message} /> (@/components/FieldError), and set aria-invalid and aria-describedby on the input. See devdocs/frontend/forms.md → Field components.",
        },
      ],
      // eslint-plugin-react-hooks 5.x added a React-Compiler ruleset that
      // flags `form.watch(cb)` from React Hook Form because the callback
      // form returns a non-memoizable subscription. RHF's watch() is the
      // idiomatic way to react to field changes, and rewriting every call
      // site to a `Controller` / `useWatch` migration is out of scope for
      // any single PR. Keep the rule off until either RHF adds a
      // memoizable surface or the codebase migrates wholesale.
      "react-hooks/incompatible-library": "off",
    },
  },
  {
    files: ["src/**/*.{test,spec}.{ts,tsx}", "src/test/**/*.{ts,tsx}"],
    languageOptions: {
      globals: { ...globals.browser, ...globals.node, ...globals.es2022 },
    },
  },
  {
    files: [
      "vite.config.ts",
      "vitest.config.ts",
      "vite-plugins/**/*.ts",
      "eslint.config.js",
      "i18next.config.ts",
      "scripts/**/*.{js,mjs,ts}",
    ],
    languageOptions: {
      globals: { ...globals.node, ...globals.es2022 },
    },
  },
  {
    files: ["**/*.cjs"],
    languageOptions: {
      sourceType: "commonjs",
      globals: { ...globals.node, ...globals.es2022 },
    },
  },
  prettier,
]
