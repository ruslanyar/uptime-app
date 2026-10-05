# Uptime App

Сервис мониторинга uptime в разработке: Go-бэкенд и независимый Next.js-фронтенд.

В бэкенде реализован HTTP API регистрации и авторизации с PostgreSQL, Argon2id,
access JWT и ротацией refresh-сессий. Оба токена передаются в HttpOnly cookies;
JSON-ответы содержат только пользователя. Миграции запускаются отдельной командой Tern;
типизированные запросы генерируются sqlc. Мониторинг сайтов и интеграция фронтенда
с API пока не реализованы.

- [Backend: запуск, API и проверки](backend/README.md)
- [Frontend: запуск и разработка](frontend/README.md)
- [План авторизации и выполненные проверки](backend/docs/plan/AUTH_PLAN.md)
- [Текущий cookie-контракт авторизации](backend/docs/plan/COOKIE_AUTH_PLAN.md)
