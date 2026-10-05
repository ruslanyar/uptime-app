<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->

# Repository Guidelines

## Project Structure & Module Organization

This project is a Next.js scaffold using TypeScript and the App Router. Business logic and backend integration are not implemented yet.

- `src/app/`: pages and root layout; the current page displays `Hello world!`.
- `public/`: static assets.
- `next.config.ts`, `tsconfig.json`, and `eslint.config.mjs`: framework, TypeScript, and lint configuration.

Shared contribution and security rules are in [../AGENTS.md](../AGENTS.md).

## Build, Test, and Development Commands

Use Node.js 20.9+ and npm. Run these commands from `frontend/`:

- `npm ci`: install dependencies using the committed lockfile.
- `npm run dev`: start development at `http://localhost:3000`.
- `npm run lint`: run ESLint with Next.js and TypeScript rules.
- `npm run typecheck`: check TypeScript without emitting files.
- `npm run build`: create a production build.
- `npm start`: serve the production build after building.

## Coding Style & Naming Conventions

Match existing TypeScript style: two-space indentation, double quotes, and semicolons. TypeScript strict checking is enabled. Use `@/*` imports for modules under `src/`, PascalCase for React components, and App Router filenames such as `page.tsx` and `layout.tsx`. No separate formatter is configured.

## Testing & Pull Request Guidelines

No frontend tests or test runner currently exist. Run lint, type checking, and a production build for frontend changes, and manually verify affected screens. Document any new test tooling and commands when introduced. Include screenshots in PRs that change the UI.
