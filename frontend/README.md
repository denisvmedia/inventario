# Inventario Frontend

A modern Vue 3 + TypeScript frontend for the Inventario application.

## Development Setup

### Prerequisites

- Node.js (v18 or later)
- npm (v8 or later)

### Installation

```bash
# Install dependencies
npm install
```

### Development

```bash
# Start development server with hot-reload
npm run dev
```

### Linting and Formatting

```bash
# Lint and fix files
npm run lint

# Format files with Prettier
npm run format

# Type check
npm run type-check
```

### Building for Production

```bash
# Build for production
npm run build

# Preview production build
npm run preview
```

## Coding Standards

This project follows these coding standards:

- Vue 3 Composition API with `<script setup>` syntax
- TypeScript for type safety
- ESLint for code quality
- Prettier for code formatting

## Recommended IDE Setup

- [VS Code](https://code.visualstudio.com/)
- [Volar](https://marketplace.visualstudio.com/items?itemName=Vue.volar) (disable Vetur)
- [ESLint](https://marketplace.visualstudio.com/items?itemName=dbaeumer.vscode-eslint)
- [Prettier](https://marketplace.visualstudio.com/items?itemName=esbenp.prettier-vscode)

## Project Structure

```
frontend/
├── public/              # Static assets
├── src/
│   ├── assets/          # Assets that will be processed by the build
│   ├── components/      # Reusable Vue components
│   ├── router/          # Vue Router configuration
│   ├── services/        # API services
│   ├── stores/          # Pinia stores
│   ├── types/           # TypeScript type definitions
│   ├── views/           # Page components
│   ├── App.vue          # Root component
│   └── main.ts          # Application entry point
├── .eslintrc.js         # ESLint configuration
├── .prettierrc          # Prettier configuration
├── tsconfig.json        # TypeScript configuration
└── vite.config.ts       # Vite configuration
```