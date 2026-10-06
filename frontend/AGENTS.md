<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->

# Repository Guidelines

## Project Structure & Module Organization

This project implements browser authentication using Next.js, TypeScript and the App Router. It calls the Go API directly with HttpOnly cookies. Uptime monitoring is not implemented yet.

- `src/app/`: root layout, styles and login/register/account pages.
- `src/components/`: AuthProvider, forms and session states.
- `src/lib/auth/`: typed API, validation, session and browser coordination.
- `tests/`: Vitest/component tests and Playwright E2E; `scripts/`: isolated E2E servers.
- `public/`: static assets.
- `next.config.ts`, `tsconfig.json`, and `eslint.config.mjs`: framework, TypeScript, and lint configuration.

Shared contribution and security rules are in [../AGENTS.md](../AGENTS.md).

## Build, Test, and Development Commands

Use Node.js 20.19+ (or 22.12+ / 24+) and npm. Run these commands from `frontend/`:

- `npm ci`: install dependencies using the committed lockfile.
- `npm run dev`: start development at `http://localhost:3000`.
- `npm run lint`: run ESLint with Next.js and TypeScript rules.
- `npm run format`: format source, configuration, and documentation with Prettier.
- `npm run format:check`: check formatting without modifying files.
- `npm run typecheck`: check TypeScript without emitting files.
- `npm test`: run Vitest and React Testing Library checks.
- `npm run test:e2e`: run Playwright with the real API and isolated PostgreSQL.
- `npm run build`: create a production build with `NEXT_PUBLIC_API_URL` set.
- `npm start`: serve the production build after building.

## Coding Style & Naming Conventions

Match existing TypeScript style: two-space indentation, single quotes in JavaScript/TypeScript strings, double quotes in JSX attributes, and semicolons. Prettier is configured in `.prettierrc.json`; `.prettierignore` excludes dependencies, generated files, test artifacts, and local configuration. Run `npm run format:check` when validating changes. TypeScript strict checking is enabled. Use `@/*` imports for modules under `src/`, PascalCase for React components, and App Router filenames such as `page.tsx` and `layout.tsx`.

## Testing & Pull Request Guidelines

Run lint, type checking, Vitest, a production build, and Playwright E2E for authentication changes. E2E requires Docker Compose, Go, OpenSSL and Chromium; see README for ports and configuration. Skipped real-API scenarios do not count as validation. Inspect affected screens. Document any new test tooling and commands when introduced. Include screenshots in PRs that change the UI.
