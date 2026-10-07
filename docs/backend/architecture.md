# Структура

- `cmd/api` — API; `cmd/migrate` — отдельный запуск миграций.
- `internal/app` — подключение БД, HTTP-сервер, таймауты и graceful shutdown.
- `internal/config`, `internal/auth` — настройки и сервис авторизации.
- `internal/controllers/auth` — контроллеры регистрации, входа, refresh, logout и `/me`.
- `internal/httpapi` — маршруты, CORS/CSRF middleware и общие JSON-ответы.
- `internal/storage/postgres` — транзакции и преобразование типов/ошибок БД.
- `internal/storage/postgres/queries` — SQL; `sqlc` — коммитируемый генерируемый пакет.
- `migrations` — общая схема для Tern и sqlc.
