# Frontend

Чистый проект Next.js 16.3.4 из официального шаблона `app-empty`:
TypeScript, App Router, ESLint и npm. На стартовой странице только `Hello world!`.

Требуется Node.js 20.9+ и npm.

Из директории `frontend`:

```sh
npm ci
npm run dev
```

Приложение доступно по адресу http://localhost:3000.

Проверки и production-сборка:

```sh
npm run lint
npm run typecheck
npm run build
npm start
```

- `src/app` — корневой layout и стартовая страница.
- `public` — директория для статических файлов.
- `next.config.ts`, `tsconfig.json`, `eslint.config.mjs` — базовая конфигурация.

Бизнес-логика и подключение к backend ещё не реализованы.
