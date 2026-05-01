# Inventario — React frontend (`frontend/`)

The Inventario web client, built in React 19 + TypeScript + Vite + Tailwind v4 + shadcn/ui.

## Stack

| Layer | Choice |
|---|---|
| Framework | React 19 + TypeScript (strict) |
| Build | Vite 8 + `@tailwindcss/vite` |
| Styling | Tailwind CSS v4 (`@theme inline` + OKLCH tokens) |
| Components | shadcn/ui (new-york / neutral) on top of `radix-ui` |
| Icons | `lucide-react` |
| Forms | `react-hook-form` + `zod` |
| Notifications | `sonner` |
| Tests | Vitest + `@testing-library/react` + `@testing-library/user-event` + `jsdom` + `jest-axe` |
| E2E | Playwright (`e2e/` at the repo root) |
| Lint/format | ESLint flat config + Prettier |

The visual contract is canonical in [`denisvmedia/inventario-design`](https://github.com/denisvmedia/inventario-design) — see its `CLAUDE.md`. Do not modify the mock from this codebase; if the mock is wrong or missing something, file an issue *there* first.

## Quick start

```bash
# from this directory
npm install
npm run dev          # http://localhost:5173, proxies /api to :3333
npm run build        # produces dist/
npm run preview      # serves the production bundle
npm run lint
npm run typecheck
npm run test
npm run test:coverage
```

The shell-friendly wrappers live in the repo's root `Makefile`:

```bash
make build-frontend
make lint-frontend
make test-frontend
```

## Layout

```
frontend/
├── public/                 static assets (favicon, etc.)
├── src/
│   ├── app/                application shell, providers, router
│   ├── pages/              one folder per route
│   ├── features/           feature slices (auth, group, commodity, file, tag, …)
│   ├── components/
│   │   ├── ui/             shadcn primitives copied via the shadcn CLI
│   │   └── theme-provider  tiny custom theme hook (no next-themes dep)
│   ├── lib/                cn(), env, http
│   ├── hooks/              cross-feature hooks
│   ├── i18n/               react-i18next config
│   ├── types/              OpenAPI-generated DTOs + hand-written types
│   ├── test/               Vitest setup + shared fixtures
│   ├── index.css           Tailwind v4 + @theme tokens
│   └── main.tsx            entry
├── frontend.go             //go:embed all:dist for the Go binary
├── go.mod                  companion module so `with_frontend` builds work
├── components.json         shadcn CLI config
├── eslint.config.js        flat config
├── tsconfig*.json          strict TS, project refs
├── vite.config.ts
└── vitest.config.ts
```

## Embedding into the Go binary

`frontend.go`, under the `with_frontend` build tag, embeds `dist/` via `//go:embed all:dist` and exposes it as `frontend.GetDist()`. `apiserver.FrontendHandler` consumes that and serves the SPA at `/`.

## Adding shadcn primitives

Use the shadcn CLI with this directory as the working dir:

```bash
cd frontend
npx shadcn@latest add button input dialog
```

The CLI uses `components.json` and writes into `src/components/ui/`. Lock to Radix defaults — do not pull in `@base-ui/react`.
